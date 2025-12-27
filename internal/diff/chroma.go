package diff

import (
	"bytes"
	"errors"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// ChromaRenderer uses alecthomas/chroma/v2 to syntax-highlight unified diffs.
type ChromaRenderer struct {
	lexer     chroma.Lexer
	formatter chroma.Formatter
	style     *chroma.Style
}

// NewChromaRenderer creates a ChromaRenderer with chroma's Diff lexer and
// truecolor terminal formatter.
func NewChromaRenderer() (*ChromaRenderer, error) {
	lexer := lexers.Get("Diff")
	if lexer == nil {
		return nil, errors.New("diff lexer not found")
	}

	formatter := formatters.Get("terminal16m")
	if formatter == nil {
		formatter = formatters.Fallback
	}

	style := styles.Get("onedark")
	if style == nil {
		style = styles.Fallback
	}

	return &ChromaRenderer{
		lexer:     lexer,
		formatter: formatter,
		style:     style,
	}, nil
}

// Render tokenizes the raw diff with chroma's Diff lexer and formats it with
// ANSI escape codes. On any lexer or formatter error it returns the raw input
// as graceful degradation.
func (c *ChromaRenderer) Render(rawDiff string) (string, error) {
	if rawDiff == "" {
		return "", nil
	}

	iterator, err := c.lexer.Tokenise(nil, rawDiff)
	if err != nil {
		return rawDiff, nil
	}

	var buf bytes.Buffer
	if err := c.formatter.Format(&buf, c.style, iterator); err != nil {
		return rawDiff, nil
	}

	return buf.String(), nil
}
