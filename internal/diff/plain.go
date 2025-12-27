package diff

// PlainRenderer returns the raw diff input unchanged.
// It serves as a zero-cost identity transform fallback.
type PlainRenderer struct{}

// Render returns rawDiff unmodified. It never returns an error.
func (p *PlainRenderer) Render(rawDiff string) (string, error) {
	return rawDiff, nil
}
