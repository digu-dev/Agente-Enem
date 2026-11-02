package corpus

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const MaxPDFBytes = 50 << 20

type Entry struct {
	Name          string `json:"nome"`
	SHA256        string `json:"sha256"`
	Year          int    `json:"ano"`
	Booklet       string `json:"caderno"`
	Type          string `json:"tipo"`
	Source        string `json:"fonte"`
	Split         string `json:"split"`
	CorpusVersion string `json:"corpus_version"`
}

type Manifest struct {
	Files []Entry `json:"-"`
}

func ReadManifest(path string) (Manifest, error) {
	pathInfo, err := os.Lstat(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("stat manifest path: %w", err)
	}
	if !pathInfo.Mode().IsRegular() {
		return Manifest{}, fmt.Errorf("manifest must be a regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("open manifest: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return Manifest{}, fmt.Errorf("stat manifest: %w", err)
	}
	if !os.SameFile(pathInfo, info) {
		return Manifest{}, fmt.Errorf("manifest changed while opening")
	}
	if !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > 1<<20 {
		return Manifest{}, fmt.Errorf("manifest must be a regular file no larger than 1 MiB")
	}
	data, err := io.ReadAll(io.LimitReader(file, 1<<20+1))
	if err != nil {
		return Manifest{}, fmt.Errorf("read manifest: %w", err)
	}
	if len(data) > 1<<20 {
		return Manifest{}, fmt.Errorf("manifest must be no larger than 1 MiB")
	}
	var files []Entry
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&files); err != nil {
		return Manifest{}, fmt.Errorf("decode manifest: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		if err == nil {
			return Manifest{}, fmt.Errorf("manifest must contain exactly one JSON value")
		}
		return Manifest{}, fmt.Errorf("decode manifest trailing data: %w", err)
	}
	if len(files) == 0 || len(files) > 10000 {
		return Manifest{}, fmt.Errorf("manifest cannot contain more than 10000 files")
	}
	seen := make(map[string]bool, len(files))
	for i := range files {
		entry := &files[i]
		if err := validateEntry(*entry); err != nil {
			return Manifest{}, fmt.Errorf("manifest entry %d: %w", i+1, err)
		}
		if seen[entry.Name] {
			return Manifest{}, fmt.Errorf("duplicate manifest file entry")
		}
		seen[entry.Name] = true
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	return Manifest{Files: files}, nil
}

func validateEntry(entry Entry) error {
	if entry.Name == "" || filepath.Base(entry.Name) != entry.Name || strings.ContainsAny(entry.Name, `/\`) || entry.Name == "." {
		return fmt.Errorf("file name must be a simple basename")
	}
	if !strings.EqualFold(filepath.Ext(entry.Name), ".pdf") {
		return fmt.Errorf("manifest file must have a .pdf extension")
	}
	if len(entry.SHA256) != 64 {
		return fmt.Errorf("sha256 must contain 64 hexadecimal characters")
	}
	if _, err := hex.DecodeString(entry.SHA256); err != nil {
		return fmt.Errorf("sha256 must be hexadecimal")
	}
	if entry.Year < 1998 || entry.Year > 2100 || strings.TrimSpace(entry.Booklet) == "" ||
		strings.TrimSpace(entry.Source) == "" || strings.TrimSpace(entry.CorpusVersion) == "" {
		return fmt.Errorf("year, booklet, source, and corpus_version are required")
	}
	if entry.Type != "prova" && entry.Type != "gabarito" {
		return fmt.Errorf("type must be prova or gabarito")
	}
	if entry.Split != "train" && entry.Split != "validation" && entry.Split != "index" && entry.Split != "holdout" {
		return fmt.Errorf("split must be train, validation, index, or holdout")
	}
	return nil
}

func ValidatePDF(path string, entry Entry) ([]byte, error) {
	pathInfo, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("stat input path: %w", err)
	}
	if !pathInfo.Mode().IsRegular() {
		return nil, fmt.Errorf("input must be a regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open input: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat input: %w", err)
	}
	if !os.SameFile(pathInfo, info) {
		return nil, fmt.Errorf("input changed while opening")
	}
	if info.Size() <= 0 || info.Size() > MaxPDFBytes {
		return nil, fmt.Errorf("PDF size must be between 1 byte and %d MiB", MaxPDFBytes>>20)
	}
	data, err := io.ReadAll(io.LimitReader(file, MaxPDFBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read input: %w", err)
	}
	if int64(len(data)) > MaxPDFBytes {
		return nil, fmt.Errorf("PDF size exceeds %d MiB", MaxPDFBytes>>20)
	}
	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		return nil, fmt.Errorf("input is not a PDF")
	}
	digest := sha256.Sum256(data)
	if !strings.EqualFold(hex.EncodeToString(digest[:]), entry.SHA256) {
		return nil, fmt.Errorf("manifest sha256 mismatch")
	}
	return data, nil
}

func FileHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
