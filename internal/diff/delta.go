package diff

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// DeltaRenderer shells out to the delta binary for diff rendering.
type DeltaRenderer struct {
	path string
}

// NewDeltaRenderer creates a DeltaRenderer that invokes the delta binary at
// the given path. No --width flag is passed so that the output is full-width
// and the viewport can handle horizontal scrolling.
func NewDeltaRenderer(path string) *DeltaRenderer {
	return &DeltaRenderer{path: path}
}

// Render passes rawDiff to the delta binary via stdin and returns the
// formatted output.
func (d *DeltaRenderer) Render(rawDiff string) (string, error) {
	if rawDiff == "" {
		return "", nil
	}

	cmd := exec.Command(d.path)
	cmd.Stdin = strings.NewReader(rawDiff)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("delta: %w: %s", err, stderr.String())
	}

	return stdout.String(), nil
}
