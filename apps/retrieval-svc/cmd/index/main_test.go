package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIndexURL(t *testing.T) {
	tests := []struct {
		endpoint string
		valid    bool
	}{
		{"https://enem-search.search.windows.net", true},
		{"https://enem-search.search.windows.net/", true},
		{"http://enem-search.search.windows.net", false},
		{"https://enem-search.search.windows.net.evil.example", false},
		{"https://user@enem-search.search.windows.net", false},
		{"https://enem-search.search.windows.net:443", false},
		{"https://enem-search.search.windows.net/?x=y", false},
		{"https://enem-search.search.windows.net/other", false},
	}
	for _, tt := range tests {
		t.Run(tt.endpoint, func(t *testing.T) {
			got, err := indexURL(tt.endpoint)
			if (err == nil) != tt.valid {
				t.Fatalf("indexURL(%q) = %q, %v", tt.endpoint, got, err)
			}
			if tt.valid && !strings.HasSuffix(got, "/indexes/questoes-v1?api-version=2024-07-01") {
				t.Fatalf("unexpected URL %q", got)
			}
		})
	}
}

func TestVersionedSchema(t *testing.T) {
	data, err := os.ReadFile("../../index/questoes-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Name   string `json:"name"`
		Fields []struct {
			Name       string `json:"name"`
			Filterable bool   `json:"filterable"`
			Dimensions int    `json:"dimensions"`
			Profile    string `json:"vectorSearchProfile"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	if schema.Name != "questoes-v1" {
		t.Fatalf("unexpected index name %q", schema.Name)
	}
	fields := make(map[string]bool)
	vector := false
	for _, field := range schema.Fields {
		if field.Filterable {
			fields[field.Name] = true
		}
		if field.Name == "vetor" {
			vector = field.Dimensions == 256 && field.Profile == "questoes-hnsw"
		}
	}
	for _, name := range []string{"ano", "habilidade", "conteudos", "origem", "corpus_version"} {
		if !fields[name] {
			t.Errorf("%s must be filterable", name)
		}
	}
	if !vector {
		t.Error("vector field must use the expected dimensions and profile")
	}
}

func TestRunCallsAzureCLI(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "args")
	az := filepath.Join(dir, "az")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$ARGUMENT_FILE\"\n"
	if err := os.WriteFile(az, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("ARGUMENT_FILE", output)
	schema := filepath.Join(dir, "schema.json")
	if err := os.WriteFile(schema, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run("https://example.search.windows.net", schema); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	want := "rest\n--method\nput\n--uri\nhttps://example.search.windows.net/indexes/questoes-v1?api-version=2024-07-01\n--resource\nhttps://search.azure.com\n--body\n@" + schema + "\n--only-show-errors\n"
	if string(got) != want {
		t.Fatalf("arguments = %q, want %q", got, want)
	}
}
