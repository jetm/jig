package diff

import (
	"errors"
	"testing"

	"github.com/jetm/gti/internal/config"
)

func TestChain_PlainFallback(t *testing.T) {
	orig := newChromaFunc
	newChromaFunc = func() (*ChromaRenderer, error) {
		return nil, errors.New("simulated failure")
	}
	defer func() { newChromaFunc = orig }()

	cfg := config.Config{DeltaPath: ""}
	r := Chain(cfg)

	if _, ok := r.(*PlainRenderer); !ok {
		t.Errorf("Chain with failed chroma: got %T, want *PlainRenderer", r)
	}
}
