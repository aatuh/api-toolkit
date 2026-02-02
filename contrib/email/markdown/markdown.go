package markdown

import (
	"bytes"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

var renderer = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
)

// Render converts Markdown input into HTML.
func Render(input string) (string, error) {
	var buf bytes.Buffer
	if err := renderer.Convert([]byte(input), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}
