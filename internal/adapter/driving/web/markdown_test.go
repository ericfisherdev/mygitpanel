package web

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRenderMarkdown_EmptyInput(t *testing.T) {
	assert.Equal(t, "", RenderMarkdown(""))
}

func TestRenderMarkdown_PlainText(t *testing.T) {
	result := RenderMarkdown("hello world")
	assert.Contains(t, result, "hello world")
}

func TestRenderMarkdown_Bold(t *testing.T) {
	result := RenderMarkdown("**bold text**")
	assert.Contains(t, result, "<strong>bold text</strong>")
}

func TestRenderMarkdown_InlineCode(t *testing.T) {
	result := RenderMarkdown("use `fmt.Println`")
	assert.Contains(t, result, "<code>fmt.Println</code>")
}

func TestRenderMarkdown_CodeBlock(t *testing.T) {
	input := "```go\nfmt.Println(\"hello\")\n```"
	result := RenderMarkdown(input)
	assert.Contains(t, result, "<code")
	assert.Contains(t, result, "fmt.Println")
}

func TestRenderMarkdown_Link(t *testing.T) {
	result := RenderMarkdown("[click](https://example.com)")
	assert.Contains(t, result, `<a href="https://example.com"`)
	assert.Contains(t, result, "click</a>")
}

func TestRenderMarkdown_SanitizesScript(t *testing.T) {
	result := RenderMarkdown(`<script>alert("xss")</script>`)
	assert.NotContains(t, result, "<script>")
}

func TestRenderMarkdown_GFMStrikethrough(t *testing.T) {
	result := RenderMarkdown("~~deleted~~")
	assert.Contains(t, result, "<del>deleted</del>")
}

func TestRenderMarkdown_GFMTaskList(t *testing.T) {
	result := RenderMarkdown("- [x] done\n- [ ] todo")
	assert.Contains(t, result, "<li>")
	assert.Contains(t, result, "done")
	assert.Contains(t, result, "todo")
}

func TestRenderDiffHunk_EmptyInput(t *testing.T) {
	assert.Equal(t, "", RenderDiffHunk(""))
}

func TestRenderDiffHunk_LineClasses(t *testing.T) {
	hunk := "@@ -1,3 +1,4 @@\n context line\n+added line\n-removed line"
	result := RenderDiffHunk(hunk)

	assert.Contains(t, result, `class="diff-header"`)
	assert.Contains(t, result, `class="diff-ctx"`)
	assert.Contains(t, result, `class="diff-add"`)
	assert.Contains(t, result, `class="diff-del"`)
}

func TestRenderDiffHunk_EscapesHTML(t *testing.T) {
	hunk := "+<script>alert('xss')</script>"
	result := RenderDiffHunk(hunk)

	assert.NotContains(t, result, "<script>")
	assert.Contains(t, result, `class="diff-add"`)
}

func TestRenderDiffHunk_PreservesNewlines(t *testing.T) {
	hunk := "@@ header\n+add\n-del"
	result := RenderDiffHunk(hunk)

	spans := strings.Count(result, "<span")
	assert.Equal(t, 3, spans)
}
