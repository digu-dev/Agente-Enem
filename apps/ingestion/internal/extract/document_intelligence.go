package extract

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/digu-dev/Agente-Enem/apps/ingestion/internal/corpus"
	"github.com/digu-dev/Agente-Enem/apps/ingestion/internal/parse"
	"github.com/digu-dev/Agente-Enem/apps/ingestion/internal/records"
)

const (
	documentIntelligenceAPIVersion = "2024-11-30"
	documentIntelligenceMaxBytes   = 4 << 20
	documentIntelligenceMaxPages   = 500
)

type PageAnalyzer interface {
	AnalyzePage(context.Context, []byte) (string, error)
}

type DocumentIntelligenceClient struct {
	endpoint   string
	credential interface {
		GetToken(context.Context, policy.TokenRequestOptions) (azcore.AccessToken, error)
	}
	httpClient   *http.Client
	pollInterval time.Duration
}

func NewDocumentIntelligenceFromEnvironment() (*DocumentIntelligenceClient, error) {
	endpoint := strings.TrimRight(os.Getenv("DOCUMENT_INTELLIGENCE_ENDPOINT"), "/")
	if endpoint == "" {
		return nil, errors.New("DOCUMENT_INTELLIGENCE_ENDPOINT is required")
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil ||
		parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("Document Intelligence endpoint must be an HTTPS URL without embedded credentials")
	}
	credential, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("create Azure credential: %w", err)
	}
	return &DocumentIntelligenceClient{
		endpoint: endpoint, credential: credential,
		httpClient: &http.Client{
			Timeout: 45 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		pollInterval: time.Second,
	}, nil
}

func (c *DocumentIntelligenceClient) AnalyzePage(ctx context.Context, image []byte) (string, error) {
	if len(image) == 0 || len(image) > documentIntelligenceMaxBytes {
		return "", errors.New("Document Intelligence page image must be no larger than 4 MiB")
	}
	requestBody, err := json.Marshal(map[string]string{"base64Source": base64.StdEncoding.EncodeToString(image)})
	if err != nil {
		return "", err
	}
	requestURL := c.endpoint + "/documentintelligence/documentModels/prebuilt-read:analyze?api-version=" + documentIntelligenceAPIVersion
	token, err := c.credential.GetToken(ctx, policy.TokenRequestOptions{Scopes: []string{"https://cognitiveservices.azure.com/.default"}})
	if err != nil {
		return "", fmt.Errorf("get Document Intelligence token: %w", err)
	}
	var response *http.Response
	for attempt := 0; attempt < 5; attempt++ {
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(requestBody))
		if err != nil {
			return "", err
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", "Bearer "+token.Token)
		response, err = c.httpClient.Do(request)
		if err != nil {
			if attempt == 4 {
				return "", errors.New("Document Intelligence request failed after retries")
			}
			if err := waitRetry(ctx, attempt); err != nil {
				return "", err
			}
			continue
		}
		if response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500 {
			response.Body.Close()
			if attempt == 4 {
				return "", fmt.Errorf("Document Intelligence returned retryable status %d after retries", response.StatusCode)
			}
			if err := waitRetry(ctx, attempt); err != nil {
				return "", err
			}
			continue
		}
		break
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
	response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		return "", fmt.Errorf("Document Intelligence returned status %d", response.StatusCode)
	}
	operation := response.Header.Get("Operation-Location")
	if !sameServiceOrigin(c.endpoint, operation) {
		return "", errors.New("Document Intelligence returned an invalid operation URL")
	}
	for attempt := 0; attempt < 30; attempt++ {
		if err := sleepContext(ctx, time.Duration(math.Min(float64(attempt+1), 5))*c.pollInterval); err != nil {
			return "", err
		}
		poll, err := http.NewRequestWithContext(ctx, http.MethodGet, operation, nil)
		if err != nil {
			return "", err
		}
		poll.Header.Set("Authorization", "Bearer "+token.Token)
		result, err := c.httpClient.Do(poll)
		if err != nil {
			if retryable := waitRetry(ctx, attempt); retryable != nil {
				return "", retryable
			}
			continue
		}
		data, readErr := io.ReadAll(io.LimitReader(result.Body, 4<<20))
		result.Body.Close()
		if readErr != nil {
			return "", errors.New("read Document Intelligence result failed")
		}
		if result.StatusCode == http.StatusTooManyRequests || result.StatusCode >= 500 {
			if err := waitRetry(ctx, attempt); err != nil {
				return "", err
			}
			continue
		}
		if result.StatusCode < 200 || result.StatusCode >= 300 {
			return "", fmt.Errorf("Document Intelligence polling returned status %d", result.StatusCode)
		}
		var payload struct {
			Status        string `json:"status"`
			AnalyzeResult struct {
				Content string `json:"content"`
			} `json:"analyzeResult"`
		}
		if err := json.Unmarshal(data, &payload); err != nil {
			return "", errors.New("Document Intelligence returned invalid result JSON")
		}
		switch strings.ToLower(payload.Status) {
		case "succeeded":
			return payload.AnalyzeResult.Content, nil
		case "failed", "canceled":
			return "", errors.New("Document Intelligence analysis did not succeed")
		}
	}
	return "", errors.New("Document Intelligence analysis timed out")
}

func waitRetry(ctx context.Context, attempt int) error {
	delay := time.Duration(1<<min(attempt, 5)) * 200 * time.Millisecond
	jitter, err := rand.Int(rand.Reader, big.NewInt(int64(delay/2)+1))
	if err == nil {
		delay += time.Duration(jitter.Int64())
	}
	return sleepContext(ctx, delay)
}

func sameServiceOrigin(service, operation string) bool {
	serviceURL, err1 := url.Parse(service)
	operationURL, err2 := url.Parse(operation)
	return err1 == nil && err2 == nil && operationURL.User == nil && operationURL.Scheme == serviceURL.Scheme &&
		strings.EqualFold(serviceURL.Host, operationURL.Host)
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

type DIOptions struct {
	WorkDir      string
	MonthlyPages int
}

func ExtractWithDocumentIntelligence(ctx context.Context, analyzer PageAnalyzer, pdfPath string, entry corpus.Entry, options DIOptions) ([]records.Question, error) {
	textPages, imagePaths, err := readDocumentPagesWithDI(ctx, analyzer, pdfPath, entry, options)
	if err != nil {
		return nil, err
	}
	parsed, err := parse.ParsePages(textPages)
	if err != nil {
		return nil, err
	}
	if len(parsed) == 0 {
		return nil, errors.New("Document Intelligence did not extract any questions")
	}
	result := make([]records.Question, 0, len(parsed))
	for _, question := range parsed {
		alternatives := make([]records.Alternative, len(question.Alternatives))
		for i, alternative := range question.Alternatives {
			alternatives[i] = records.Alternative{Letter: alternative.Letter, Text: alternative.Text}
		}
		id := QuestionID(entry, question.Number)
		images, err := storeQuestionImages(options.WorkDir, id, question.SourcePages, imagePaths)
		if err != nil {
			return nil, err
		}
		result = append(result, records.Question{
			ID: id, File: entry.Name, Page: question.SourcePage,
			CorpusVersion: entry.CorpusVersion, Year: entry.Year, Booklet: entry.Booklet,
			Type: entry.Type, Source: entry.Source, Split: entry.Split, Number: question.Number,
			Statement: question.Statement, Alternatives: alternatives, Contents: []string{}, Images: images,
		})
	}
	return result, nil
}

func ExtractAnswersWithDocumentIntelligence(ctx context.Context, analyzer PageAnalyzer, pdfPath string, entry corpus.Entry, options DIOptions) (map[int]string, error) {
	pages, _, err := readDocumentPagesWithDI(ctx, analyzer, pdfPath, entry, options)
	if err != nil {
		return nil, err
	}
	var text strings.Builder
	for _, page := range pages {
		text.WriteString(page.Text)
		text.WriteByte('\n')
	}
	return parse.ParseAnswerKey(text.String()), nil
}

func readDocumentPagesWithDI(ctx context.Context, analyzer PageAnalyzer, pdfPath string, entry corpus.Entry, options DIOptions) ([]parse.Page, map[int]string, error) {
	if options.MonthlyPages == 0 {
		options.MonthlyPages = documentIntelligenceMaxPages
	}
	if options.MonthlyPages < 1 || options.MonthlyPages > documentIntelligenceMaxPages {
		return nil, nil, errors.New("Document Intelligence monthly page budget must be between 1 and 500")
	}
	info, err := runLimited(ctx, 64<<10, "pdfinfo", pdfPath)
	if err != nil {
		return nil, nil, errors.New("pdfinfo is required for the Document Intelligence fallback")
	}
	pageCount := 0
	for _, line := range strings.Split(string(info), "\n") {
		if strings.HasPrefix(line, "Pages:") {
			pageCount, _ = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "Pages:")))
			break
		}
	}
	if pageCount < 1 {
		return nil, nil, errors.New("could not determine PDF page count")
	}
	usagePath := filepath.Join(options.WorkDir, "document-intelligence-usage.json")
	usage, err := loadDIUsage(usagePath)
	if err != nil {
		return nil, nil, err
	}
	textPages := make([]parse.Page, 0, pageCount)
	imagePaths := make(map[int]string, pageCount)
	local := LocalPDF{WorkDir: options.WorkDir}
	for page := 1; page <= pageCount; page++ {
		imageRelative, err := local.renderPage(ctx, pdfPath, entry, page)
		if err != nil {
			return nil, nil, err
		}
		imagePaths[page] = imageRelative
		image, err := os.ReadFile(filepath.Join(options.WorkDir, filepath.FromSlash(imageRelative)))
		if err != nil {
			return nil, nil, fmt.Errorf("read rasterized page: %w", err)
		}
		if len(image) > documentIntelligenceMaxBytes {
			return nil, nil, errors.New("rasterized page exceeds the 4 MiB Document Intelligence F0 request limit")
		}
		cacheKey := corpus.FileHash([]byte(entry.SHA256 + ":" + strconv.Itoa(page)))
		cachePath := filepath.Join(options.WorkDir, "document-intelligence-cache", cacheKey+".txt")
		text, err := os.ReadFile(cachePath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, nil, fmt.Errorf("read Document Intelligence cache: %w", err)
		}
		if errors.Is(err, os.ErrNotExist) {
			if usage.Month == time.Now().UTC().Format("2006-01") && usage.Pages >= options.MonthlyPages {
				return nil, nil, errors.New("Document Intelligence local monthly page budget exhausted")
			}
			usage.Pages++
			usage.Month = time.Now().UTC().Format("2006-01")
			if err := writeDIUsage(usagePath, usage); err != nil {
				return nil, nil, err
			}
			textValue, err := analyzer.AnalyzePage(ctx, image)
			if err != nil {
				return nil, nil, err
			}
			text = []byte(textValue)
			if err := os.MkdirAll(filepath.Dir(cachePath), 0700); err != nil {
				return nil, nil, err
			}
			if err := records.WriteBytesAtomic(cachePath, text); err != nil {
				return nil, nil, err
			}
		}
		textPages = append(textPages, parse.Page{Number: page, Text: string(text)})
	}
	return textPages, imagePaths, nil
}

type diUsage struct {
	Month string `json:"month"`
	Pages int    `json:"pages"`
}

func loadDIUsage(path string) (diUsage, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return diUsage{Month: time.Now().UTC().Format("2006-01")}, nil
	}
	if err != nil {
		return diUsage{}, fmt.Errorf("read Document Intelligence usage: %w", err)
	}
	var usage diUsage
	if err := json.Unmarshal(data, &usage); err != nil {
		return diUsage{}, errors.New("Document Intelligence usage state is invalid")
	}
	if usage.Month != time.Now().UTC().Format("2006-01") {
		return diUsage{Month: time.Now().UTC().Format("2006-01")}, nil
	}
	return usage, nil
}

func writeDIUsage(path string, usage diUsage) error {
	data, err := json.Marshal(usage)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return records.WriteBytesAtomic(path, data)
}
