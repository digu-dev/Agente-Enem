package records

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func ReadJSONL(reader io.Reader, consume func(Question) error) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		var question Question
		if err := json.Unmarshal(scanner.Bytes(), &question); err != nil {
			return fmt.Errorf("decode JSONL line %d: %w", line, err)
		}
		if err := consume(question); err != nil {
			return fmt.Errorf("process JSONL line %d: %w", line, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read JSONL: %w", err)
	}
	return nil
}

func WriteJSONL(path string, questions []Question) error {
	writer, err := NewJSONLWriter(path)
	if err != nil {
		return err
	}
	for _, question := range questions {
		if err := writer.Write(question); err != nil {
			writer.Abort()
			return err
		}
	}
	return writer.Commit()
}

type JSONLWriter struct {
	file    *os.File
	encoder *json.Encoder
	temp    string
	path    string
}

func NewJSONLWriter(path string) (*JSONLWriter, error) {
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
	encoder := json.NewEncoder(file)
	encoder.SetEscapeHTML(false)
	return &JSONLWriter{file: file, encoder: encoder, temp: file.Name(), path: path}, nil
}

func (w *JSONLWriter) Write(question Question) error {
	return w.encoder.Encode(question)
}

func (w *JSONLWriter) Commit() error {
	if err := w.file.Close(); err != nil {
		return err
	}
	return os.Rename(w.temp, w.path)
}

func (w *JSONLWriter) Abort() {
	_ = w.file.Close()
	_ = os.Remove(w.temp)
}

func WriteBytesAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tmp := file.Name()
	defer os.Remove(tmp)
	if err := file.Chmod(0600); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
