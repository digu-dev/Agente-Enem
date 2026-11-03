package validate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

type Validator struct {
	schema *jsonschema.Schema
}

func New(schemaPath string) (*Validator, error) {
	schemaData, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("read schema: %w", err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("question.json", bytes.NewReader(schemaData)); err != nil {
		return nil, fmt.Errorf("load schema: %w", err)
	}
	schema, err := compiler.Compile("question.json")
	if err != nil {
		return nil, fmt.Errorf("compile schema: %w", err)
	}
	return &Validator{schema: schema}, nil
}

func (v *Validator) ValidateJSON(data []byte) error {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("JSON line must contain exactly one value")
	}
	if err := v.schema.Validate(value); err != nil {
		return fmt.Errorf("schema validation: %w", err)
	}
	return nil
}

func (v *Validator) ValidateJSONL(reader io.Reader) (int, error) {
	scanner := newBoundedScanner(reader)
	count := 0
	for scanner.Scan() {
		count++
		if err := v.ValidateJSON(scanner.Bytes()); err != nil {
			return count, fmt.Errorf("line %d: %w", count, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return count, fmt.Errorf("read JSONL: %w", err)
	}
	return count, nil
}
