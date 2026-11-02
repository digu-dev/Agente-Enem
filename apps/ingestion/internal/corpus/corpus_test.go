package corpus

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadManifest(t *testing.T) {
	hash := strings.Repeat("a", 64)
	path := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(path, []byte(`[{"nome":"b.pdf","sha256":"`+hash+`","ano":2024,"caderno":"azul","tipo":"prova","fonte":"INEP","split":"train","corpus_version":"v1"},{"nome":"a.pdf","sha256":"`+hash+`","ano":2023,"caderno":"azul","tipo":"gabarito","fonte":"INEP","split":"holdout","corpus_version":"v1"}]`), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := ReadManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Files[0].Name != "a.pdf" || got.Files[1].Name != "b.pdf" {
		t.Fatalf("manifest entries were not ordered: %#v", got.Files)
	}
}

func TestReadManifestRejectsInvalidOrDuplicateEntries(t *testing.T) {
	valid := `{"nome":"x.pdf","sha256":"` + strings.Repeat("0", 64) + `","ano":2024,"caderno":"azul","tipo":"prova","fonte":"INEP","split":"train","corpus_version":"v1"}`
	for _, content := range []string{
		`[{"nome":"../escape.pdf"}]`,
		`[` + valid + `,` + valid + `]`,
		`[{"nome":"x.pdf","sha256":"bad","ano":2024,"caderno":"azul","tipo":"prova","fonte":"INEP","split":"train","corpus_version":"v1"}]`,
	} {
		path := filepath.Join(t.TempDir(), "manifest.json")
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadManifest(path); err == nil {
			t.Errorf("ReadManifest(%s) = nil error", content)
		}
	}
}

func TestValidatePDF(t *testing.T) {
	data := []byte("%PDF-1.7\nfake but signature checked\n")
	sum := sha256.Sum256(data)
	entry := Entry{Name: "source.pdf", SHA256: hex.EncodeToString(sum[:])}
	path := filepath.Join(t.TempDir(), entry.Name)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := ValidatePDF(path, entry); err != nil || string(got) != string(data) {
		t.Fatalf("ValidatePDF() = %q, %v", got, err)
	}
	entry.SHA256 = strings.Repeat("0", 64)
	if _, err := ValidatePDF(path, entry); err == nil {
		t.Fatal("ValidatePDF() accepted mismatched hash")
	}
}

func TestValidatePDFRejectsNonPDF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.pdf")
	if err := os.WriteFile(path, []byte("not pdf"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidatePDF(path, Entry{}); err == nil {
		t.Fatal("ValidatePDF() accepted a non-PDF")
	}
}
