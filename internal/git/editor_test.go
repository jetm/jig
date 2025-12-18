package git_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jetm/gti/internal/git"
	"github.com/jetm/gti/internal/testhelper"
)

func TestResolveEditor_GIT_EDITOR(t *testing.T) {
	t.Setenv("GIT_EDITOR", "nvim")
	fr := &testhelper.FakeRunner{}
	got := git.ResolveEditor(context.Background(), fr)
	if got != "nvim" {
		t.Errorf("got %q, want %q", got, "nvim")
	}
}

func TestResolveEditor_CoreEditor(t *testing.T) {
	t.Setenv("GIT_EDITOR", "")
	fr := &testhelper.FakeRunner{
		Outputs: []string{"code --wait"},
		Errors:  []error{nil},
	}
	got := git.ResolveEditor(context.Background(), fr)
	if got != "code --wait" {
		t.Errorf("got %q, want %q", got, "code --wait")
	}
}

func TestResolveEditor_VISUAL(t *testing.T) {
	t.Setenv("GIT_EDITOR", "")
	t.Setenv("VISUAL", "vim")
	fr := &testhelper.FakeRunner{
		Outputs: []string{""},
		Errors:  []error{errors.New("no config")},
	}
	got := git.ResolveEditor(context.Background(), fr)
	if got != "vim" {
		t.Errorf("got %q, want %q", got, "vim")
	}
}

func TestResolveEditor_EDITOR(t *testing.T) {
	t.Setenv("GIT_EDITOR", "")
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "nano")
	fr := &testhelper.FakeRunner{
		Outputs: []string{""},
		Errors:  []error{errors.New("no config")},
	}
	got := git.ResolveEditor(context.Background(), fr)
	if got != "nano" {
		t.Errorf("got %q, want %q", got, "nano")
	}
}

func TestResolveEditor_Fallback(t *testing.T) {
	t.Setenv("GIT_EDITOR", "")
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	fr := &testhelper.FakeRunner{
		Outputs: []string{""},
		Errors:  []error{errors.New("no config")},
	}
	got := git.ResolveEditor(context.Background(), fr)
	if got != "vi" {
		t.Errorf("got %q, want %q", got, "vi")
	}
}
