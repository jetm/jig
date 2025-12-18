package git

import (
	"context"
	"os"
	"strings"
)

// ResolveEditor returns the user's preferred editor by checking:
// $GIT_EDITOR → git config core.editor → $VISUAL → $EDITOR → "vi"
func ResolveEditor(ctx context.Context, r Runner) string {
	if v := os.Getenv("GIT_EDITOR"); v != "" {
		return v
	}
	if out, err := r.Run(ctx, "config", "core.editor"); err == nil {
		if v := strings.TrimSpace(out); v != "" {
			return v
		}
	}
	if v := os.Getenv("VISUAL"); v != "" {
		return v
	}
	if v := os.Getenv("EDITOR"); v != "" {
		return v
	}
	return "vi"
}
