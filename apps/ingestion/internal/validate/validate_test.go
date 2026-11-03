package validate

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateJSONGolden(t *testing.T) {
	validator, err := New(schemaPath())
	if err != nil {
		t.Fatal(err)
	}
	golden, err := os.ReadFile(filepath.Join("..", "parse", "testdata", "questions.golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	var questions []map[string]any
	if err := json.Unmarshal(golden, &questions); err != nil {
		t.Fatal(err)
	}
	if len(questions) != 1 {
		t.Fatalf("parser golden questions = %d, want 1", len(questions))
	}
	question := questions[0]
	question["id"] = "enem-2024-caderno-7-q7"
	question["arquivo"] = "prova.pdf"
	question["pagina"] = 12
	question["corpus_version"] = "v1"
	question["ano"] = 2024
	question["caderno"] = "7"
	question["tipo"] = "prova"
	question["fonte"] = "INEP"
	question["split"] = "train"
	question["gabarito"] = "B"
	question["habilidade"] = nil
	question["competencia"] = nil
	question["conteudos"] = []string{}
	question["imagens"] = []string{}
	goldenRecord, err := json.Marshal(question)
	if err != nil {
		t.Fatal(err)
	}
	if err := validator.ValidateJSON(goldenRecord); err != nil {
		t.Fatalf("valid question rejected: %v", err)
	}
}

func TestValidateJSONRejectsInvalidRecords(t *testing.T) {
	validator, err := New(schemaPath())
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{
		`{"id":"x"}`,
		`{"id":"x","unexpected":true}`,
		`{"id":"x"} {}`,
		`{"id":"x","ano":"2024"}`,
	} {
		if err := validator.ValidateJSON([]byte(value)); err == nil {
			t.Errorf("ValidateJSON(%s) = nil, want error", value)
		}
	}
}

func TestValidateJSONL(t *testing.T) {
	validator, err := New(schemaPath())
	if err != nil {
		t.Fatal(err)
	}
	if count, err := validator.ValidateJSONL(strings.NewReader("")); err != nil || count != 0 {
		t.Fatalf("empty JSONL = (%d, %v), want (0, nil)", count, err)
	}
	if count, err := validator.ValidateJSONL(strings.NewReader("{bad}\n")); err == nil || count != 1 {
		t.Fatalf("invalid JSONL = (%d, %v), want (1, error)", count, err)
	}
}

func TestNewRejectsMissingAndInvalidSchema(t *testing.T) {
	if _, err := New(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatal("New accepted a missing schema")
	}
	path := filepath.Join(t.TempDir(), "schema.json")
	if err := os.WriteFile(path, []byte(`{"type":`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := New(path); err == nil {
		t.Fatal("New accepted an invalid schema")
	}
}

func TestValidateJSONLReturnsReaderError(t *testing.T) {
	validator, err := New(schemaPath())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := validator.ValidateJSONL(failingReader{}); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("ValidateJSONL() error = %v, want reader error", err)
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) {
	return 0, os.ErrPermission
}

func schemaPath() string {
	return filepath.Join("..", "..", "..", "..", "contracts", "questao.schema.json")
}
