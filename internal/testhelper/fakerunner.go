package testhelper

import (
	"context"
	"slices"
	"strings"
	"sync"
	"testing"
)

// Call records a single invocation of a Runner method.
type Call struct {
	Args  []string
	Env   []string // populated only for RunWithEnv
	Stdin string   // populated only for RunWithStdin
}

// FakeRunner records all calls and returns scripted responses in FIFO order.
// Safe for concurrent use from parallel subtests.
type FakeRunner struct {
	mu      sync.Mutex
	Calls   []Call
	Outputs []string // FIFO: first call gets Outputs[0]
	Errors  []error  // FIFO: parallel to Outputs
}

func (f *FakeRunner) pop() (string, error) {
	if len(f.Outputs) == 0 {
		return "", nil
	}
	out := f.Outputs[0]
	f.Outputs = f.Outputs[1:]
	var err error
	if len(f.Errors) > 0 {
		err = f.Errors[0]
		f.Errors = f.Errors[1:]
	}
	return out, err
}

// Run records Call{Args: args} and returns the next scripted output.
func (f *FakeRunner) Run(_ context.Context, args ...string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, Call{Args: args})
	return f.pop()
}

// RunWithEnv records Call{Args: args, Env: env} and returns the next scripted output.
func (f *FakeRunner) RunWithEnv(_ context.Context, env []string, args ...string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, Call{Args: args, Env: env})
	return f.pop()
}

// RunWithStdin records Call{Args: args, Stdin: stdin} and returns the next scripted output.
func (f *FakeRunner) RunWithStdin(_ context.Context, stdin string, args ...string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, Call{Args: args, Stdin: stdin})
	return f.pop()
}

// MustHaveCall fails if no recorded Call contains args as a contiguous subsequence of Call.Args.
func MustHaveCall(tb testing.TB, f *FakeRunner, args ...string) {
	tb.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.Calls {
		if containsSubsequence(c.Args, args) {
			return
		}
	}
	tb.Errorf("FakeRunner: no call with args %v recorded; calls: %v", args, f.Calls)
}

// MustHaveNoCall fails if any Call was recorded.
func MustHaveNoCall(tb testing.TB, f *FakeRunner) {
	tb.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.Calls) > 0 {
		tb.Errorf("FakeRunner: expected no calls but got %d: %v", len(f.Calls), f.Calls)
	}
}

// MustHaveEnv fails if no RunWithEnv call had keyvalue in Env.
func MustHaveEnv(tb testing.TB, f *FakeRunner, keyvalue string) {
	tb.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.Calls {
		if slices.Contains(c.Env, keyvalue) {
			return
		}
	}
	tb.Errorf("FakeRunner: no call with env %q recorded; calls: %v", keyvalue, f.Calls)
}

// MustHaveStdin fails if no RunWithStdin call contained substr in Stdin.
func MustHaveStdin(tb testing.TB, f *FakeRunner, substr string) {
	tb.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.Calls {
		if strings.Contains(c.Stdin, substr) {
			return
		}
	}
	tb.Errorf("FakeRunner: no call with stdin containing %q recorded; calls: %v", substr, f.Calls)
}

// CallCount returns the number of recorded calls.
func CallCount(f *FakeRunner) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.Calls)
}

// NthCall returns the nth recorded Call (0-indexed).
func NthCall(f *FakeRunner, n int) Call {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.Calls[n]
}

// containsSubsequence reports whether needle appears as a contiguous subsequence in haystack.
func containsSubsequence(haystack, needle []string) bool {
	if len(needle) == 0 {
		return true
	}
	if len(needle) > len(haystack) {
		return false
	}
	for i := 0; i <= len(haystack)-len(needle); i++ {
		match := true
		for j, v := range needle {
			if haystack[i+j] != v {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
