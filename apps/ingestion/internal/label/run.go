package label

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/digu-dev/Agente-Enem/apps/ingestion/internal/records"
	"github.com/digu-dev/Agente-Enem/apps/ingestion/internal/validate"
)

type Options struct {
	Workers   int
	RPM       int
	TPM       int
	MaxTokens int
	DryRun    bool
	CacheDir  string
}

type labelJob struct {
	index    int
	question records.Question
	tokens   int
}

type labelResult struct {
	index    int
	question records.Question
	err      error
}

func Run(ctx context.Context, inputPath, outputPath, schemaPath string, client Client, options Options) (int, int, error) {
	if options.Workers < 1 || options.RPM < 1 || options.TPM < 1 || options.MaxTokens < 0 {
		return 0, 0, errors.New("workers, rpm, and tpm must be positive; max-tokens cannot be negative")
	}
	validator, err := validate.New(schemaPath)
	if err != nil {
		return 0, 0, err
	}
	input, err := os.Open(inputPath)
	if err != nil {
		return 0, 0, fmt.Errorf("open input JSONL: %w", err)
	}
	count, err := validator.ValidateJSONL(input)
	input.Close()
	if err != nil {
		return count, 0, err
	}
	input, err = os.Open(inputPath)
	if err != nil {
		return count, 0, fmt.Errorf("reopen input JSONL: %w", err)
	}
	totalTokens := 0
	if err := records.ReadJSONL(input, func(question records.Question) error {
		totalTokens += estimateTokens(question)
		return nil
	}); err != nil {
		input.Close()
		return count, totalTokens, err
	}
	input.Close()
	if options.MaxTokens > 0 && totalTokens > options.MaxTokens {
		return count, totalTokens, fmt.Errorf("estimated input tokens %d exceed --max-tokens %d", totalTokens, options.MaxTokens)
	}
	if options.DryRun {
		return count, totalTokens, nil
	}
	if client == nil {
		return 0, totalTokens, errors.New("label client is required unless --dry-run is set")
	}
	if options.CacheDir == "" {
		options.CacheDir = filepath.Join(filepath.Dir(outputPath), "cache")
	}
	if err := os.MkdirAll(options.CacheDir, 0700); err != nil {
		return 0, totalTokens, fmt.Errorf("create label cache: %w", err)
	}
	writer, err := records.NewJSONLWriter(outputPath)
	if err != nil {
		return count, totalTokens, fmt.Errorf("prepare labeled JSONL: %w", err)
	}
	defer writer.Abort()
	limiter := NewRateLimiter(options.RPM, options.TPM)
	if retryAware, ok := client.(interface {
		SetRetryRateLimiter(func(context.Context, int) error)
	}); ok {
		retryAware.SetRetryRateLimiter(limiter.Wait)
	}
	maxInflight := max(options.Workers*2, 1)
	jobs := make(chan labelJob, maxInflight)
	results := make(chan labelResult, maxInflight)
	inflight := make(chan struct{}, maxInflight)
	var wg sync.WaitGroup
	var errorMu sync.Mutex
	var firstErr error
	workerContext, cancel := context.WithCancel(ctx)
	defer cancel()
	setError := func(err error) {
		if err == nil {
			return
		}
		errorMu.Lock()
		if firstErr == nil {
			firstErr = err
			cancel()
		}
		errorMu.Unlock()
	}
	getError := func() error {
		errorMu.Lock()
		defer errorMu.Unlock()
		return firstErr
	}
	for i := 0; i < options.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range jobs {
				result := labelQuestion(workerContext, task, client, limiter, options.CacheDir, validator)
				if result.err != nil && workerContext.Err() == nil {
					setError(result.err)
				}
				results <- result
			}
		}()
	}
	go func() {
		wg.Wait()
		close(results)
	}()
	dispatchErr := make(chan error, 1)
	go func() {
		defer close(jobs)
		file, openErr := os.Open(inputPath)
		if openErr != nil {
			dispatchErr <- fmt.Errorf("reopen input JSONL: %w", openErr)
			return
		}
		defer file.Close()
		index := 0
		dispatchErr <- records.ReadJSONL(file, func(question records.Question) error {
			select {
			case inflight <- struct{}{}:
			case <-workerContext.Done():
				return workerContext.Err()
			}
			select {
			case jobs <- labelJob{index: index, question: question, tokens: estimateTokens(question)}:
				index++
				return nil
			case <-workerContext.Done():
				<-inflight
				return workerContext.Err()
			}
		})
	}()
	pending := make(map[int]labelResult, maxInflight)
	next := 0
	for result := range results {
		pending[result.index] = result
		for {
			ready, found := pending[next]
			if !found {
				break
			}
			delete(pending, next)
			<-inflight
			if ready.err != nil {
				setError(ready.err)
			} else if getError() == nil {
				if err := writer.Write(ready.question); err != nil {
					setError(fmt.Errorf("write labeled question: %w", err))
				}
			}
			next++
		}
	}
	dispatchError := <-dispatchErr
	if dispatchError != nil && !errors.Is(dispatchError, context.Canceled) {
		setError(dispatchError)
	}
	if err := getError(); err != nil {
		return count, totalTokens, err
	}
	if err := ctx.Err(); err != nil {
		return count, totalTokens, err
	}
	if next != count {
		return count, totalTokens, fmt.Errorf("label pipeline wrote %d of %d records", next, count)
	}
	if err := writer.Commit(); err != nil {
		return count, totalTokens, fmt.Errorf("commit labeled JSONL: %w", err)
	}
	return count, totalTokens, nil
}

func labelQuestion(ctx context.Context, task labelJob, client Client, limiter *RateLimiter, cacheDir string, validator *validate.Validator) labelResult {
	question := task.question
	if question.Skill != nil && question.Competency != nil {
		return labelResult{index: task.index, question: question}
	}
	cachePath := filepath.Join(cacheDir, cacheKey(question)+".json")
	labels, found, err := readCache(cachePath)
	if err != nil {
		return labelResult{index: task.index, err: err}
	}
	if !found {
		if err := limiter.Wait(ctx, task.tokens); err != nil {
			return labelResult{index: task.index, err: err}
		}
		labels, err = client.Label(ctx, question)
		if err != nil {
			return labelResult{index: task.index, err: fmt.Errorf("label request failed: %w", err)}
		}
		labels, err = normalizeLabels(labels)
		if err != nil {
			return labelResult{index: task.index, err: fmt.Errorf("label client returned invalid fields: %w", err)}
		}
		if err := writeCache(cachePath, labels); err != nil {
			return labelResult{index: task.index, err: err}
		}
	}
	skill, competency := labels.Skill, labels.Competency
	question.Skill, question.Competency = &skill, &competency
	question.Contents = labels.Contents
	data, err := json.Marshal(question)
	if err == nil {
		err = validator.ValidateJSON(data)
	}
	if err != nil {
		return labelResult{index: task.index, err: fmt.Errorf("labeled record failed schema validation: %w", err)}
	}
	return labelResult{index: task.index, question: question}
}

func estimateTokens(question records.Question) int {
	characters := len(question.Statement)
	for _, alternative := range question.Alternatives {
		characters += len(alternative.Text)
	}
	return characters + 512 + maxOutputTokens
}

func cacheKey(question records.Question) string {
	data, _ := json.Marshal(struct {
		PromptVersion string                `json:"prompt_version"`
		Provider      string                `json:"provider"`
		Model         string                `json:"model"`
		ID            string                `json:"id"`
		Statement     string                `json:"enunciado"`
		Alternatives  []records.Alternative `json:"alternativas"`
	}{
		PromptVersion: "classification-v1",
		Provider:      os.Getenv("AZURE_OPENAI_ENDPOINT") + os.Getenv("LLM_BASE_URL"),
		Model:         os.Getenv("AZURE_OPENAI_DEPLOYMENT") + os.Getenv("LLM_MODEL"),
		ID:            question.ID,
		Statement:     question.Statement,
		Alternatives:  question.Alternatives,
	})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func readCache(path string) (Labels, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Labels{}, false, nil
	}
	if err != nil {
		return Labels{}, false, fmt.Errorf("read label cache: %w", err)
	}
	var labels Labels
	if err := json.Unmarshal(data, &labels); err != nil || labels.Skill == "" || labels.Competency == "" || labels.Contents == nil {
		return Labels{}, false, errors.New("label cache entry is invalid")
	}
	return labels, true, nil
}

func writeCache(path string, labels Labels) error {
	data, err := json.Marshal(labels)
	if err != nil {
		return err
	}
	if err := records.WriteBytesAtomic(path, append(data, '\n')); err != nil {
		return fmt.Errorf("write label cache: %w", err)
	}
	return nil
}
