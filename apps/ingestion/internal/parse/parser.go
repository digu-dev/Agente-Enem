package parse

import (
	"fmt"
	"regexp"
	"sort"
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
	questionHeader  = regexp.MustCompile(`(?i)^\s*(?:QUEST[ÃA]O\s*)?(\d{1,3})[.)]?\s*[-–:]?\s*(.*)$`)
	alternative     = regexp.MustCompile(`(?m)^\s*(?:\(\s*([A-E])\s*\)|([A-E])\s*[\).])\s*[-.)]?\s*(.*)$`)
	answerEntry     = regexp.MustCompile(`(?i)(?:QUEST[ÃA]O\s*)?(\d{1,3})\s*[-.)：:]?\s*([A-E])\b`)
	explicitHeader  = regexp.MustCompile(`(?i)^\s*QUEST[ÃA]O\s*\d{1,3}\s*$`)
	bareAlternative = regexp.MustCompile(`^\s*([A-E])(?:\s+(\S.*)|(\d.*))?$`)
	inlineNoise     = regexp.MustCompile(`\*\d{6}[A-Z]{2}\d+\*|Quest[õo]es de \d+ a \d+|(?:CIÊNCIAS|MATEMÁTICA)(?: DA NATUREZA| E SUAS)*\s*$|[A-ZÀ-Ú, ]+ TECNOLOGIAS(?:\s*\|\s*\d+º\s*DIA(?:\s*\|\s*CADERNO(?:\s*\d+)?(?:\s*\|\s*[A-Z]+)?)?)?|\S*\.ind[bd]\s+\d+|\d{2}/\d{2}/\d{4}\s+\d{2}:\d{2}:\d{2}|\S*_Dia_\S*`)
	coverPage       = regexp.MustCompile(`(?s)\bCapa\b.*\bCapa\b|\bCapa\b.*\d+º Ca`)
	trailingLetter  = regexp.MustCompile(`^(.*\S)\s{3,}([A-E])$`)
	leadingLetters  = regexp.MustCompile(`^([A-E](?:\s+[A-E]){2,})(?:\s+(\S.*))?$`)
	wideSpace       = regexp.MustCompile(`\s{2,}`)
	pageNoise       = regexp.MustCompile(`^(?:\S*(?:EM|NEM|ENEM)\d{4}\S*|\*?\d{6}[A-Z]{2}\d?\*?|\d{6}[A-Z]{2}\.indb\s+\d+|.*\|\s*\d+º\s*DIA\s*\|\s*CADERNO\s*\d+.*|.*\|\s*(?:AZUL|AMARELO|ROSA|BRANCO|CINZA|VERDE)\b.*|\d{2}/\d{2}/\d{4}\s+\d{2}:\d{2}:\d{2}|\d{1,3})$`)
)

func ParsePages(pages []Page) ([]Question, error) {
	var out []Question
	var current *Question
	strict := hasExplicitHeaders(pages)
	for pageIndex, page := range pages {
		if page.Number < 1 {
			return nil, fmt.Errorf("page number must be positive")
		}
		if strict && coverPage.MatchString(page.Text) {
			continue
		}
		if strings.TrimSpace(page.Text) == "" {
			if current != nil && current.SourcePages[len(current.SourcePages)-1] != page.Number {
				current.SourcePages = append(current.SourcePages, page.Number)
			}
			continue
		}
		lines := strings.Split(strings.ReplaceAll(page.Text, "\r\n", "\n"), "\n")
		for index, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strict {
				trimmed = strings.TrimSpace(inlineNoise.ReplaceAllString(trimmed, ""))
			}
			if strict && trimmed != "" && pageNoise.MatchString(trimmed) {
				continue
			}
			if trimmed == "" {
				if current != nil {
					current.Statement = appendText(current.Statement, "")
				}
				continue
			}
			matches := questionHeader.FindStringSubmatch(trimmed)
			if matches != nil && (!strict || explicitHeader.MatchString(trimmed)) && looksLikeQuestionHeader(trimmed, matches[2]) {
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
			if strict {
				if m := trailingLetter.FindStringSubmatch(trimmed); m != nil && len(current.Alternatives) == 0 && nextLinesAreLetters(followingLines(pages, pageIndex, lines[index+1:]), "BCDE") {
					current.Statement = appendText(current.Statement, m[1])
					current.Alternatives = append(current.Alternatives, Alternative{Letter: m[2]})
					continue
				}
				if m := leadingLetters.FindStringSubmatch(trimmed); m != nil && onlyEmptyAlternatives(current.Alternatives) {
					for _, letter := range strings.Fields(m[1]) {
						current.Alternatives = append(current.Alternatives, Alternative{Letter: letter})
					}
					if m[2] != "" {
						trimmed = m[2]
					} else {
						continue
					}
				} else if letters := imageLetters(trimmed); letters != nil && onlyEmptyAlternatives(current.Alternatives) {
					for _, letter := range letters {
						current.Alternatives = append(current.Alternatives, Alternative{Letter: letter})
					}
					sort.SliceStable(current.Alternatives, func(i, j int) bool { return current.Alternatives[i].Letter < current.Alternatives[j].Letter })
					continue
				}
				if alt := bareAlternative.FindStringSubmatch(trimmed); alt != nil && isNextBareAlternative(current.Alternatives, alt[1], followingLines(pages, pageIndex, lines[index+1:])) {
					current.Alternatives = append(current.Alternatives, Alternative{Letter: alt[1], Text: strings.TrimSpace(alt[2] + alt[3])})
					continue
				}
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

// nextLinesAreLetters reports whether the next non-empty lines are exactly the
// given single letters, in order.
func nextLinesAreLetters(lines []string, letters string) bool {
	want := []rune(letters)
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		if len(want) == 0 {
			return true
		}
		if t != string(want[0]) {
			return false
		}
		want = want[1:]
	}
	return len(want) == 0
}

var imageLetterPair = regexp.MustCompile(`^([A-E])\s{3,}([A-E])$`)

// imageLetters reads letters of image-only alternatives printed in two columns
// ("A    D") or alone ("C"), where the text carries no content.
func imageLetters(line string) []string {
	if m := imageLetterPair.FindStringSubmatch(line); m != nil {
		return []string{m[1], m[2]}
	}
	if len(line) == 1 && line[0] >= 'A' && line[0] <= 'E' {
		return []string{line}
	}
	return nil
}

func onlyEmptyAlternatives(alts []Alternative) bool {
	for _, alt := range alts {
		if alt.Text != "" {
			return false
		}
	}
	return true
}

// hasExplicitHeaders reports whether the document labels questions as
// "Questão N". Numbered instruction items are then not treated as questions.
func hasExplicitHeaders(pages []Page) bool {
	for _, page := range pages {
		for _, line := range strings.Split(page.Text, "\n") {
			if explicitHeader.MatchString(strings.TrimSpace(line)) {
				return true
			}
		}
	}
	return false
}

// followingLines returns the rest of the page plus the next page, because the
// alternatives of one question may continue on the following page.
func followingLines(pages []Page, pageIndex int, rest []string) []string {
	following := append([]string(nil), rest...)
	if pageIndex+1 < len(pages) {
		following = append(following, strings.Split(strings.ReplaceAll(pages[pageIndex+1].Text, "\r\n", "\n"), "\n")...)
	}
	return following
}

// isNextBareAlternative accepts "A texto" style alternatives, without a
// delimiter, only in order. A leading "A" needs B-E to follow before the next
// question and no later "A" line may come first, so statement sentences
// starting with "A" are not misread.
func isNextBareAlternative(existing []Alternative, letter string, following []string) bool {
	if len(existing) > 0 {
		return letter == string(rune('A'+len(existing)))
	}
	if letter != "A" {
		return false
	}
	next := byte('B')
	for _, line := range following {
		trimmed := strings.TrimSpace(line)
		if explicitHeader.MatchString(trimmed) {
			return false
		}
		alt := bareAlternative.FindStringSubmatch(trimmed)
		if alt != nil && alt[1] == "A" && next == 'B' {
			return false
		}
		if alt != nil && alt[1][0] == next {
			if next == 'E' {
				return true
			}
			next++
		}
	}
	return false
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
	next = wideSpace.ReplaceAllString(next, " ")
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
		if alternatives[i].Letter != want {
			return fmt.Errorf("alternatives must be A-E in order")
		}
	}
	return nil
}
