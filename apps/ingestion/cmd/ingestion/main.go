package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/digu-dev/Agente-Enem/apps/ingestion/internal/blob"
	"github.com/digu-dev/Agente-Enem/apps/ingestion/internal/corpus"
	"github.com/digu-dev/Agente-Enem/apps/ingestion/internal/extract"
	"github.com/digu-dev/Agente-Enem/apps/ingestion/internal/label"
	"github.com/digu-dev/Agente-Enem/apps/ingestion/internal/parse"
	"github.com/digu-dev/Agente-Enem/apps/ingestion/internal/records"
	"github.com/digu-dev/Agente-Enem/apps/ingestion/internal/validate"
)

const defaultSchema = "../../contracts/questao.schema.json"

type answerKey struct {
	hash    string
	answers map[int]string
}

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}
	if args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printUsage()
		return nil
	}
	if len(args) > 1 && (args[1] == "--help" || args[1] == "-h") {
		printCommandUsage(args[0])
		return nil
	}
	switch args[0] {
	case "extract":
		return extractCommand(ctx, args[1:])
	case "validate":
		return validateCommand(args[1:])
	case "label":
		return labelCommand(ctx, args[1:])
	case "export":
		return exportCommand(ctx, args[1:])
	default:
		return fmt.Errorf("unknown command %q; expected extract, validate, label, or export", args[0])
	}
}

func printUsage() {
	fmt.Fprintln(os.Stdout, "Usage: ingestion <extract|validate|label|export> [flags]")
	fmt.Fprintln(os.Stdout, "Run 'ingestion <command> --help' for command flags.")
}

func printCommandUsage(command string) {
	switch command {
	case "extract":
		fmt.Fprintln(os.Stdout, "Usage: ingestion extract [--input data/raw] [--work-dir data/work] [--blob] [--schema contracts/questao.schema.json]")
	case "validate":
		fmt.Fprintln(os.Stdout, "Usage: ingestion validate [--input questoes.jsonl] [--schema contracts/questao.schema.json]")
	case "label":
		fmt.Fprintln(os.Stdout, "Usage: ingestion label [--input questoes.jsonl] [--output labeled.jsonl] [--workers N] [--rpm N] [--tpm N] [--max-tokens N] [--dry-run]")
	case "export":
		fmt.Fprintln(os.Stdout, "Usage: ingestion export [--input labeled.jsonl] [--image-dir data/work] [--output data] [--upload]")
	default:
		printUsage()
	}
}

func extractCommand(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("extract", flag.ContinueOnError)
	inputDir := flags.String("input", "../../data/raw", "directory containing manifest.json and PDFs")
	workDir := flags.String("work-dir", "../../data/work", "working directory for extraction output")
	useBlob := flags.Bool("blob", false, "read the manifest and listed PDFs from Azure Blob")
	schemaPath := flags.String("schema", defaultSchema, "JSON Schema path")
	if err := flags.Parse(args); err != nil {
		return err
	}
	rawDir := *inputDir
	var err error
	var blobStore *blob.Store
	var blobNames []string
	containerName := os.Getenv("RAW_CONTAINER")
	if containerName == "" {
		containerName = "enem-raw"
	}
	if *useBlob {
		blobStore, err = blob.NewFromEnvironment()
		if err != nil {
			return err
		}
		blobNames, err = blobStore.List(ctx, containerName)
		if err != nil {
			return err
		}
		rawDir = filepath.Join(*workDir, "raw-input")
		if err := os.MkdirAll(rawDir, 0700); err != nil {
			return fmt.Errorf("create Blob download directory: %w", err)
		}
		if err := blobStore.DownloadFile(ctx, containerName, "manifest.json", filepath.Join(rawDir, "manifest.json"), 1<<20); err != nil {
			return err
		}
	}
	manifest, err := corpus.ReadManifest(filepath.Join(rawDir, "manifest.json"))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(*workDir, "images"), 0700); err != nil {
		return fmt.Errorf("create work directory: %w", err)
	}
	entries := make(map[string]corpus.Entry, len(manifest.Files))
	for _, entry := range manifest.Files {
		entries[entry.Name] = entry
	}
	extras := make([]string, 0)
	if *useBlob {
		for _, name := range blobNames {
			if name != "manifest.json" {
				if _, found := entries[name]; !found {
					sum := sha256.Sum256([]byte(name))
					extras = append(extras, hex.EncodeToString(sum[:]))
				}
			}
		}
	} else {
		dirEntries, err := os.ReadDir(rawDir)
		if err != nil {
			return fmt.Errorf("read input directory: %w", err)
		}
		for _, entry := range dirEntries {
			if entry.Name() != "manifest.json" {
				if _, found := entries[entry.Name()]; !found {
					sum := sha256.Sum256([]byte(entry.Name()))
					extras = append(extras, hex.EncodeToString(sum[:]))
				}
			}
		}
	}
	sort.Strings(extras)
	report, err := json.MarshalIndent(struct {
		IgnoredCount int      `json:"ignored_count"`
		NameHashes   []string `json:"name_sha256"`
	}{len(extras), extras}, "", "  ")
	if err != nil {
		return err
	}
	if err := records.WriteBytesAtomic(filepath.Join(*workDir, "ignored-files.json"), append(report, '\n')); err != nil {
		return fmt.Errorf("write ignored file report: %w", err)
	}
	outputValidator, err := validate.New(*schemaPath)
	if err != nil {
		return err
	}

	answerKeys := make(map[string]answerKey)
	for _, entry := range manifest.Files {
		if entry.Type != "gabarito" {
			continue
		}
		path := filepath.Join(rawDir, entry.Name)
		if *useBlob {
			if err := blobStore.DownloadFile(ctx, containerName, entry.Name, path, corpus.MaxPDFBytes); err != nil {
				return err
			}
		}
		data, err := corpus.ValidatePDF(path, entry)
		if err != nil {
			return fmt.Errorf("manifest file validation failed: %w", err)
		}
		verifiedPath, err := stageVerifiedPDF(*workDir, data)
		if err != nil {
			return err
		}
		text, err := extract.ReadText(ctx, verifiedPath)
		answers := parse.ParseAnswerKey(text)
		if err != nil || len(answers) == 0 {
			if os.Getenv("DOCUMENT_INTELLIGENCE_ENDPOINT") == "" {
				if err != nil {
					return err
				}
				return errors.New("gabarito PDF contains no parseable answer key")
			}
			analyzer, budget, analyzerErr := documentIntelligenceConfig()
			if analyzerErr != nil {
				return analyzerErr
			}
			answers, err = extract.ExtractAnswersWithDocumentIntelligence(ctx, analyzer, verifiedPath, entry, extract.DIOptions{
				WorkDir: *workDir, MonthlyPages: budget,
			})
			if err != nil {
				return fmt.Errorf("Document Intelligence answer-key fallback failed: %w", err)
			}
		}
		if len(answers) == 0 {
			return errors.New("gabarito PDF contains no parseable answer key")
		}
		if _, duplicate := answerKeys[answerKeyID(entry.Year, entry.Booklet, entry.Split, entry.CorpusVersion)]; duplicate {
			return errors.New("manifest contains multiple gabaritos for the same year, booklet, split, and corpus version")
		}
		answerKeys[answerKeyID(entry.Year, entry.Booklet, entry.Split, entry.CorpusVersion)] = answerKey{
			hash: corpus.FileHash(data), answers: answers,
		}
	}
	writer, err := records.NewJSONLWriter(filepath.Join(*workDir, "questoes.jsonl"))
	if err != nil {
		return fmt.Errorf("prepare extracted JSONL: %w", err)
	}
	defer writer.Abort()
	questionCount := 0
	for _, entry := range manifest.Files {
		if entry.Type != "prova" {
			continue
		}
		path := filepath.Join(rawDir, entry.Name)
		if *useBlob {
			if err := blobStore.DownloadFile(ctx, containerName, entry.Name, path, corpus.MaxPDFBytes); err != nil {
				return err
			}
		}
		data, err := corpus.ValidatePDF(path, entry)
		if err != nil {
			return fmt.Errorf("manifest file validation failed: %w", err)
		}
		verifiedPath, err := stageVerifiedPDF(*workDir, data)
		if err != nil {
			return err
		}
		key := answerKeys[answerKeyID(entry.Year, entry.Booklet, entry.Split, entry.CorpusVersion)]
		checkpointPath := checkpointFilePath(*workDir, entry, key.hash)
		cached, validCache, err := readCheckpoint(checkpointPath, *workDir, entry, key, *schemaPath)
		if err != nil {
			return err
		}
		if validCache {
			for _, question := range cached {
				if err := writer.Write(question); err != nil {
					return fmt.Errorf("write checkpointed question: %w", err)
				}
				questionCount++
			}
			continue
		}
		pdf := extract.LocalPDF{WorkDir: *workDir}
		found, err := pdf.Extract(ctx, verifiedPath, entry)
		if err != nil {
			if os.Getenv("DOCUMENT_INTELLIGENCE_ENDPOINT") == "" {
				return fmt.Errorf("local PDF extraction failed; configure Document Intelligence for the optional fallback: %w", err)
			}
			analyzer, budget, analyzerErr := documentIntelligenceConfig()
			if analyzerErr != nil {
				return analyzerErr
			}
			found, err = extract.ExtractWithDocumentIntelligence(ctx, analyzer, verifiedPath, entry, extract.DIOptions{
				WorkDir: *workDir, MonthlyPages: budget,
			})
			if err != nil {
				return fmt.Errorf("Document Intelligence fallback failed: %w", err)
			}
		}
		for i := range found {
			if key, exists := answerKeys[answerKeyID(found[i].Year, found[i].Booklet, found[i].Split, found[i].CorpusVersion)]; exists {
				if answer, hasAnswer := key.answers[found[i].Number]; hasAnswer {
					found[i].Answer = &answer
				}
				encoded, err := json.Marshal(found[i])
				if err != nil {
					return fmt.Errorf("encode extracted question: %w", err)
				}
				if err := outputValidator.ValidateJSON(encoded); err != nil {
					return fmt.Errorf("extracted question failed schema validation: %w", err)
				}
			}
		}
		sort.Slice(found, func(i, j int) bool { return found[i].ID < found[j].ID })
		if len(found) == 0 {
			return fmt.Errorf("manifest PDF produced no question records")
		}
		if err := records.WriteJSONL(checkpointPath, found); err != nil {
			return fmt.Errorf("write extraction checkpoint: %w", err)
		}
		for _, question := range found {
			if err := writer.Write(question); err != nil {
				return fmt.Errorf("write extracted question: %w", err)
			}
			questionCount++
		}
	}
	if questionCount == 0 {
		return errors.New("no questions were extracted from manifest PDFs")
	}
	if err := writer.Commit(); err != nil {
		return fmt.Errorf("commit extracted JSONL: %w", err)
	}
	fmt.Printf("extract: %d manifest files processed, %d files ignored, %d questions extracted\n", len(manifest.Files), len(extras), questionCount)
	return nil
}

func answerKeyID(year int, booklet, split, corpusVersion string) string {
	return fmt.Sprintf("%d-%s-%s-%s", year, strings.ToLower(strings.TrimSpace(booklet)), split, corpusVersion)
}

func stageVerifiedPDF(workDir string, data []byte) (string, error) {
	hash := corpus.FileHash(data)
	path := filepath.Join(workDir, "inputs", hash+".pdf")
	if err := records.WriteBytesAtomic(path, data); err != nil {
		return "", fmt.Errorf("stage verified PDF: %w", err)
	}
	return path, nil
}

func documentIntelligenceConfig() (extract.PageAnalyzer, int, error) {
	analyzer, err := extract.NewDocumentIntelligenceFromEnvironment()
	if err != nil {
		return nil, 0, err
	}
	budget := 500
	if configuredBudget := os.Getenv("DOCUMENT_INTELLIGENCE_MONTHLY_BUDGET"); configuredBudget != "" {
		budget, err = strconv.Atoi(configuredBudget)
		if err != nil || budget < 1 || budget > 500 {
			return nil, 0, errors.New("DOCUMENT_INTELLIGENCE_MONTHLY_BUDGET must be between 1 and 500")
		}
	}
	return analyzer, budget, nil
}

func checkpointFilePath(workDir string, entry corpus.Entry, answerHash string) string {
	data, _ := json.Marshal(struct {
		ParserVersion string       `json:"parser_version"`
		Entry         corpus.Entry `json:"entry"`
		AnswerHash    string       `json:"answer_hash"`
	}{"parser-v1", entry, answerHash})
	sum := sha256.Sum256(data)
	return filepath.Join(workDir, "checkpoints", hex.EncodeToString(sum[:])+".jsonl")
}

func readCheckpoint(path, workDir string, entry corpus.Entry, key answerKey, schemaPath string) ([]records.Question, bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("stat extraction checkpoint: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() > 64<<20 {
		return nil, false, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, false, fmt.Errorf("open extraction checkpoint: %w", err)
	}
	defer file.Close()
	validator, err := validate.New(schemaPath)
	if err != nil {
		return nil, false, err
	}
	var questions []records.Question
	valid := true
	err = records.ReadJSONL(file, func(question records.Question) error {
		data, err := json.Marshal(question)
		if err != nil || validator.ValidateJSON(data) != nil ||
			question.ID != extract.QuestionID(entry, question.Number) ||
			question.File != entry.Name || question.Year != entry.Year ||
			question.Booklet != entry.Booklet || question.Type != entry.Type ||
			question.Source != entry.Source || question.Split != entry.Split ||
			question.CorpusVersion != entry.CorpusVersion || question.Skill != nil ||
			question.Competency != nil || len(question.Contents) != 0 ||
			question.Page < 1 || len(question.Images) == 0 {
			valid = false
			return nil
		}
		if answer, found := key.answers[question.Number]; found {
			if question.Answer == nil || *question.Answer != answer {
				valid = false
				return nil
			}
		} else if question.Answer != nil {
			valid = false
			return nil
		}
		for _, image := range question.Images {
			clean := filepath.Clean(filepath.FromSlash(image))
			if filepath.IsAbs(clean) || clean == ".." ||
				strings.HasPrefix(clean, ".."+string(filepath.Separator)) ||
				!strings.HasPrefix(clean, "images"+string(filepath.Separator)) ||
				!strings.HasPrefix(filepath.Base(clean), question.ID+"-p") {
				valid = false
				return nil
			}
			root, err := filepath.EvalSymlinks(workDir)
			if err != nil {
				valid = false
				return nil
			}
			resolved, err := filepath.EvalSymlinks(filepath.Join(root, clean))
			if err != nil {
				valid = false
				return nil
			}
			relative, err := filepath.Rel(root, resolved)
			if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
				valid = false
				return nil
			}
			data, err := os.ReadFile(resolved)
			if err != nil || len(data) <= 8 || len(data) > 8<<20 || string(data[:8]) != "\x89PNG\r\n\x1a\n" {
				valid = false
				return nil
			}
		}
		questions = append(questions, question)
		return nil
	})
	if err != nil || !valid || len(questions) == 0 {
		return nil, false, nil
	}
	return questions, true, nil
}

func validateCommand(args []string) error {
	flags := flag.NewFlagSet("validate", flag.ContinueOnError)
	inputPath := flags.String("input", "../../data/work/questoes.jsonl", "JSONL file")
	schemaPath := flags.String("schema", defaultSchema, "JSON Schema path")
	if err := flags.Parse(args); err != nil {
		return err
	}
	validator, err := validate.New(*schemaPath)
	if err != nil {
		return err
	}
	input, err := os.Open(*inputPath)
	if err != nil {
		return fmt.Errorf("open JSONL: %w", err)
	}
	defer input.Close()
	count, err := validator.ValidateJSONL(input)
	if err != nil {
		return err
	}
	fmt.Printf("validate: %d records valid\n", count)
	return nil
}

func labelCommand(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("label", flag.ContinueOnError)
	inputPath := flags.String("input", "../../data/work/questoes.jsonl", "input JSONL")
	outputPath := flags.String("output", "../../data/work/labeled.jsonl", "output JSONL")
	workers := flags.Int("workers", 2, "concurrent model requests")
	rpm := flags.Int("rpm", 5, "maximum requests per minute")
	tpm := flags.Int("tpm", 20000, "maximum input tokens per minute")
	maxTokens := flags.Int("max-tokens", 0, "abort if estimated total input tokens exceed this limit")
	dryRun := flags.Bool("dry-run", false, "estimate token usage without calling a model")
	schemaPath := flags.String("schema", defaultSchema, "JSON Schema path")
	cacheDir := flags.String("cache-dir", "", "label cache directory (default: sibling cache/)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *workers < 1 || *rpm < 1 || *tpm < 1 || *maxTokens < 0 {
		return errors.New("workers, rpm, and tpm must be positive; max-tokens cannot be negative")
	}
	var client label.Client
	if !*dryRun {
		configured, err := label.NewFromEnvironment()
		if err != nil {
			return err
		}
		client = configured
	}
	if err := os.MkdirAll(filepath.Dir(*outputPath), 0700); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	questions, tokens, err := label.Run(ctx, *inputPath, *outputPath, *schemaPath, client, label.Options{
		Workers: *workers, RPM: *rpm, TPM: *tpm, MaxTokens: *maxTokens, DryRun: *dryRun, CacheDir: *cacheDir,
	})
	if err != nil {
		return err
	}
	if *dryRun {
		fmt.Printf("label dry-run: %d records, estimated up to %d total tokens\n", questions, tokens)
	} else {
		fmt.Printf("label: %d records labeled, estimated up to %d total tokens\n", questions, tokens)
	}
	return nil
}

func exportCommand(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("export", flag.ContinueOnError)
	inputPath := flags.String("input", "../../data/work/labeled.jsonl", "input JSONL")
	imageDir := flags.String("image-dir", "../../data/work", "directory containing extracted images")
	outputDir := flags.String("output", "../../data", "root directory for enem-processed and enem-golden")
	schemaPath := flags.String("schema", defaultSchema, "JSON Schema path")
	upload := flags.Bool("upload", false, "upload the exported trees to Azure Blob")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if err := exportRecords(*inputPath, *imageDir, *outputDir, *schemaPath); err != nil {
		return err
	}
	if *upload {
		store, err := blob.NewFromEnvironment()
		if err != nil {
			return err
		}
		processedContainer := os.Getenv("PROCESSED_CONTAINER")
		if processedContainer == "" {
			processedContainer = "enem-processed"
		}
		goldenContainer := os.Getenv("GOLDEN_CONTAINER")
		if goldenContainer == "" {
			goldenContainer = "enem-golden"
		}
		if err := uploadTree(ctx, store, filepath.Join(*outputDir, "enem-processed"), processedContainer); err != nil {
			return err
		}
		return uploadTree(ctx, store, filepath.Join(*outputDir, "enem-golden"), goldenContainer)
	}
	return nil
}

func uploadTree(ctx context.Context, store *blob.Store, root, container string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk export tree: %w", walkErr)
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("export tree cannot contain symbolic links")
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if err := store.UploadFile(ctx, container, filepath.ToSlash(relative), path); err != nil {
			return err
		}
		return nil
	})
}
