package markdown

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync/atomic"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/ast"
	htmlrenderer "github.com/gomarkdown/markdown/html"
	mdparser "github.com/gomarkdown/markdown/parser"
	"github.com/microcosm-cc/bluemonday"
)

var textRenderer = htmlrenderer.NewRenderer(htmlrenderer.RendererOptions{
	Flags: htmlFlags,
	RenderNodeHook: func(w io.Writer, node ast.Node, entering bool) (ast.WalkStatus, bool) {
		switch node := node.(type) {
		case *ast.HTMLSpan:
			htmlrenderer.EscapeHTML(w, node.Literal)
			return ast.GoToNext, true
		case *ast.HTMLBlock:
			_, _ = io.WriteString(w, "\n")
			htmlrenderer.EscapeHTML(w, node.Literal)
			_, _ = io.WriteString(w, "\n")
			return ast.GoToNext, true
		case *ast.Code:
			_, _ = io.WriteString(w, fmt.Sprintf("`%s`", node.Literal))
			return ast.GoToNext, true
		}
		return ast.GoToNext, false
	},
})

var strictPolicy = bluemonday.StrictPolicy()
var regexNewlines = regexp.MustCompile(`\n+`)

type plainTextCacheEntry struct {
	key   string
	value string
}

const plainTextCacheSize = 512
const plainTextCacheMaxInputLen = 10000

var plainTextCache [plainTextCacheSize]atomic.Pointer[plainTextCacheEntry]

func plainTextHash(s string) uint32 {
	h := uint32(2166136261)
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619
	}
	return h
}

func PlainText(input string) string {
	if input == "" {
		return ""
	}

	// NOTE: this might come back to bite me in the ass in
	// the future, but for now it's simple and works, just be
	// careful people aren't being malicious with cache MISS'
	useCache := len(input) <= plainTextCacheMaxInputLen
	var idx uint32
	if useCache {
		idx = plainTextHash(input) % plainTextCacheSize
		if entry := plainTextCache[idx].Load(); entry != nil && entry.key == input {
			return entry.value
		}
	}

	parser := mdparser.NewWithExtensions(mdExtns)
	output := markdown.ToHTML([]byte(input), parser, textRenderer)
	sanitizedOutput := strictPolicy.Sanitize(string(output))
	sanitizedOutput = regexNewlines.ReplaceAllString(sanitizedOutput, "\n")
	result := strings.TrimSpace(sanitizedOutput)

	if useCache {
		plainTextCache[idx].Store(&plainTextCacheEntry{key: input, value: result})
	}
	return result
}
