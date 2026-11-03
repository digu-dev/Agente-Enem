package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractVerifiesManifestIgnoresExtrasAndIsIdempotent(t *testing.T) {
	root := t.TempDir()
	inputDir := filepath.Join(root, "raw")
	workDir := filepath.Join(root, "work")
	if err := os.MkdirAll(inputDir, 0700); err != nil {
		t.Fatal(err)
	}
	pdfData := []byte("%PDF-1.7\n")
	if err := os.WriteFile(filepath.Join(inputDir, "paper.pdf"), pdfData, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inputDir, "unlisted-private-name.pdf"), []byte("%PDF-1.7"), 0600); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(pdfData)
	manifest, err := json.Marshal([]map[string]any{{
		"nome": "paper.pdf", "sha256": hex.EncodeToString(hash[:]), "ano": 2024,
		"caderno": "azul", "tipo": "prova", "fonte": "INEP",
		"split": "train", "corpus_version": "v1",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inputDir, "manifest.json"), manifest, 0600); err != nil {
		t.Fatal(err)
	}
	tools := filepath.Join(root, "tools")
	if err := os.MkdirAll(tools, 0700); err != nil {
		t.Fatal(err)
	}
	counter := filepath.Join(root, "pdftotext-count")
	t.Setenv("PDF_TEXT_COUNTER", counter)
	for name, content := range map[string]string{
		"pdftotext": "#!/bin/sh\nprintf x >> \"$PDF_TEXT_COUNTER\"\nprintf '%b' 'QUESTÃO 1\\nCalcule o resultado.\\n(A) um\\n(B) dois\\n(C) três\\n(D) quatro\\n(E) cinco'\n",
		"pdftoppm":  "#!/bin/sh\nlast=''\nfor arg do last=\"$arg\"; done\nprintf '\\211PNG\\r\\n\\032\\nimage' > \"$last.png\"\n",
	} {
		if err := os.WriteFile(filepath.Join(tools, name), []byte(content), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", tools+string(os.PathListSeparator)+os.Getenv("PATH"))
	schema := filepath.Join("..", "..", "..", "..", "contracts", "questao.schema.json")
	args := []string{"--input", inputDir, "--work-dir", workDir, "--schema", schema}
	if err := extractCommand(context.Background(), args); err != nil {
		t.Fatal(err)
	}
	outputPath := filepath.Join(workDir, "questoes.jsonl")
	first, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := extractCommand(context.Background(), args); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("second extract run changed output")
	}
	counted, err := os.ReadFile(counter)
	if err != nil || len(counted) != 1 {
		t.Fatalf("local parser calls = %d, want one across both runs (err: %v)", len(counted), err)
	}
	ignoredReport, err := os.ReadFile(filepath.Join(workDir, "ignored-files.json"))
	if err != nil || !strings.Contains(string(ignoredReport), `"ignored_count": 1`) ||
		strings.Contains(string(ignoredReport), "unlisted-private-name.pdf") {
		t.Fatalf("unexpected ignored-file report: %s (%v)", ignoredReport, err)
	}
	if err := validateCommand([]string{"--input", outputPath, "--schema", filepath.Join("..", "..", "..", "..", "contracts", "questao.schema.json")}); err != nil {
		t.Fatalf("validate extracted JSONL: %v", err)
	}
}

func TestExtractRejectsManifestHashMismatch(t *testing.T) {
	root := t.TempDir()
	inputDir := filepath.Join(root, "raw")
	if err := os.MkdirAll(inputDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inputDir, "paper.pdf"), []byte("%PDF-1.7\n"), 0600); err != nil {
		t.Fatal(err)
	}
	manifest := `[{"nome":"paper.pdf","sha256":"` + strings.Repeat("0", 64) + `","ano":2024,"caderno":"azul","tipo":"prova","fonte":"INEP","split":"train","corpus_version":"v1"}]`
	if err := os.WriteFile(filepath.Join(inputDir, "manifest.json"), []byte(manifest), 0600); err != nil {
		t.Fatal(err)
	}
	if err := extractCommand(context.Background(), []string{"--input", inputDir, "--work-dir", filepath.Join(root, "work")}); err == nil {
		t.Fatal("extract accepted a PDF whose hash differs from the manifest")
	}
}
