package commands

// CommitDoneMsg is sent after a devtool commit subprocess exits.
// Both AddModel and HunkAddModel handle this message to refresh state.
type CommitDoneMsg struct {
	Err error
}
