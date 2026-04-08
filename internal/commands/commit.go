package commands

// CommitDoneMsg is sent after a commit subprocess (cfg.CommitCmd) exits.
// Both AddModel and HunkAddModel handle this message to refresh state.
type CommitDoneMsg struct {
	Err error
}
