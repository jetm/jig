package commands

import (
	"strings"
	"testing"

	"github.com/jetm/gti/internal/git"
)

func TestRebaseItem_Title(t *testing.T) {
	entry := git.RebaseTodoEntry{
		Action:  git.ActionPick,
		Hash:    "abc1234",
		Subject: "feat: add feature",
	}
	item := rebaseItem{entry: entry}

	title := item.Title()
	if !strings.Contains(title, "abc1234") {
		t.Errorf("Title() = %q, want it to contain hash", title)
	}
	if !strings.Contains(title, "feat: add feature") {
		t.Errorf("Title() = %q, want it to contain subject", title)
	}
	if !strings.Contains(title, "pick") {
		t.Errorf("Title() = %q, want it to contain action", title)
	}
}

func TestRebaseItem_Description(t *testing.T) {
	entry := git.RebaseTodoEntry{
		Action:  git.ActionSquash,
		Hash:    "bbb5678",
		Subject: "fix: something",
	}
	item := rebaseItem{entry: entry}

	desc := item.Description()
	if !strings.Contains(desc, "squash") {
		t.Errorf("Description() = %q, want it to contain action", desc)
	}
	if !strings.Contains(desc, "bbb5678") {
		t.Errorf("Description() = %q, want it to contain hash", desc)
	}
}

func TestRebaseItem_FilterValue(t *testing.T) {
	entry := git.RebaseTodoEntry{
		Action:  git.ActionDrop,
		Hash:    "ccc9012",
		Subject: "chore: cleanup",
	}
	item := rebaseItem{entry: entry}

	filter := item.FilterValue()
	if !strings.Contains(filter, "ccc9012") {
		t.Errorf("FilterValue() = %q, want it to contain hash", filter)
	}
	if !strings.Contains(filter, "chore: cleanup") {
		t.Errorf("FilterValue() = %q, want it to contain subject", filter)
	}
}

func TestRebaseItem_AllActions(t *testing.T) {
	actions := []git.RebaseAction{
		git.ActionPick,
		git.ActionReword,
		git.ActionEdit,
		git.ActionSquash,
		git.ActionFixup,
		git.ActionDrop,
	}
	for _, a := range actions {
		t.Run(string(a), func(t *testing.T) {
			item := rebaseItem{entry: git.RebaseTodoEntry{Action: a, Hash: "abc1234", Subject: "test"}}
			title := item.Title()
			if !strings.Contains(title, string(a)) {
				t.Errorf("Title() = %q, want it to contain action %q", title, a)
			}
		})
	}
}

func TestActionIcon_AllActions(t *testing.T) {
	actions := []git.RebaseAction{
		git.ActionPick,
		git.ActionReword,
		git.ActionEdit,
		git.ActionSquash,
		git.ActionFixup,
		git.ActionDrop,
	}
	for _, a := range actions {
		t.Run(string(a), func(t *testing.T) {
			icon := actionIcon(a)
			if icon == "" {
				t.Errorf("actionIcon(%q) returned empty string", a)
			}
		})
	}
}

func TestActionIcon_Unknown(t *testing.T) {
	icon := actionIcon("unknown")
	if icon == "" {
		t.Error("actionIcon(unknown) returned empty string")
	}
}
