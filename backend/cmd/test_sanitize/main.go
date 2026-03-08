package main

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	pongo2 "github.com/flosch/pongo2/v6"
)

var reTagBlock = regexp.MustCompile(`(?sU)({{.+}}|{%.+%}|{#.+#})`)

func sanitizeTemplate(content []byte) []byte {
	return reTagBlock.ReplaceAllFunc(content, func(match []byte) []byte {
		cleaned := bytes.ReplaceAll(match, []byte("\n"), []byte(" "))
		cleaned = bytes.ReplaceAll(cleaned, []byte("  "), []byte(" "))
		return cleaned
	})
}

type SanitizingLoader struct {
	basePath string
}

func NewSanitizingLoader(basePath string) (*SanitizingLoader, error) {
	absPath, err := filepath.Abs(basePath)
	if err != nil {
		return nil, err
	}
	return &SanitizingLoader{basePath: absPath}, nil
}

func (l *SanitizingLoader) Abs(base, name string) string {
	if filepath.IsAbs(name) {
		return name
	}
	return filepath.Join(l.basePath, name)
}

func (l *SanitizingLoader) Get(path string) (io.Reader, error) {
	absPath := path
	if !filepath.IsAbs(absPath) {
		absPath = filepath.Join(l.basePath, path)
	}
	absPath = filepath.Clean(absPath)
	if !strings.HasPrefix(absPath, l.basePath) {
		return nil, os.ErrNotExist
	}
	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}
	cleaned := sanitizeTemplate(content)
	return bytes.NewReader(cleaned), nil
}

func main() {
	templatesDir := "/Users/eric/Documents/Gridea Pro/themes/amore-jinja2/templates"

	loader, err := NewSanitizingLoader(templatesDir)
	if err != nil {
		slog.Error("FATAL", "error", err)
		os.Exit(1)
	}

	set := pongo2.NewSet("test", loader)
	set.Debug = true

	// List all .html files
	templates := []string{}
	filepath.Walk(templatesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(path, ".html") {
			rel, _ := filepath.Rel(templatesDir, path)
			templates = append(templates, rel)
		}
		return nil
	})

	successCount := 0
	failCount := 0

	for _, tmplName := range templates {
		_, err := set.FromFile(tmplName)
		if err != nil {
			fmt.Printf("❌ FAIL: %s\n   Error: %v\n\n", tmplName, err)
			failCount++
		} else {
			fmt.Printf("✅ OK:   %s\n", tmplName)
			successCount++
		}
	}

	fmt.Printf("\n=== 结果: %d 成功, %d 失败 ===\n", successCount, failCount)
}
