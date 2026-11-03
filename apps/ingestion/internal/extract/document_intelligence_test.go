package extract

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/digu-dev/Agente-Enem/apps/ingestion/internal/corpus"
)

type fakePageAnalyzer struct {
	calls atomic.Int32
	text  string
}

func (f *fakePageAnalyzer) AnalyzePage(_ context.Context, image []byte) (string, error) {
	f.calls.Add(1)
	if len(image) == 0 {
		return "", os.ErrInvalid
	}
	if f.text != "" {
		return f.text, nil
	}
	return "QUESTÃO 1\nQual é o resultado?\n(A) um\n(B) dois\n(C) três\n(D) quatro\n(E) cinco", nil
}

type fakeTokenCredential struct{}

func (fakeTokenCredential) GetToken(context.Context, policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{Token: "test-token"}, nil
}

func TestExtractWithDocumentIntelligenceCachesAndRespectsBudget(t *testing.T) {
	tools := fakePDFTools(t, "")
	pdfinfo := filepath.Join(tools, "pdfinfo")
	if err := os.WriteFile(pdfinfo, []byte("#!/bin/sh\nprintf 'Pages:          1\\n'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", tools+string(os.PathListSeparator)+os.Getenv("PATH"))
	workDir := t.TempDir()
	pdfPath := filepath.Join(t.TempDir(), "exam.pdf")
	if err := os.WriteFile(pdfPath, []byte("%PDF-1.7"), 0600); err != nil {
		t.Fatal(err)
	}
	entry := corpus.Entry{
		Name: "exam.pdf", SHA256: strings.Repeat("b", 64), Year: 2024,
		Booklet: "blue", Type: "prova", Source: "INEP",
		Split: "train", CorpusVersion: "v1",
	}
	analyzer := &fakePageAnalyzer{}
	options := DIOptions{WorkDir: workDir, MonthlyPages: 1}
	first, err := ExtractWithDocumentIntelligence(context.Background(), analyzer, pdfPath, entry, options)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ExtractWithDocumentIntelligence(context.Background(), analyzer, pdfPath, entry, options)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || len(second) != 1 || analyzer.calls.Load() != 1 {
		t.Fatalf("fallback calls/results = %d / %d / %d", len(first), len(second), analyzer.calls.Load())
	}
}

func TestExtractAnswersWithDocumentIntelligence(t *testing.T) {
	tools := fakePDFTools(t, "")
	if err := os.WriteFile(filepath.Join(tools, "pdfinfo"), []byte("#!/bin/sh\nprintf 'Pages:          1\\n'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", tools+string(os.PathListSeparator)+os.Getenv("PATH"))
	entry := corpus.Entry{
		Name: "key.pdf", SHA256: strings.Repeat("c", 64), Year: 2024,
		Booklet: "blue", Type: "gabarito", Source: "INEP",
		Split: "train", CorpusVersion: "v1",
	}
	analyzer := &fakePageAnalyzer{text: "Questão 1 A\nQuestão 2 C"}
	answers, err := ExtractAnswersWithDocumentIntelligence(
		context.Background(), analyzer, "key.pdf", entry,
		DIOptions{WorkDir: t.TempDir(), MonthlyPages: 10},
	)
	if err != nil {
		t.Fatal(err)
	}
	if answers[1] != "A" || answers[2] != "C" {
		t.Fatalf("unexpected answer key: %#v", answers)
	}
}

func TestDocumentIntelligenceRESTClientUsesEntraAndPolls(t *testing.T) {
	var analyzeCalls atomic.Int32
	var pollCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("missing Entra bearer token")
		}
		if r.Method == http.MethodPost {
			if analyzeCalls.Add(1) == 1 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			if r.URL.Query().Get("api-version") != documentIntelligenceAPIVersion {
				t.Errorf("API version = %q", r.URL.Query().Get("api-version"))
			}
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if body["base64Source"] != base64.StdEncoding.EncodeToString([]byte("png")) {
				t.Error("request did not send the source page")
			}
			w.Header().Set("Operation-Location", serverURL(r)+"/poll")
			w.WriteHeader(http.StatusAccepted)
			return
		}
		if pollCalls.Add(1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`{"status":"succeeded","analyzeResult":{"content":"page text"}}`))
	}))
	defer server.Close()
	client := &DocumentIntelligenceClient{
		endpoint: server.URL, credential: fakeTokenCredential{},
		httpClient: server.Client(), pollInterval: 0,
	}
	text, err := client.AnalyzePage(context.Background(), []byte("png"))
	if err != nil || text != "page text" {
		t.Fatalf("AnalyzePage() = %q, %v", text, err)
	}
	if analyzeCalls.Load() != 2 || pollCalls.Load() != 2 {
		t.Fatalf("retry counts = analyze %d, poll %d; want 2 each", analyzeCalls.Load(), pollCalls.Load())
	}
	if _, err := client.AnalyzePage(context.Background(), make([]byte, documentIntelligenceMaxBytes+1)); err == nil {
		t.Fatal("AnalyzePage accepted an oversized page")
	}
}

func serverURL(r *http.Request) string {
	return "http://" + r.Host
}
