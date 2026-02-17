package utils

import (
	"bytes"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

var (
	// 定义两个全局实例，复用以提升性能
	mdSafe   goldmark.Markdown
	mdUnsafe goldmark.Markdown
)

func init() {
	// 1. 初始化安全实例
	mdSafe = goldmark.New(
		goldmark.WithExtensions(extension.GFM, extension.Typographer),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(html.WithHardWraps(), html.WithXHTML()),
	)

	// 2. 初始化非安全实例 (允许 Raw HTML)
	mdUnsafe = goldmark.New(
		goldmark.WithExtensions(extension.GFM, extension.Typographer),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithXHTML(),
			html.WithUnsafe(), // 关键差异
		),
	)
}

// ToHTML 将 Markdown 文本转换为 HTML
func ToHTML(markdown string) string {
	return convert(mdSafe, markdown)
}

// ToHTMLUnsafe 将 Markdown 文本转换为 HTML（允许原始 HTML）
// 警告: 此函数允许 Markdown 中的原始 HTML，可能存在 XSS 风险
func ToHTMLUnsafe(markdown string) string {
	return convert(mdUnsafe, markdown)
}

// 统一的内部转换逻辑
func convert(engine goldmark.Markdown, markdown string) string {
	if markdown == "" {
		return ""
	}
	var buf bytes.Buffer
	if err := engine.Convert([]byte(markdown), &buf); err != nil {
		// fallback: simple wrapper
		return "<p>" + markdown + "</p>"
	}
	return buf.String()
}
