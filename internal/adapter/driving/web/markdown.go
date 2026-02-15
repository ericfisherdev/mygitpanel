package web

import (
	"bytes"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

var (
	mdRenderer    goldmark.Markdown
	htmlSanitizer *bluemonday.Policy
)

func init() {
	mdRenderer = goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithRendererOptions(html.WithUnsafe()),
	)

	htmlSanitizer = bluemonday.UGCPolicy()
}

// RenderMarkdown converts a markdown string to sanitized HTML.
// Returns empty string for empty input.
func RenderMarkdown(src string) string {
	if src == "" {
		return ""
	}

	var buf bytes.Buffer
	if err := mdRenderer.Convert([]byte(src), &buf); err != nil {
		return htmlSanitizer.Sanitize(src)
	}

	return htmlSanitizer.Sanitize(buf.String())
}

// RenderDiffHunk converts a unified diff hunk into HTML with line-level CSS classes.
// Each line is wrapped in a <span> with a class indicating its diff role:
//   - diff-add: added lines (prefix "+")
//   - diff-del: deleted lines (prefix "-")
//   - diff-header: hunk headers (prefix "@@")
//   - diff-ctx: context lines (no special prefix)
func RenderDiffHunk(hunk string) string {
	if hunk == "" {
		return ""
	}

	lines := strings.Split(hunk, "\n")
	var buf strings.Builder
	buf.Grow(len(hunk) * 2)

	for i, line := range lines {
		if i > 0 {
			buf.WriteByte('\n')
		}

		cssClass := classForDiffLine(line)
		escaped := htmlSanitizer.Sanitize(line)

		buf.WriteString(`<span class="`)
		buf.WriteString(cssClass)
		buf.WriteString(`">`)
		buf.WriteString(escaped)
		buf.WriteString(`</span>`)
	}

	return buf.String()
}

func classForDiffLine(line string) string {
	if strings.HasPrefix(line, "@@") {
		return "diff-header"
	}
	if strings.HasPrefix(line, "+") {
		return "diff-add"
	}
	if strings.HasPrefix(line, "-") {
		return "diff-del"
	}
	return "diff-ctx"
}
