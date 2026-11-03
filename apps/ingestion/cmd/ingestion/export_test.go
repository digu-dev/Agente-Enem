package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/digu-dev/Agente-Enem/apps/ingestion/internal/records"
)

func TestExportIdempotentlySeparatesHoldout(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "labeled.jsonl")
	schema := filepath.Join("..", "..", "..", "..", "contracts", "questao.schema.json")
	questions := make([]records.Question, 10)
	for i := range questions {
		split := "train"
		if i == 1 {
			split = "holdout"
		}
		skill, competency, answer := "H1", "C1", "A"
		questions[i] = records.Question{
			ID: fmt.Sprintf("enem-test-q%02d", i), File: "test.pdf", Page: 1,
			CorpusVersion: "v1", Year: 2024, Booklet: "blue", Type: "prova",
			Source: "INEP", Split: split, Number: i + 1,
			Statement: "Statement", Alternatives: []records.Alternative{
				{Letter: "A", Text: "one"}, {Letter: "B", Text: "two"},
				{Letter: "C", Text: "three"}, {Letter: "D", Text: "four"},
				{Letter: "E", Text: "five"},
			},
			Answer: &answer, Skill: &skill, Competency: &competency,
			Contents: []string{"functions"}, Images: []string{},
		}
	}
	if err := records.WriteJSONL(input, questions); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, "export")
	if err := exportRecords(input, root, output, schema); err != nil {
		t.Fatal(err)
	}
	processedPath := filepath.Join(output, "enem-processed", "questoes.jsonl")
	goldenPath := filepath.Join(output, "enem-golden", "questoes.jsonl")
	reviewPath := filepath.Join(output, "enem-processed", "quality-review.jsonl")
	before := readTestFile(t, processedPath)
	golden := readTestFile(t, goldenPath)
	review := readTestFile(t, reviewPath)
	if strings.Count(string(before), "\n") != 9 {
		t.Fatalf("processed records = %d, want 9", strings.Count(string(before), "\n"))
	}
	if strings.Count(string(golden), "\n") != 1 {
		t.Fatalf("golden records = %d, want 1", strings.Count(string(golden), "\n"))
	}
	if strings.Count(string(review), "\n") != 1 {
		t.Fatalf("review sample records = %d, want 1", strings.Count(string(review), "\n"))
	}
	if err := exportRecords(input, root, output, schema); err != nil {
		t.Fatal(err)
	}
	if after := readTestFile(t, processedPath); !bytes.Equal(before, after) {
		t.Fatal("second export changed processed output")
	}
	if after := readTestFile(t, goldenPath); !bytes.Equal(golden, after) {
		t.Fatal("second export changed golden output")
	}
	if after := readTestFile(t, reviewPath); !bytes.Equal(review, after) {
		t.Fatal("second export changed review output")
	}
}

func readTestFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestCopyImageKeepsReferencesBoundToQuestionAndSourceRoot(t *testing.T) {
	root := t.TempDir()
	imageRoot := filepath.Join(root, "work")
	if err := os.MkdirAll(filepath.Join(imageRoot, "images"), 0700); err != nil {
		t.Fatal(err)
	}
	imageData := []byte("\x89PNG\r\n\x1a\nimage")
	relative := filepath.ToSlash(filepath.Join("images", "enem-test-q1-p1.png"))
	source := filepath.Join(imageRoot, filepath.FromSlash(relative))
	if err := os.WriteFile(source, imageData, 0600); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(root, "output", "enem-processed")
	if err := copyImage(imageRoot, destination, "enem-test-q1", relative); err != nil {
		t.Fatal(err)
	}
	if err := copyImage(imageRoot, destination, "enem-other-q2", relative); err == nil {
		t.Fatal("copyImage accepted an image associated with another question")
	}
	outside := filepath.Join(root, "outside.png")
	if err := os.WriteFile(outside, imageData, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(source); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, source); err != nil {
		t.Fatal(err)
	}
	if err := copyImage(imageRoot, destination, "enem-test-q1", relative); err == nil {
		t.Fatal("copyImage followed a symlink outside the image root")
	}
}
