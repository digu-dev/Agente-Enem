package parse

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

type Alternative struct {
	Letter string `json:"letra"`
	Text   string `json:"texto"`
}

type Question struct {
	Number       int           `json:"numero"`
	Statement    string        `json:"enunciado"`
	Alternatives []Alternative `json:"alternativas"`
	SourcePage   int           `json:"-"`
	SourcePages  []int         `json:"-"`
}

type Page struct {
	Number int
	Text   string
}

var (
	questionHeader = regexp.MustCompile(`(?i)^\s*(?:QUEST[ÃA]O\s*)?(\d{1,3})[.)]?\s*[-–:]?\s*(.*)$`)
	alternative    = regexp.MustCompile(`(?m)^\s*(?:\(\s*([A-E])\s*\)|([A-E])\s*[\).])\s*[-.)]?\s*(.*)$`)
	answerEntry    = regexp.MustCompile(`(?i)(?:QUEST[ÃA]O\s*)?(\d{1,3})\s*[-.)：:]?\s*([A-E])\b`)
)

func ParsePages(pages []Page) ([]Question, error) {
	var out []Question
	var current *Question
	for _, page := range pages {
		if page.Number < 1 {
			return nil, fmt.Errorf("page number must be positive")
		}
		if strings.TrimSpace(page.Text) == "" {
			if current != nil && current.SourcePages[len(current.SourcePages)-1] != page.Number {
				current.SourcePages = append(current.SourcePages, page.Number)
			}
			continue
		}
		for _, line := range strings.Split(strings.ReplaceAll(page.Text, "\r\n", "\n"), "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				if current != nil {
					current.Statement = appendText(current.Statement, "")
				}
				continue
			}
			matches := questionHeader.FindStringSubmatch(trimmed)
			if matches != nil && looksLikeQuestionHeader(trimmed, matches[2]) {
				if current != nil {
					finalize(current)
					out = append(out, *current)
				}
				var number int
				if _, err := fmt.Sscanf(matches[1], "%d", &number); err != nil {
					return nil, fmt.Errorf("invalid question number: %w", err)
				}
				current = &Question{Number: number, Alternatives: []Alternative{}, SourcePage: page.Number, SourcePages: []int{page.Number}}
				if text := strings.TrimSpace(matches[2]); text != "" {
					current.Statement = text
				}
				continue
			}
			if current == nil {
				continue
			}
			if current.SourcePages[len(current.SourcePages)-1] != page.Number {
				current.SourcePages = append(current.SourcePages, page.Number)
			}
			if alt := alternative.FindStringSubmatch(trimmed); alt != nil {
				letter := alt[1]
				if letter == "" {
					letter = alt[2]
				}
				current.Alternatives = append(current.Alternatives, Alternative{
					Letter: strings.ToUpper(letter),
					Text:   strings.TrimSpace(alt[3]),
				})
				continue
			}
			if n := len(current.Alternatives); n > 0 {
				current.Alternatives[n-1].Text = appendText(current.Alternatives[n-1].Text, trimmed)
			} else {
				current.Statement = appendText(current.Statement, trimmed)
			}
		}
	}
	if current != nil {
		finalize(current)
		out = append(out, *current)
	}
	for i := range out {
		if out[i].Number < 1 || out[i].Statement == "" {
			return nil, fmt.Errorf("question %d has no statement", out[i].Number)
		}
		if err := validateAlternatives(out[i].Alternatives); err != nil {
			return nil, fmt.Errorf("question %d: %w", out[i].Number, err)
		}
	}
	return out, nil
}

func ParseAnswerKey(text string) map[int]string {
	answers := make(map[int]string)
	for _, match := range answerEntry.FindAllStringSubmatch(text, -1) {
		var number int
		if _, err := fmt.Sscanf(match[1], "%d", &number); err == nil && number > 0 {
			answers[number] = strings.ToUpper(match[2])
		}
	}
	return answers
}

func looksLikeQuestionHeader(line, remainder string) bool {
	if strings.Contains(strings.ToUpper(line), "QUEST") {
		return true
	}
	if strings.TrimSpace(remainder) == "" {
		return false
	}
	runes := []rune(strings.TrimSpace(remainder))
	return len(runes) > 0 && (unicode.IsUpper(runes[0]) || unicode.IsDigit(runes[0]) || strings.HasPrefix(strings.TrimSpace(remainder), "Uma "))
}

func appendText(existing, next string) string {
	if next == "" {
		if existing == "" || strings.HasSuffix(existing, "\n\n") {
			return existing
		}
		return existing + "\n\n"
	}
	if existing == "" {
		return next
	}
	if strings.HasSuffix(existing, "\n\n") {
		return existing + next
	}
	return existing + " " + next
}

func finalize(question *Question) {
	question.Statement = strings.TrimSpace(question.Statement)
	for i := range question.Alternatives {
		question.Alternatives[i].Text = strings.TrimSpace(question.Alternatives[i].Text)
	}
}

func validateAlternatives(alternatives []Alternative) error {
	if len(alternatives) != 5 {
		return fmt.Errorf("found %d alternatives, want 5", len(alternatives))
	}
	for i, want := range []string{"A", "B", "C", "D", "E"} {
		if alternatives[i].Letter != want || alternatives[i].Text == "" {
			return fmt.Errorf("alternatives must be non-empty A-E in order")
		}
	}
	return nil
}
