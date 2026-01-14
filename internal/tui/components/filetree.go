package components

import (
	"path/filepath"
	"sort"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	tree "github.com/mariusor/bubbles-tree"

	"github.com/jetm/gti/internal/git"
	"github.com/jetm/gti/internal/tui"
)

// NodeChecked is a custom state bit for file selection (checkbox) in add/checkout/reset views.
const NodeChecked tree.NodeState = tree.NodeMaxState << 1

// FileEntry represents a file with its path and git status, used as input to BuildTree.
type FileEntry struct {
	Path   string
	Status git.FileStatus
}

// FileNode represents a file leaf in the tree.
type FileNode struct {
	name         string
	fullPath     string
	status       git.FileStatus
	parent       tree.Node
	state        tree.NodeState
	checked      bool
	showCheckbox bool
}

// Init implements tree.Node.
func (n *FileNode) Init() tea.Cmd { return nil }

// Parent implements tree.Node.
func (n *FileNode) Parent() tree.Node { return n.parent }

// Children implements tree.Node.
func (n *FileNode) Children() tree.Nodes { return nil }

// State implements tree.Node.
func (n *FileNode) State() tree.NodeState {
	s := n.state
	if n.checked {
		s |= NodeChecked
	}
	return s
}

// Update implements tree.Node.
func (n *FileNode) Update(msg tea.Msg) tea.Cmd {
	if st, ok := msg.(tree.NodeState); ok {
		n.state = st
	}
	return nil
}

// View implements tree.Node.
func (n *FileNode) View() string {
	var b strings.Builder
	if n.showCheckbox {
		if n.checked {
			b.WriteString(tui.IconChecked)
		} else {
			b.WriteString(tui.IconUnchecked)
		}
		b.WriteString(" ")
	}
	b.WriteString(statusIcon(n.status))
	b.WriteString(" ")
	b.WriteString(n.name)
	return b.String()
}

// FullPath returns the full relative path of this file.
func (n *FileNode) FullPath() string { return n.fullPath }

// IsChecked returns whether this file node is selected.
func (n *FileNode) IsChecked() bool { return n.checked }

// SetChecked sets the checked state.
func (n *FileNode) SetChecked(v bool) { n.checked = v }

// DirNode represents a directory in the tree.
type DirNode struct {
	name     string
	parent   tree.Node
	children tree.Nodes
	state    tree.NodeState
}

// Init implements tree.Node.
func (n *DirNode) Init() tea.Cmd { return nil }

// Parent implements tree.Node.
func (n *DirNode) Parent() tree.Node { return n.parent }

// Children implements tree.Node.
func (n *DirNode) Children() tree.Nodes { return n.children }

// State implements tree.Node.
func (n *DirNode) State() tree.NodeState {
	return n.state | tree.NodeCollapsible
}

// Update implements tree.Node.
func (n *DirNode) Update(msg tea.Msg) tea.Cmd {
	if st, ok := msg.(tree.NodeState); ok {
		n.state = st
	}
	return nil
}

// View implements tree.Node.
func (n *DirNode) View() string {
	dirStyle := lipgloss.NewStyle().Foreground(tui.ColorBlue)
	return dirStyle.Render(tui.IconFolder + " " + n.name)
}

// statusIcon returns icon-only for a git file status.
func statusIcon(s git.FileStatus) string {
	switch s {
	case git.Added:
		return tui.IconAdded
	case git.Deleted:
		return tui.IconDeleted
	case git.Renamed:
		return tui.IconRenamed
	default:
		return tui.IconModified
	}
}

// BuildTree converts a flat list of FileEntry items into a tree.Nodes hierarchy.
// It groups files by directory, sorts them, and collapses single-child directory chains.
// If showCheckbox is true, file nodes render with checkbox icons.
func BuildTree(entries []FileEntry, showCheckbox bool) tree.Nodes {
	if len(entries) == 0 {
		return nil
	}

	// Build an intermediate structure: map of dir path -> dirBuild
	type dirBuild struct {
		files    []FileEntry
		subdirs  map[string]bool
		dirOrder []string
	}

	dirs := make(map[string]*dirBuild)
	ensureDir := func(path string) *dirBuild {
		if d, ok := dirs[path]; ok {
			return d
		}
		d := &dirBuild{subdirs: make(map[string]bool)}
		dirs[path] = d
		return d
	}

	// Parse all entries into their directory components.
	for _, e := range entries {
		dir := filepath.Dir(e.Path)
		if dir == "." {
			dir = ""
		}
		d := ensureDir(dir)
		d.files = append(d.files, e)

		// Ensure all parent directories exist.
		for dir != "" {
			parent := filepath.Dir(dir)
			if parent == "." {
				parent = ""
			}
			pd := ensureDir(parent)
			base := filepath.Base(dir)
			if !pd.subdirs[base] {
				pd.subdirs[base] = true
				pd.dirOrder = append(pd.dirOrder, base)
			}
			dir = parent
		}
	}

	// Recursive builder that creates tree nodes from the directory map.
	var buildNodes func(dirPath string, parent tree.Node) tree.Nodes
	buildNodes = func(dirPath string, parent tree.Node) tree.Nodes {
		d, ok := dirs[dirPath]
		if !ok {
			return nil
		}

		var nodes tree.Nodes

		// Sort subdirectories.
		sortedDirs := make([]string, len(d.dirOrder))
		copy(sortedDirs, d.dirOrder)
		sort.Strings(sortedDirs)

		// Add directory nodes.
		for _, subName := range sortedDirs {
			subPath := subName
			if dirPath != "" {
				subPath = dirPath + "/" + subName
			}

			dirNode := &DirNode{
				name:   subName,
				parent: parent,
			}

			children := buildNodes(subPath, dirNode)
			dirNode.children = children

			// Collapse single-child directory chains.
			dirNode = collapseDirChain(dirNode)

			nodes = append(nodes, dirNode)
		}

		// Sort and add file nodes.
		sort.Slice(d.files, func(i, j int) bool {
			return filepath.Base(d.files[i].Path) < filepath.Base(d.files[j].Path)
		})

		for _, f := range d.files {
			fileNode := &FileNode{
				name:         filepath.Base(f.Path),
				fullPath:     f.Path,
				status:       f.Status,
				parent:       parent,
				showCheckbox: showCheckbox,
			}
			nodes = append(nodes, fileNode)
		}

		return nodes
	}

	return buildNodes("", nil)
}

// collapseDirChain merges single-child directory chains into combined path nodes.
// e.g., if dir "a" has only child dir "b" which has only child dir "c", collapse to "a/b/c".
func collapseDirChain(d *DirNode) *DirNode {
	for len(d.children) == 1 {
		child, ok := d.children[0].(*DirNode)
		if !ok {
			break
		}
		// Merge child into parent.
		d.name = d.name + "/" + child.name
		d.children = child.children
		// Re-parent children to point to the merged node.
		for _, c := range d.children {
			switch n := c.(type) {
			case *FileNode:
				n.parent = d
			case *DirNode:
				n.parent = d
			}
		}
	}
	return d
}

// FileTree wraps tree.Model with a file-tree-specific API.
type FileTree struct {
	tree    tree.Model
	entries []FileEntry
	// showCheckbox enables checkbox rendering for selection-based views.
	showCheckbox bool
}

// NewFileTree creates a FileTree from a list of file entries.
func NewFileTree(entries []FileEntry, showCheckbox bool) FileTree {
	nodes := BuildTree(entries, showCheckbox)
	m := tree.New(nodes)

	// Customize keymap to avoid conflicts with command bindings.
	km := m.KeyMap
	// Remove space from PageDown (conflicts with toggle selection).
	km.PageDown = key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("pgdn", "page down"))
	// Remove d from HalfPageDown (conflicts with deselect-all).
	km.HalfPageDown = key.NewBinding(key.WithKeys("ctrl+d"), key.WithHelp("ctrl+d", "½ page down"))
	// Remove f from PageDown.
	// Remove g from GotoTop (conflicts with possible bindings).
	km.GotoTop = key.NewBinding(key.WithKeys("home"), key.WithHelp("home", "go to start"))
	km.GotoBottom = key.NewBinding(key.WithKeys("end", "G"), key.WithHelp("G/end", "go to end"))
	m.KeyMap = km

	// Apply OneDark styling.
	m.Styles = tree.Styles{
		Line:     lipgloss.NewStyle(),
		Selected: lipgloss.NewStyle().Background(tui.ColorBgSel),
		Symbol:   tree.Style(lipgloss.NewStyle().Foreground(tui.ColorFgSubtle)),
	}

	return FileTree{
		tree:         m,
		entries:      entries,
		showCheckbox: showCheckbox,
	}
}

// Update forwards messages to the tree model.
func (ft *FileTree) Update(msg tea.Msg) tea.Cmd {
	_, cmd := ft.tree.Update(msg)
	return cmd
}

// View renders the tree as a string.
func (ft *FileTree) View() string {
	v := ft.tree.View()
	return v.Content
}

// SetWidth sets the tree viewport width.
func (ft *FileTree) SetWidth(w int) { ft.tree.SetWidth(w) }

// SetHeight sets the tree viewport height.
func (ft *FileTree) SetHeight(h int) { ft.tree.SetHeight(h) }

// SelectedPath returns the full path of the currently selected file node,
// or empty string if a directory is selected or no selection.
func (ft *FileTree) SelectedPath() string {
	node := ft.selectedNode()
	if fn, ok := node.(*FileNode); ok {
		return fn.fullPath
	}
	return ""
}

// SelectedNode returns the currently selected tree node, or nil.
func (ft *FileTree) SelectedNode() tree.Node {
	return ft.selectedNode()
}

func (ft *FileTree) selectedNode() tree.Node {
	nodes := ft.tree.Children()
	if nodes == nil {
		return nil
	}
	return nodeAt(nodes, ft.tree.Cursor())
}

// nodeAt traverses the tree to find the visible node at position i.
// This mirrors the unexported Nodes.at() in mariusor/bubbles-tree.
func nodeAt(nodes tree.Nodes, i int) tree.Node {
	j := 0
	for _, n := range nodes {
		if n.State().Is(tree.NodeHidden) {
			continue
		}
		if j == i {
			return n
		}
		if !n.State().Is(tree.NodeCollapsed) && n.Children() != nil {
			if nn := nodeAt(n.Children(), i-j-1); nn != nil {
				return nn
			}
			j += countVisible(n.Children())
		}
		j++
	}
	return nil
}

// countVisible counts all visible nodes (including expanded children) in the tree.
func countVisible(nodes tree.Nodes) int {
	count := 0
	for _, n := range nodes {
		if n.State().Is(tree.NodeHidden) {
			continue
		}
		count++
		if !n.State().Is(tree.NodeCollapsed) && n.Children() != nil {
			count += countVisible(n.Children())
		}
	}
	return count
}

// ToggleChecked toggles the checked state of the selected node.
// For FileNode: toggles the individual file.
// For DirNode: if all descendants are checked, unchecks all; otherwise checks all.
func (ft *FileTree) ToggleChecked() {
	node := ft.selectedNode()
	switch n := node.(type) {
	case *FileNode:
		n.checked = !n.checked
	case *DirNode:
		if !ft.showCheckbox {
			return
		}
		if allDescendantsChecked(n.children) {
			setCheckedRecursive(n.children, false)
		} else {
			setCheckedRecursive(n.children, true)
		}
	}
}

// allDescendantsChecked returns true if every FileNode under nodes is checked.
func allDescendantsChecked(nodes tree.Nodes) bool {
	for _, n := range nodes {
		switch node := n.(type) {
		case *FileNode:
			if !node.checked {
				return false
			}
		case *DirNode:
			if !allDescendantsChecked(node.children) {
				return false
			}
		}
	}
	return true
}

// SetAllChecked sets all file nodes to the given checked state.
func (ft *FileTree) SetAllChecked(checked bool) {
	setCheckedRecursive(ft.tree.Children(), checked)
}

func setCheckedRecursive(nodes tree.Nodes, checked bool) {
	for _, n := range nodes {
		switch node := n.(type) {
		case *FileNode:
			node.checked = checked
		case *DirNode:
			setCheckedRecursive(node.children, checked)
		}
	}
}

// CheckedPaths returns the full paths of all checked file nodes.
func (ft *FileTree) CheckedPaths() []string {
	var paths []string
	collectCheckedPaths(ft.tree.Children(), &paths)
	return paths
}

func collectCheckedPaths(nodes tree.Nodes, paths *[]string) {
	for _, n := range nodes {
		switch node := n.(type) {
		case *FileNode:
			if node.checked {
				*paths = append(*paths, node.fullPath)
			}
		case *DirNode:
			collectCheckedPaths(node.children, paths)
		}
	}
}

// SetCheckedByPath sets checked state for a specific file path.
func (ft *FileTree) SetCheckedByPath(path string, checked bool) {
	setCheckedByPathRecursive(ft.tree.Children(), path, checked)
}

func setCheckedByPathRecursive(nodes tree.Nodes, path string, checked bool) {
	for _, n := range nodes {
		switch node := n.(type) {
		case *FileNode:
			if node.fullPath == path {
				node.checked = checked
			}
		case *DirNode:
			setCheckedByPathRecursive(node.children, path, checked)
		}
	}
}

// Rebuild replaces the tree entries and reconstructs the tree.
func (ft *FileTree) Rebuild(entries []FileEntry) {
	ft.entries = entries
	nodes := BuildTree(entries, ft.showCheckbox)
	ft.tree = tree.New(nodes)

	// Re-apply customizations.
	km := ft.tree.KeyMap
	km.PageDown = key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("pgdn", "page down"))
	km.HalfPageDown = key.NewBinding(key.WithKeys("ctrl+d"), key.WithHelp("ctrl+d", "½ page down"))
	km.GotoTop = key.NewBinding(key.WithKeys("home"), key.WithHelp("home", "go to start"))
	km.GotoBottom = key.NewBinding(key.WithKeys("end", "G"), key.WithHelp("G/end", "go to end"))
	ft.tree.KeyMap = km

	ft.tree.Styles = tree.Styles{
		Line:     lipgloss.NewStyle(),
		Selected: lipgloss.NewStyle().Background(tui.ColorBgSel),
		Symbol:   tree.Style(lipgloss.NewStyle().Foreground(tui.ColorFgSubtle)),
	}
}
