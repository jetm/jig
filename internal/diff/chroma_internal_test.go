package diff

import (
	"errors"
	"io"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/styles"
)

// failingLexer always returns an error from Tokenise.
type failingLexer struct{}

func (f *failingLexer) Config() *chroma.Config { return &chroma.Config{Name: "failing"} }
func (f *failingLexer) Tokenise(_ *chroma.TokeniseOptions, _ string) (chroma.Iterator, error) {
	return nil, errors.New("tokenise failed")
}
func (f *failingLexer) AnalyseText(_ string) float32                     { return 0 }
func (f *failingLexer) SetRegistry(_ *chroma.LexerRegistry) chroma.Lexer { return f }
func (f *failingLexer) SetAnalyser(_ func(string) float32) chroma.Lexer  { return f }

// failingFormatter always returns an error from Format.
type failingFormatter struct{}

func (f *failingFormatter) Format(_ io.Writer, _ *chroma.Style, _ chroma.Iterator) error {
	return errors.New("format failed")
}

func TestChromaRenderer_TokeniseError(t *testing.T) {
	r := &ChromaRenderer{
		lexer:     &failingLexer{},
		formatter: &failingFormatter{},
		style:     styles.Fallback,
	}

	input := "some diff content"
	got, err := r.Render(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != input {
		t.Errorf("expected raw input on tokenise error, got %q", got)
	}
}

func TestChromaRenderer_FormatError(t *testing.T) {
	// Use real lexer but failing formatter.
	cr, err := NewChromaRenderer()
	if err != nil {
		t.Fatalf("failed to create ChromaRenderer: %v", err)
	}

	r := &ChromaRenderer{
		lexer:     cr.lexer,
		formatter: &failingFormatter{},
		style:     cr.style,
	}

	input := "--- a/f\n+++ b/f\n@@ -1 +1 @@\n-old\n+new\n"
	got, err := r.Render(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != input {
		t.Errorf("expected raw input on format error, got %q", got)
	}
}
