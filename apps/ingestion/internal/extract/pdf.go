package extract

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/digu-dev/Agente-Enem/apps/ingestion/internal/corpus"
	"github.com/digu-dev/Agente-Enem/apps/ingestion/internal/parse"
	"github.com/digu-dev/Agente-Enem/apps/ingestion/internal/records"
)

const (
	maxTextBytes  = 16 << 20
	maxImageBytes = 8 << 20
	maxToolTime   = 3 * time.Minute
)

var ErrNoQuestions = errors.New("no questions detected in native PDF text")

type Extractor interface {
	Extract(context.Context, string, corpus.Entry) ([]records.Question, error)
}

type LocalPDF struct {
	WorkDir string
}

func (l LocalPDF) Extract(ctx context.Context, path string, entry corpus.Entry) ([]records.Question, error) {
	output, err := runLimited(ctx, maxTextBytes, "pdftotext", "-layout", "-enc", "UTF-8", path, "-")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, errors.New("pdftotext is required; install Poppler")
		}
		return nil, fmt.Errorf("local PDF text extraction failed")
	}
	pages := splitPages(string(output))
	if looksTwoColumn(pages) {
		columns, err := readColumnPages(ctx, path, len(pages))
		if err != nil {
			return nil, err
		}
		pages = columns
	}
	parsed, err := parse.ParsePages(pages)
	if err != nil {
		return nil, fmt.Errorf("parse extracted PDF text: %w", err)
	}
	if len(parsed) == 0 {
		return nil, ErrNoQuestions
	}
	images := make(map[int]string)
	for _, question := range parsed {
		for _, page := range question.SourcePages {
			if _, found := images[page]; found {
				continue
			}
			imagePath, err := l.renderPage(ctx, path, entry, page)
			if err != nil {
				return nil, err
			}
			images[page] = imagePath
		}
	}
	result := make([]records.Question, 0, len(parsed))
	for _, question := range parsed {
		alternatives := make([]records.Alternative, len(question.Alternatives))
		for i, alternative := range question.Alternatives {
			alternatives[i] = records.Alternative{Letter: alternative.Letter, Text: alternative.Text}
		}
		id := QuestionID(entry, question.Number)
		imagesForQuestion, err := storeQuestionImages(l.WorkDir, id, question.SourcePages, images)
		if err != nil {
			return nil, err
		}
		record := records.Question{
			ID: id, File: entry.Name, Page: question.SourcePage,
			CorpusVersion: entry.CorpusVersion, Year: entry.Year,
			Booklet: entry.Booklet, Type: entry.Type, Source: entry.Source,
			Split: entry.Split, Number: question.Number,
			Statement: question.Statement, Alternatives: alternatives,
			Skill: nil, Competency: nil, Contents: []string{}, Images: imagesForQuestion,
		}
		result = append(result, record)
	}
	return result, nil
}

func storeQuestionImages(workDir, id string, pages []int, pageImages map[int]string) ([]string, error) {
	images := make([]string, 0, len(pages))
	for _, page := range pages {
		sourcePath := filepath.Join(workDir, filepath.FromSlash(pageImages[page]))
		info, err := os.Lstat(sourcePath)
		if err != nil {
			return nil, fmt.Errorf("stat source question image: %w", err)
		}
		if !info.Mode().IsRegular() || info.Size() <= 8 || info.Size() > maxImageBytes {
			return nil, errors.New("source question image has invalid type or size")
		}
		data, err := os.ReadFile(sourcePath)
		if err != nil {
			return nil, fmt.Errorf("read source question image: %w", err)
		}
		if len(data) <= 8 || len(data) > maxImageBytes || !bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")) {
			return nil, errors.New("source question image has invalid format or size")
		}
		name := fmt.Sprintf("%s-p%d.png", id, page)
		path := filepath.Join(workDir, "images", name)
		if err := writeAtomic(path, data, 0600); err != nil {
			return nil, fmt.Errorf("store question image: %w", err)
		}
		images = append(images, filepath.ToSlash(filepath.Join("images", name)))
	}
	return images, nil
}

func splitPages(text string) []parse.Page {
	rawPages := strings.Split(text, "\f")
	if len(rawPages) > 1 && strings.TrimSpace(rawPages[len(rawPages)-1]) == "" {
		rawPages = rawPages[:len(rawPages)-1]
	}
	pages := make([]parse.Page, 0, len(rawPages))
	for i, page := range rawPages {
		pages = append(pages, parse.Page{Number: i + 1, Text: page})
	}
	return pages
}

var (
	sideBySideHeaders = regexp.MustCompile(`(?i)QUEST[ÃA]O\s*\d{1,3}.*\sQUEST[ÃA]O\s*\d{1,3}`)
	bboxPage          = regexp.MustCompile(`<page width="([0-9.]+)" height="([0-9.]+)"`)
	bboxWord          = regexp.MustCompile(`<word xMin="([0-9.]+)" yMin="([0-9.]+)" xMax="([0-9.]+)"`)
	bboxHeader        = regexp.MustCompile(`<word xMin="([0-9.]+)"[^>]*>(?i:quest[ãa]o)</word>`)
)

type pageGeometry struct {
	width, height float64
	rightStart    float64
	words         [][2]float64
}

// looksTwoColumn detects layouts where -layout merges two question columns
// into the same text line.
func looksTwoColumn(pages []parse.Page) bool {
	for _, page := range pages {
		for _, line := range strings.Split(page.Text, "\n") {
			if sideBySideHeaders.MatchString(line) {
				return true
			}
		}
	}
	return false
}

// pageGeometries finds where the right column starts on each page from the
// x position of its "Questão" headers. Pages without one are single-column.
func pageGeometries(ctx context.Context, path string, pageCount int) ([]pageGeometry, error) {
	box, err := runLimited(ctx, maxTextBytes, "pdftotext", "-bbox", path, "-")
	if err != nil {
		return nil, errors.New("read PDF word positions failed")
	}
	geometries := make([]pageGeometry, 0, pageCount)
	for _, chunk := range strings.Split(string(box), "<page ")[1:] {
		match := bboxPage.FindStringSubmatch("<page " + chunk)
		if match == nil {
			return nil, errors.New("PDF page size not found")
		}
		width, errWidth := strconv.ParseFloat(match[1], 64)
		height, errHeight := strconv.ParseFloat(match[2], 64)
		if errWidth != nil || errHeight != nil {
			return nil, errors.New("invalid PDF page size")
		}
		geometry := pageGeometry{width: width, height: height}
		for _, header := range bboxHeader.FindAllStringSubmatch(chunk, -1) {
			x, err := strconv.ParseFloat(header[1], 64)
			if err == nil && x > width*0.45 && (geometry.rightStart == 0 || x < geometry.rightStart) {
				geometry.rightStart = x
			}
		}
		for _, word := range bboxWord.FindAllStringSubmatch(chunk, -1) {
			xMin, errMin := strconv.ParseFloat(word[1], 64)
			yMin, errY := strconv.ParseFloat(word[2], 64)
			xMax, errMax := strconv.ParseFloat(word[3], 64)
			// Watermarks, footers and wide figure labels are not column text.
			if errMin == nil && errMax == nil && errY == nil && yMin < height*0.9 && xMax-xMin < 150 {
				geometry.words = append(geometry.words, [2]float64{xMin, xMax})
			}
		}
		geometries = append(geometries, geometry)
	}
	inferRightStarts(geometries)
	if len(geometries) < pageCount {
		return nil, errors.New("PDF word positions are incomplete")
	}
	return geometries, nil
}

// inferRightStarts handles two-column pages whose right column starts with no
// "Questão" header (for example a continuation). It borrows the column start of
// the nearest page with the same parity and keeps it only when no word crosses
// the gutter, so single-column pages stay whole.
func inferRightStarts(geometries []pageGeometry) {
	known := map[int]float64{}
	for i, geometry := range geometries {
		if geometry.rightStart != 0 {
			if _, ok := known[i%2]; !ok {
				known[i%2] = geometry.rightStart
			}
		}
	}
	for i := range geometries {
		start, ok := known[i%2]
		if geometries[i].rightStart != 0 || !ok || len(geometries[i].words) == 0 {
			continue
		}
		crossing, right := false, false
		for _, word := range geometries[i].words {
			if word[0] < start-8 && word[1] > start-2 {
				crossing = true
				break
			}
			if word[0] >= start-3 {
				right = true
			}
		}
		if !crossing && right {
			geometries[i].rightStart = start
		}
	}
}

// readColumnPages reads each page as its left column followed by its right
// column, keeping the original page numbers.
func readColumnPages(ctx context.Context, path string, pageCount int) ([]parse.Page, error) {
	geometries, err := pageGeometries(ctx, path, pageCount)
	if err != nil {
		return nil, err
	}
	pages := make([]parse.Page, 0, pageCount)
	for number := 1; number <= pageCount; number++ {
		value := strconv.Itoa(number)
		geometry := geometries[number-1]
		columns := [][2]int{{0, int(geometry.width) + 1}}
		if geometry.rightStart != 0 {
			split := int(geometry.rightStart) - 3
			columns = [][2]int{{0, split}, {split, int(geometry.width) - split + 1}}
		}
		var text strings.Builder
		for _, column := range columns {
			column, err := runLimited(ctx, maxTextBytes, "pdftotext", "-layout", "-enc", "UTF-8", "-f", value, "-l", value,
				"-x", strconv.Itoa(column[0]), "-y", "0", "-W", strconv.Itoa(column[1]), "-H", strconv.Itoa(int(geometry.height)+1), path, "-")
			if err != nil {
				return nil, errors.New("local PDF column extraction failed")
			}
			text.Write(column)
			text.WriteByte('\n')
		}
		pages = append(pages, parse.Page{Number: number, Text: text.String()})
	}
	return pages, nil
}

func ReadText(ctx context.Context, path string) (string, error) {
	output, err := runLimited(ctx, maxTextBytes, "pdftotext", "-layout", "-enc", "UTF-8", path, "-")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", errors.New("pdftotext is required; install Poppler")
		}
		return "", errors.New("local PDF text extraction failed")
	}
	return string(output), nil
}

func (l LocalPDF) renderPage(ctx context.Context, inputPath string, entry corpus.Entry, page int) (string, error) {
	imageDir := filepath.Join(l.WorkDir, "images")
	if err := os.MkdirAll(imageDir, 0700); err != nil {
		return "", fmt.Errorf("create image output directory: %w", err)
	}
	hash := entry.SHA256
	if len(hash) > 10 {
		hash = hash[:10]
	}
	destName := fmt.Sprintf("%d-%s-%s-p%d.png", entry.Year, safeName(entry.Booklet), hash, page)
	dest := filepath.Join(imageDir, destName)
	if info, err := os.Lstat(dest); err == nil && info.Mode().IsRegular() && info.Size() > 8 && info.Size() <= maxImageBytes {
		header, readErr := os.ReadFile(dest)
		if readErr == nil && bytes.HasPrefix(header, []byte("\x89PNG\r\n\x1a\n")) {
			return filepath.ToSlash(filepath.Join("images", destName)), nil
		}
	}
	tmp, err := os.MkdirTemp("", "enem-render-*")
	if err != nil {
		return "", fmt.Errorf("create temporary render directory: %w", err)
	}
	defer os.RemoveAll(tmp)
	prefix := filepath.Join(tmp, "page")
	pageValue := strconv.Itoa(page)
	if _, err := runLimited(ctx, 1024, "pdftoppm", "-f", pageValue, "-l", pageValue, "-png", "-singlefile", "-scale-to", "1800", inputPath, prefix); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", errors.New("pdftoppm is required to rasterize question pages; install Poppler")
		}
		return "", fmt.Errorf("PDF page rasterization failed")
	}
	renderedPath := prefix + ".png"
	info, err := os.Lstat(renderedPath)
	if err != nil {
		return "", fmt.Errorf("stat rasterized page: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() <= 8 || info.Size() > maxImageBytes {
		return "", errors.New("rasterized page has invalid format or size")
	}
	rendered, err := os.ReadFile(renderedPath)
	if err != nil {
		return "", fmt.Errorf("read rasterized page: %w", err)
	}
	if len(rendered) <= 8 || len(rendered) > maxImageBytes || !bytes.HasPrefix(rendered, []byte("\x89PNG\r\n\x1a\n")) {
		return "", errors.New("rasterized page has invalid format or size")
	}
	if err := writeAtomic(dest, rendered, 0600); err != nil {
		return "", fmt.Errorf("write rasterized page: %w", err)
	}
	return filepath.ToSlash(filepath.Join("images", destName)), nil
}

func QuestionID(entry corpus.Entry, number int) string {
	hash := corpus.FileHash([]byte(entry.Name + "\x00" + entry.SHA256))
	return fmt.Sprintf("enem-%d-%s-q%d-%s", entry.Year, safeName(entry.Booklet), number, hash[:10])
}

func safeName(value string) string {
	var out strings.Builder
	for _, r := range strings.ToLower(value) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			out.WriteRune(r)
		} else if out.Len() > 0 && !strings.HasSuffix(out.String(), "-") {
			out.WriteByte('-')
		}
	}
	return strings.Trim(out.String(), "-")
}

type boundedBuffer struct {
	bytes.Buffer
	limit int
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.limit {
		return 0, errors.New("command output exceeds configured limit")
	}
	return b.Buffer.Write(p)
}

func runLimited(ctx context.Context, limit int, name string, args ...string) ([]byte, error) {
	commandContext, cancel := context.WithTimeout(ctx, maxToolTime)
	defer cancel()
	command := exec.CommandContext(commandContext, name, args...)
	stdout := &boundedBuffer{limit: limit}
	command.Stdout = stdout
	command.Stderr = io.Discard
	if err := command.Run(); err != nil {
		return nil, err
	}
	return stdout.Bytes(), nil
}

func writeAtomic(path string, data []byte, mode os.FileMode) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tmp := file.Name()
	defer os.Remove(tmp)
	if err := file.Chmod(mode); err != nil {
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
