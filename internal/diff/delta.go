package diff

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// DeltaRenderer shells out to the delta binary for diff rendering.
type DeltaRenderer struct {
	path  string
	width int
}

// NewDeltaRenderer creates a DeltaRenderer that invokes the delta binary at
// path with the given output width.
func NewDeltaRenderer(path string, width int) *DeltaRenderer {
	return &DeltaRenderer{path: path, width: width}
}

// Render passes rawDiff to the delta binary via stdin and returns the
// formatted output. The --width flag is set to prevent delta from using
// the full terminal width.
func (d *DeltaRenderer) Render(rawDiff string) (string, error) {
	if rawDiff == "" {
		return "", nil
	}

	cmd := exec.Command(d.path, "--width", strconv.Itoa(d.width))
	cmd.Stdin = strings.NewReader(rawDiff)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("delta: %w: %s", err, stderr.String())
	}

	return stdout.String(), nil
}
