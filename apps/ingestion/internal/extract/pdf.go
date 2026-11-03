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
