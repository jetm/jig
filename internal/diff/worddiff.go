package diff

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Span marks a visible-character range [Start, End) in a string.
type Span struct {
	Start, End int
}

// WordDiff computes which visible-character spans differ between old and new.
// It tokenizes both on word/punctuation boundaries, runs LCS on the tokens,
// and returns the character-position spans of tokens that were removed (in old)
// and added (in new). Both strings should be stripped of their leading +/-
// diff prefix before calling.
func WordDiff(old, new string) (oldSpans, newSpans []Span) {
	oldTokens := tokenize(old)
	newTokens := tokenize(new)

	m, n := len(oldTokens), len(newTokens)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if oldTokens[i-1].text == newTokens[j-1].text {
				dp[i][j] = dp[i-1][j-1] + 1
			} else {
				dp[i][j] = max(dp[i-1][j], dp[i][j-1])
			}
		}
	}

	oldChanged := make([]bool, m)
	newChanged := make([]bool, n)
	i, j := m, n
	for i > 0 && j > 0 {
		switch {
		case oldTokens[i-1].text == newTokens[j-1].text:
			i--
			j--
		case dp[i-1][j] >= dp[i][j-1]:
			oldChanged[i-1] = true
			i--
		default:
			newChanged[j-1] = true
			j--
		}
	}
	for ; i > 0; i-- {
		oldChanged[i-1] = true
	}
	for ; j > 0; j-- {
		newChanged[j-1] = true
	}

	return collectSpans(oldTokens, oldChanged), collectSpans(newTokens, newChanged)
}

// token holds a word with its rune-offset in the source string.
type token struct {
	text  string
	start int
	end   int
}

// tokenize splits s into word and non-word tokens, recording their rune offsets.
func tokenize(s string) []token {
	runes := []rune(s)
	var tokens []token
	i := 0
	for i < len(runes) {
		r := runes[i]
		end := i + 1
		if unicode.IsSpace(r) {
			for end < len(runes) && unicode.IsSpace(runes[end]) {
				end++
			}
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			for end < len(runes) && (unicode.IsLetter(runes[end]) || unicode.IsDigit(runes[end]) || runes[end] == '_') {
				end++
			}
		}
		tokens = append(tokens, token{text: string(runes[i:end]), start: i, end: end})
		i = end
	}
	return tokens
}

// collectSpans merges consecutive changed tokens into contiguous Spans.
func collectSpans(tokens []token, changed []bool) []Span {
	var spans []Span
	inSpan := false
	var cur Span
	for i, t := range tokens {
		if changed[i] {
			if !inSpan {
				cur = Span{Start: t.start}
				inSpan = true
			}
			cur.End = t.end
		} else if inSpan {
			spans = append(spans, cur)
			inSpan = false
		}
	}
	if inSpan {
		spans = append(spans, cur)
	}
	return spans
}

// ApplyHighlightSpans overlays a background colour on the given visible-rune
// spans of an already-ANSI-coloured string. bg is the ANSI SGR open sequence
// (e.g. "\x1b[48;2;80;30;30m"); the reset injected at the end of each span is
// always \x1b[49m (background reset only, preserving foreground colours).
func ApplyHighlightSpans(colored string, spans []Span, bg string) string {
	if len(spans) == 0 || bg == "" {
		return colored
	}

	type insertion struct {
		pos  int
		code string
	}
	inserts := make([]insertion, 0, len(spans)*2)
	for _, sp := range spans {
		inserts = append(inserts, insertion{sp.Start, bg}, insertion{sp.End, "\x1b[49m"})
	}

	var out strings.Builder
	out.Grow(len(colored) + len(inserts)*20)
	visPos := 0
	nextIns := 0
	bs := []byte(colored)
	i := 0

	for i < len(bs) {
		for nextIns < len(inserts) && inserts[nextIns].pos == visPos {
			out.WriteString(inserts[nextIns].code)
			nextIns++
		}

		// ANSI CSI escape: pass through without advancing visPos.
		if bs[i] == '\x1b' && i+1 < len(bs) && bs[i+1] == '[' {
			j := i + 2
			for j < len(bs) && (bs[j] < 0x40 || bs[j] > 0x7E) {
				j++
			}
			if j < len(bs) {
				j++
			}
			out.Write(bs[i:j])
			i = j
			continue
		}

		r, size := utf8.DecodeRune(bs[i:])
		out.WriteRune(r)
		i += size
		visPos++
	}
	for nextIns < len(inserts) {
		out.WriteString(inserts[nextIns].code)
		nextIns++
	}
	return out.String()
}

// StripANSI removes CSI escape sequences from s, returning plain text.
func StripANSI(s string) string {
	var out strings.Builder
	bs := []byte(s)
	i := 0
	for i < len(bs) {
		if bs[i] == '\x1b' && i+1 < len(bs) && bs[i+1] == '[' {
			j := i + 2
			for j < len(bs) && (bs[j] < 0x40 || bs[j] > 0x7E) {
				j++
			}
			if j < len(bs) {
				j++
			}
			i = j
			continue
		}
		r, size := utf8.DecodeRune(bs[i:])
		out.WriteRune(r)
		i += size
	}
	return out.String()
}
