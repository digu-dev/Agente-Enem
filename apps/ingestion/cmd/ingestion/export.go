package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/digu-dev/Agente-Enem/apps/ingestion/internal/records"
	"github.com/digu-dev/Agente-Enem/apps/ingestion/internal/validate"
)

type stagedJSONL struct {
	file *os.File
	temp string
	path string
}

func newStagedJSONL(path string) (*stagedJSONL, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".jsonl-*")
	if err != nil {
		return nil, err
	}
	if err := file.Chmod(0600); err != nil {
		file.Close()
		os.Remove(file.Name())
		return nil, err
	}
	return &stagedJSONL{file: file, temp: file.Name(), path: path}, nil
}

func (s *stagedJSONL) write(question records.Question) error {
	encoder := json.NewEncoder(s.file)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(question)
}

func (s *stagedJSONL) finish() error {
	if err := s.file.Close(); err != nil {
		return err
	}
	return os.Rename(s.temp, s.path)
}

func (s *stagedJSONL) discard() {
	_ = s.file.Close()
	_ = os.Remove(s.temp)
}

func exportRecords(inputPath, imageDir, outputDir, schemaPath string) error {
	validator, err := validate.New(schemaPath)
	if err != nil {
		return err
	}
	input, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("open input JSONL: %w", err)
	}
	expectedTotal, validationErr := validator.ValidateJSONL(input)
	closeErr := input.Close()
	if validationErr != nil {
		return validationErr
	}
	if closeErr != nil {
		return fmt.Errorf("close validated input JSONL: %w", closeErr)
	}
	processedDir := filepath.Join(outputDir, "enem-processed")
	goldenDir := filepath.Join(outputDir, "enem-golden")
	processed, err := newStagedJSONL(filepath.Join(processedDir, "questoes.jsonl"))
	if err != nil {
		return fmt.Errorf("prepare processed JSONL: %w", err)
	}
	defer processed.discard()
	golden, err := newStagedJSONL(filepath.Join(goldenDir, "questoes.jsonl"))
	if err != nil {
		return fmt.Errorf("prepare golden JSONL: %w", err)
	}
	defer golden.discard()
	processedReview, err := newStagedJSONL(filepath.Join(processedDir, "quality-review.jsonl"))
	if err != nil {
		return fmt.Errorf("prepare processed review sample: %w", err)
	}
	defer processedReview.discard()
	goldenReview, err := newStagedJSONL(filepath.Join(goldenDir, "quality-review.jsonl"))
	if err != nil {
		return fmt.Errorf("prepare golden review sample: %w", err)
	}
	defer goldenReview.discard()
	input, err = os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("open input JSONL: %w", err)
	}
	defer input.Close()
	total, reviewed, processedCount, goldenCount := 0, 0, 0, 0
	processedReviewed, goldenReviewed := 0, 0
	reviewTarget := (expectedTotal*125 + 500) / 1000
	if expectedTotal > 0 && reviewTarget == 0 {
		reviewTarget = 1
	}
	if err := records.ReadJSONL(input, func(question records.Question) error {
		encoded, err := json.Marshal(question)
		if err != nil {
			return err
		}
		if err := validator.ValidateJSON(encoded); err != nil {
			return err
		}
		if question.Skill == nil || strings.TrimSpace(*question.Skill) == "" ||
			question.Competency == nil || strings.TrimSpace(*question.Competency) == "" ||
			len(question.Contents) == 0 {
			return fmt.Errorf("question record is missing pedagogical labels")
		}
		for _, content := range question.Contents {
			if strings.TrimSpace(content) == "" {
				return fmt.Errorf("question record contains an empty content label")
			}
		}
		isGolden := question.Split == "holdout"
		destination := processed
		review := processedReview
		targetDir := processedDir
		if isGolden {
			destination, review, targetDir = golden, goldenReview, goldenDir
		}
		if err := destination.write(question); err != nil {
			return fmt.Errorf("write exported question: %w", err)
		}
		if expectedTotal > 0 && (total+1)*reviewTarget/expectedTotal > total*reviewTarget/expectedTotal {
			if err := review.write(question); err != nil {
				return fmt.Errorf("write quality review sample: %w", err)
			}
			reviewed++
			if isGolden {
				goldenReviewed++
			} else {
				processedReviewed++
			}
		}
		for _, image := range question.Images {
			if err := copyImage(imageDir, targetDir, question.ID, image); err != nil {
				return err
			}
		}
		total++
		if isGolden {
			goldenCount++
		} else {
			processedCount++
		}
		return nil
	}); err != nil {
		return err
	}
	if total != expectedTotal {
		return fmt.Errorf("input JSONL changed during export: validated %d records but read %d", expectedTotal, total)
	}
	if err := processed.finish(); err != nil {
		return fmt.Errorf("finalize processed JSONL: %w", err)
	}
	if err := golden.finish(); err != nil {
		return fmt.Errorf("finalize golden JSONL: %w", err)
	}
	if err := processedReview.finish(); err != nil {
		return fmt.Errorf("finalize processed review sample: %w", err)
	}
	if err := goldenReview.finish(); err != nil {
		return fmt.Errorf("finalize golden review sample: %w", err)
	}
	report := struct {
		TotalQuestions     int     `json:"total_questions"`
		ProcessedQuestions int     `json:"processed_questions"`
		GoldenQuestions    int     `json:"golden_questions"`
		ReviewedQuestions  int     `json:"reviewed_questions"`
		ReviewFraction     float64 `json:"review_fraction"`
	}{total, processedCount, goldenCount, reviewed, 0}
	if total > 0 {
		report.ReviewFraction = float64(reviewed) / float64(total)
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := records.WriteBytesAtomic(filepath.Join(outputDir, "quality-report.json"), append(data, '\n')); err != nil {
		return fmt.Errorf("write quality report: %w", err)
	}
	if err := writeSplitQualityReport(filepath.Join(processedDir, "quality-report.json"), "processed", processedCount, processedReviewed); err != nil {
		return err
	}
	if err := writeSplitQualityReport(filepath.Join(goldenDir, "quality-report.json"), "golden", goldenCount, goldenReviewed); err != nil {
		return err
	}
	fmt.Printf("export: %d processed, %d holdout, %d selected for manual review\n", processedCount, goldenCount, reviewed)
	return nil
}

func writeSplitQualityReport(path, split string, total, reviewed int) error {
	fraction := 0.0
	if total > 0 {
		fraction = float64(reviewed) / float64(total)
	}
	data, err := json.MarshalIndent(struct {
		Split             string  `json:"split"`
		TotalQuestions    int     `json:"total_questions"`
		ReviewedQuestions int     `json:"reviewed_questions"`
		ReviewFraction    float64 `json:"review_fraction"`
	}{split, total, reviewed, fraction}, "", "  ")
	if err != nil {
		return err
	}
	if err := records.WriteBytesAtomic(path, append(data, '\n')); err != nil {
		return fmt.Errorf("write %s quality report: %w", split, err)
	}
	return nil
}

func copyImage(sourceRoot, destinationRoot, questionID, relativePath string) error {
	clean := filepath.Clean(filepath.FromSlash(relativePath))
	if filepath.IsAbs(clean) || clean == "." || clean == ".." ||
		strings.HasPrefix(clean, ".."+string(filepath.Separator)) ||
		!strings.HasPrefix(clean, "images"+string(filepath.Separator)) {
		return errors.New("question image path is invalid")
	}
	base := filepath.Base(clean)
	if !strings.HasPrefix(base, questionID+"-p") || !strings.HasSuffix(base, ".png") {
		return errors.New("question image must be named with its question id")
	}
	resolvedRoot, err := filepath.EvalSymlinks(sourceRoot)
	if err != nil {
		return fmt.Errorf("resolve extracted image directory: %w", err)
	}
	resolvedSource, err := filepath.EvalSymlinks(filepath.Join(resolvedRoot, clean))
	if err != nil {
		return fmt.Errorf("resolve extracted image: %w", err)
	}
	relativeSource, err := filepath.Rel(resolvedRoot, resolvedSource)
	if err != nil || relativeSource == ".." || strings.HasPrefix(relativeSource, ".."+string(filepath.Separator)) {
		return errors.New("extracted image resolves outside the image directory")
	}
	source, err := os.Open(resolvedSource)
	if err != nil {
		return fmt.Errorf("open extracted image: %w", err)
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil {
		return fmt.Errorf("stat extracted image: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() <= 8 || info.Size() > 8<<20 {
		return errors.New("extracted image has invalid type or size")
	}
	header := make([]byte, 8)
	if _, err := io.ReadFull(source, header); err != nil || !bytes.Equal(header, []byte("\x89PNG\r\n\x1a\n")) {
		return errors.New("extracted image is not a PNG")
	}
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("rewind extracted image: %w", err)
	}
	destination := filepath.Join(destinationRoot, clean)
	if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
		return fmt.Errorf("create image output directory: %w", err)
	}
	temp, err := os.CreateTemp(filepath.Dir(destination), ".image-*")
	if err != nil {
		return err
	}
	name := temp.Name()
	defer os.Remove(name)
	if err := temp.Chmod(0600); err != nil {
		temp.Close()
		return err
	}
	if _, err := io.Copy(temp, io.LimitReader(source, 8<<20+1)); err != nil {
		temp.Close()
		return fmt.Errorf("copy extracted image: %w", err)
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(name, destination)
}
