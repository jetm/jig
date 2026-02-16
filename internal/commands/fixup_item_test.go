package commands

import (
	"strings"
	"testing"

	"github.com/jetm/jig/internal/git"
)

func TestFixupItem_Methods(t *testing.T) {
	entry := git.CommitEntry{
		Hash:    "abc1234",
		Subject: "feat: add fixup command",
		Author:  "Jane Doe",
		Date:    "2 hours ago",
	}
	item := fixupItem{entry: entry}

	title := item.Title()
	if !strings.Contains(title, "abc1234") {
		t.Errorf("Title() = %q, want it to contain hash", title)
	}
	if !strings.Contains(title, "feat: add fixup command") {
		t.Errorf("Title() = %q, want it to contain subject", title)
	}

	desc := item.Description()
	if !strings.Contains(desc, "Jane Doe") {
		t.Errorf("Description() = %q, want it to contain author", desc)
	}
	if !strings.Contains(desc, "2 hours ago") {
		t.Errorf("Description() = %q, want it to contain date", desc)
	}

	filter := item.FilterValue()
	if !strings.Contains(filter, "abc1234") {
		t.Errorf("FilterValue() = %q, want it to contain hash", filter)
	}
	if !strings.Contains(filter, "feat: add fixup command") {
		t.Errorf("FilterValue() = %q, want it to contain subject", filter)
	}
}
