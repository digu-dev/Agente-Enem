package extract

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/digu-dev/Agente-Enem/apps/ingestion/internal/corpus"
)

func TestLocalPDFExtractsQuestionAndRendersPage(t *testing.T) {
	tools := fakePDFTools(t, `QUESTÃO 1
Calcule o valor solicitado.
(A) um
(B) dois
(C) três
(D) quatro
(E) cinco`)
	t.Setenv("PATH", tools+string(os.PathListSeparator)+os.Getenv("PATH"))
	workDir := t.TempDir()
	pdfPath := filepath.Join(t.TempDir(), "exam.pdf")
	if err := os.WriteFile(pdfPath, []byte("%PDF-1.7"), 0600); err != nil {
		t.Fatal(err)
	}
	entry := corpus.Entry{
		Name: "exam.pdf", SHA256: strings.Repeat("a", 64), Year: 2024,
		Booklet: "blue", Type: "prova", Source: "INEP",
		Split: "train", CorpusVersion: "v1",
	}
	got, err := (LocalPDF{WorkDir: workDir}).Extract(context.Background(), pdfPath, entry)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Number != 1 || got[0].Page != 1 || len(got[0].Images) != 1 {
		t.Fatalf("unexpected extracted questions: %#v", got)
	}
	image, err := os.ReadFile(filepath.Join(workDir, filepath.FromSlash(got[0].Images[0])))
	if err != nil || len(image) < 8 {
		t.Fatalf("rendered image missing or invalid: %v", err)
	}
}

func TestLocalPDFRequiresPopplerWhenTextExtractionUnavailable(t *testing.T) {
	emptyPath := t.TempDir()
	t.Setenv("PATH", emptyPath)
	_, err := (LocalPDF{WorkDir: t.TempDir()}).Extract(
		context.Background(), "source.pdf", corpus.Entry{},
	)
	if err == nil || !strings.Contains(err.Error(), "pdftotext is required") {
		t.Fatalf("Extract() error = %v, want missing pdftotext", err)
	}
}

func fakePDFTools(t *testing.T, text string) string {
	t.Helper()
	dir := t.TempDir()
	pdftotext := "#!/bin/sh\nprintf '%s' '" + strings.ReplaceAll(text, "'", "'\\''") + "'\n"
	pdftoppm := "#!/bin/sh\nlast=''\nfor arg do last=\"$arg\"; done\nprintf '\\211PNG\\r\\n\\032\\nimage' > \"$last.png\"\n"
	for name, content := range map[string]string{"pdftotext": pdftotext, "pdftoppm": pdftoppm} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0700); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}
