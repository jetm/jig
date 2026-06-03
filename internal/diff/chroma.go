package diff

import (
	"bytes"
	"errors"
	"strings"

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

// bgRemoved is the truecolor background injected onto changed words in removal lines.
const bgRemoved = "\x1b[48;2;80;30;30m"

// bgAdded is the truecolor background injected onto changed words in addition lines.
const bgAdded = "\x1b[48;2;20;60;20m"

// Render tokenizes the raw diff with chroma's Diff lexer and formats it with
// ANSI escape codes. Adjacent -/+ line pairs receive intra-line word-level
// background highlights for the changed words. On any lexer or formatter error
// it returns the raw input as graceful degradation.
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

	return applyWordDiffHighlights(buf.String()), nil
}

// applyWordDiffHighlights post-processes chroma-coloured diff output, adding
// per-word background highlights to adjacent -/+ line pairs.
func applyWordDiffHighlights(colored string) string {
	lines := strings.Split(colored, "\n")
	out := make([]string, 0, len(lines))

	i := 0
	for i < len(lines) {
		raw := StripANSI(lines[i])
		if len(raw) > 0 && raw[0] == '-' && i+1 < len(lines) {
			nextRaw := StripANSI(lines[i+1])
			if len(nextRaw) > 0 && nextRaw[0] == '+' {
				// Pair found: compute word diff on content without the prefix.
				oldContent := raw[1:]
				newContent := nextRaw[1:]
				oldSpans, newSpans := WordDiff(oldContent, newContent)

				// Shift spans by 1 to account for the +/- prefix character.
				for k := range oldSpans {
					oldSpans[k].Start++
					oldSpans[k].End++
				}
				for k := range newSpans {
					newSpans[k].Start++
					newSpans[k].End++
				}

				out = append(out, ApplyHighlightSpans(lines[i], oldSpans, bgRemoved))
				out = append(out, ApplyHighlightSpans(lines[i+1], newSpans, bgAdded))
				i += 2
				continue
			}
		}
		out = append(out, lines[i])
		i++
	}
	return strings.Join(out, "\n")
}
