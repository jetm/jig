package components

import (
	"testing"

	tree "github.com/mariusor/bubbles-tree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jetm/gti/internal/git"
	"github.com/jetm/gti/internal/tui"
)

func TestBuildTree_SameDirectoryGrouping(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "internal/commands/diff.go", Status: git.Modified},
		{Path: "internal/commands/add.go", Status: git.Added},
	}

	nodes := BuildTree(entries, false)

	require.Len(t, nodes, 1, "should have one top-level dir node")
	dirNode, ok := nodes[0].(*DirNode)
	require.True(t, ok, "top-level should be DirNode")
	// Should be collapsed to "internal/commands" since single-child chain.
	assert.Equal(t, "internal/commands", dirNode.name)
	assert.Len(t, dirNode.children, 2, "should have 2 file children")

	// Files sorted alphabetically.
	f0, ok := dirNode.children[0].(*FileNode)
	require.True(t, ok)
	assert.Equal(t, "add.go", f0.name)
	assert.Equal(t, "internal/commands/add.go", f0.fullPath)

	f1, ok := dirNode.children[1].(*FileNode)
	require.True(t, ok)
	assert.Equal(t, "diff.go", f1.name)
	assert.Equal(t, "internal/commands/diff.go", f1.fullPath)
}

func TestBuildTree_DifferentDirectorySeparation(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "internal/commands/diff.go", Status: git.Modified},
		{Path: "internal/tui/styles.go", Status: git.Modified},
	}

	nodes := BuildTree(entries, false)

	require.Len(t, nodes, 1, "should have one top-level internal dir")
	dirNode, ok := nodes[0].(*DirNode)
	require.True(t, ok)
	assert.Equal(t, "internal", dirNode.name)
	assert.Len(t, dirNode.children, 2, "should have 2 subdirs")

	sub0, ok := dirNode.children[0].(*DirNode)
	require.True(t, ok)
	assert.Equal(t, "commands", sub0.name)

	sub1, ok := dirNode.children[1].(*DirNode)
	require.True(t, ok)
	assert.Equal(t, "tui", sub1.name)
}

func TestBuildTree_RootLevelFiles(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "go.mod", Status: git.Modified},
		{Path: "main.go", Status: git.Modified},
	}

	nodes := BuildTree(entries, false)

	require.Len(t, nodes, 2, "should have 2 root-level file nodes")
	f0, ok := nodes[0].(*FileNode)
	require.True(t, ok)
	assert.Equal(t, "go.mod", f0.name)
	assert.Equal(t, "go.mod", f0.fullPath)

	f1, ok := nodes[1].(*FileNode)
	require.True(t, ok)
	assert.Equal(t, "main.go", f1.name)
}

func TestBuildTree_SingleChildCollapsing(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "a/b/c/file.go", Status: git.Modified},
	}

	nodes := BuildTree(entries, false)

	require.Len(t, nodes, 1)
	dirNode, ok := nodes[0].(*DirNode)
	require.True(t, ok)
	assert.Equal(t, "a/b/c", dirNode.name, "single-child chain should be collapsed")
	require.Len(t, dirNode.children, 1)
	f, ok := dirNode.children[0].(*FileNode)
	require.True(t, ok)
	assert.Equal(t, "file.go", f.name)
}

func TestBuildTree_MultiChildNotCollapsed(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "a/b/file1.go", Status: git.Modified},
		{Path: "a/c/file2.go", Status: git.Added},
	}

	nodes := BuildTree(entries, false)

	require.Len(t, nodes, 1)
	dirNode, ok := nodes[0].(*DirNode)
	require.True(t, ok)
	assert.Equal(t, "a", dirNode.name, "multi-child dir should NOT be collapsed")
	assert.Len(t, dirNode.children, 2)
}

func TestBuildTree_MixedRootAndSubdir(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "README.md", Status: git.Modified},
		{Path: "internal/main.go", Status: git.Added},
	}

	nodes := BuildTree(entries, false)

	require.Len(t, nodes, 2, "should have dir + root file")
	// Directories come first, then files.
	dirNode, ok := nodes[0].(*DirNode)
	require.True(t, ok)
	assert.Equal(t, "internal", dirNode.name)

	fileNode, ok := nodes[1].(*FileNode)
	require.True(t, ok)
	assert.Equal(t, "README.md", fileNode.name)
}

func TestBuildTree_Empty(t *testing.T) {
	t.Parallel()
	nodes := BuildTree(nil, false)
	assert.Nil(t, nodes)
}

func TestFileNodeView_StatusIcons(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		status     git.FileStatus
		wantPrefix string
	}{
		{"modified shows pencil", git.Modified, tui.IconModified},
		{"added shows plus", git.Added, tui.IconAdded},
		{"deleted shows minus", git.Deleted, tui.IconDeleted},
		{"renamed shows rename", git.Renamed, tui.IconRenamed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			n := &FileNode{name: "test.go", status: tt.status}
			view := n.View()
			assert.Contains(t, view, tt.wantPrefix)
			assert.Contains(t, view, "test.go")
		})
	}
}

func TestFileNodeView_CheckboxRendering(t *testing.T) {
	t.Parallel()

	t.Run("unchecked shows unchecked icon", func(t *testing.T) {
		t.Parallel()
		n := &FileNode{name: "file.go", status: git.Modified, showCheckbox: true, checked: false}
		view := n.View()
		assert.Contains(t, view, tui.IconUnchecked)
		assert.NotContains(t, view, tui.IconChecked)
	})

	t.Run("checked shows checked icon", func(t *testing.T) {
		t.Parallel()
		n := &FileNode{name: "file.go", status: git.Modified, showCheckbox: true, checked: true}
		view := n.View()
		assert.Contains(t, view, tui.IconChecked)
	})

	t.Run("no checkbox when disabled", func(t *testing.T) {
		t.Parallel()
		n := &FileNode{name: "file.go", status: git.Modified, showCheckbox: false}
		view := n.View()
		assert.NotContains(t, view, tui.IconChecked)
		assert.NotContains(t, view, tui.IconUnchecked)
	})
}

func TestDirNodeView_FolderIconAndName(t *testing.T) {
	t.Parallel()
	n := &DirNode{name: "commands"}
	view := n.View()
	assert.Contains(t, view, tui.IconFolder)
	assert.Contains(t, view, "commands")
}

func TestDirNodeState_IsCollapsible(t *testing.T) {
	t.Parallel()
	n := &DirNode{name: "dir"}
	assert.True(t, n.State().Is(tree.NodeCollapsible))
}

func TestFileNode_ParentPointers(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "a/b/file.go", Status: git.Modified},
	}
	nodes := BuildTree(entries, false)

	dir := nodes[0].(*DirNode)
	file := dir.children[0].(*FileNode)
	assert.Equal(t, dir, file.parent, "file parent should point to dir")
}

func TestNodeAt_TraversesTree(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "a/file1.go", Status: git.Modified},
		{Path: "a/file2.go", Status: git.Added},
		{Path: "b.go", Status: git.Modified},
	}
	nodes := BuildTree(entries, false)

	// Position 0: dir "a"
	n0 := nodeAt(nodes, 0)
	require.NotNil(t, n0)
	dir, ok := n0.(*DirNode)
	require.True(t, ok)
	assert.Equal(t, "a", dir.name)

	// Position 1: "a/file1.go"
	n1 := nodeAt(nodes, 1)
	require.NotNil(t, n1)
	f1, ok := n1.(*FileNode)
	require.True(t, ok)
	assert.Equal(t, "file1.go", f1.name)

	// Position 2: "a/file2.go"
	n2 := nodeAt(nodes, 2)
	require.NotNil(t, n2)
	f2, ok := n2.(*FileNode)
	require.True(t, ok)
	assert.Equal(t, "file2.go", f2.name)

	// Position 3: "b.go"
	n3 := nodeAt(nodes, 3)
	require.NotNil(t, n3)
	f3, ok := n3.(*FileNode)
	require.True(t, ok)
	assert.Equal(t, "b.go", f3.name)

	// Out of bounds
	assert.Nil(t, nodeAt(nodes, 4))
}

func TestFileNode_Init(t *testing.T) {
	t.Parallel()
	n := &FileNode{name: "f.go"}
	assert.Nil(t, n.Init())
}

func TestFileNode_Parent(t *testing.T) {
	t.Parallel()
	parent := &DirNode{name: "dir"}
	n := &FileNode{name: "f.go", parent: parent}
	assert.Equal(t, parent, n.Parent())
}

func TestFileNode_Children(t *testing.T) {
	t.Parallel()
	n := &FileNode{name: "f.go"}
	assert.Nil(t, n.Children())
}

func TestFileNode_FullPath(t *testing.T) {
	t.Parallel()
	n := &FileNode{name: "f.go", fullPath: "src/f.go"}
	assert.Equal(t, "src/f.go", n.FullPath())
}

func TestFileNode_IsChecked(t *testing.T) {
	t.Parallel()
	n := &FileNode{name: "f.go"}
	assert.False(t, n.IsChecked())
	n.checked = true
	assert.True(t, n.IsChecked())
}

func TestFileNode_SetChecked(t *testing.T) {
	t.Parallel()
	n := &FileNode{name: "f.go"}
	n.SetChecked(true)
	assert.True(t, n.checked)
	n.SetChecked(false)
	assert.False(t, n.checked)
}

func TestFileNode_Update(t *testing.T) {
	t.Parallel()
	n := &FileNode{name: "f.go"}
	cmd := n.Update(tree.NodeState(0))
	assert.Nil(t, cmd)
}

func TestFileNode_State_WithChecked(t *testing.T) {
	t.Parallel()
	n := &FileNode{name: "f.go", checked: true}
	assert.True(t, n.State().Is(NodeChecked))

	n.checked = false
	assert.False(t, n.State().Is(NodeChecked))
}

func TestDirNode_Init(t *testing.T) {
	t.Parallel()
	n := &DirNode{name: "dir"}
	assert.Nil(t, n.Init())
}

func TestDirNode_Parent(t *testing.T) {
	t.Parallel()
	parent := &DirNode{name: "root"}
	n := &DirNode{name: "child", parent: parent}
	assert.Equal(t, parent, n.Parent())
}

func TestDirNode_Update(t *testing.T) {
	t.Parallel()
	n := &DirNode{name: "dir"}
	cmd := n.Update(tree.NodeState(0))
	assert.Nil(t, cmd)
}

func TestNewFileTree_CreatesTree(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "a/file1.go", Status: git.Modified},
		{Path: "b.go", Status: git.Added},
	}
	ft := NewFileTree(entries, false)
	ft.SetWidth(80)
	ft.SetHeight(24)
	assert.NotEmpty(t, ft.View())
}

func TestFileTree_SetWidthHeight(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "file.go", Status: git.Modified},
	}
	ft := NewFileTree(entries, false)
	ft.SetWidth(80)
	ft.SetHeight(24)
	// Should not panic, view should still render.
	assert.NotEmpty(t, ft.View())
}

func TestFileTree_SelectedPath_FileNode(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "file.go", Status: git.Modified},
	}
	ft := NewFileTree(entries, false)
	ft.SetWidth(80)
	ft.SetHeight(24)

	// First node is the file (root-level).
	path := ft.SelectedPath()
	assert.Equal(t, "file.go", path)
}

func TestFileTree_SelectedPath_DirNode(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "a/file.go", Status: git.Modified},
	}
	ft := NewFileTree(entries, false)
	ft.SetWidth(80)
	ft.SetHeight(24)

	// First node is dir "a", not a file - SelectedPath returns "".
	path := ft.SelectedPath()
	assert.Empty(t, path)
}

func TestFileTree_SelectedNode(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "file.go", Status: git.Modified},
	}
	ft := NewFileTree(entries, false)
	ft.SetWidth(80)
	ft.SetHeight(24)

	node := ft.SelectedNode()
	require.NotNil(t, node)
	fn, ok := node.(*FileNode)
	require.True(t, ok)
	assert.Equal(t, "file.go", fn.name)
}

func TestFileTree_ToggleChecked(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "file.go", Status: git.Modified},
	}
	ft := NewFileTree(entries, true)
	ft.SetWidth(80)
	ft.SetHeight(24)

	ft.ToggleChecked()
	paths := ft.CheckedPaths()
	assert.Equal(t, []string{"file.go"}, paths)

	ft.ToggleChecked()
	paths = ft.CheckedPaths()
	assert.Empty(t, paths)
}

func TestFileTree_SetAllChecked(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "a.go", Status: git.Modified},
		{Path: "b.go", Status: git.Added},
	}
	ft := NewFileTree(entries, true)
	ft.SetWidth(80)
	ft.SetHeight(24)

	ft.SetAllChecked(true)
	paths := ft.CheckedPaths()
	assert.ElementsMatch(t, []string{"a.go", "b.go"}, paths)

	ft.SetAllChecked(false)
	paths = ft.CheckedPaths()
	assert.Empty(t, paths)
}

func TestFileTree_CheckedPaths_Empty(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "file.go", Status: git.Modified},
	}
	ft := NewFileTree(entries, true)
	paths := ft.CheckedPaths()
	assert.Empty(t, paths)
}

func TestFileTree_SetCheckedByPath(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "a.go", Status: git.Modified},
		{Path: "b.go", Status: git.Added},
	}
	ft := NewFileTree(entries, true)

	ft.SetCheckedByPath("b.go", true)
	paths := ft.CheckedPaths()
	assert.Equal(t, []string{"b.go"}, paths)

	ft.SetCheckedByPath("b.go", false)
	paths = ft.CheckedPaths()
	assert.Empty(t, paths)
}

func TestFileTree_SetCheckedByPath_InSubdir(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "dir/file.go", Status: git.Modified},
	}
	ft := NewFileTree(entries, true)

	ft.SetCheckedByPath("dir/file.go", true)
	paths := ft.CheckedPaths()
	assert.Equal(t, []string{"dir/file.go"}, paths)
}

func TestFileTree_Rebuild(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "old.go", Status: git.Modified},
	}
	ft := NewFileTree(entries, false)
	ft.SetWidth(80)
	ft.SetHeight(24)

	newEntries := []FileEntry{
		{Path: "new1.go", Status: git.Added},
		{Path: "new2.go", Status: git.Modified},
	}
	ft.Rebuild(newEntries)
	ft.SetWidth(80)
	ft.SetHeight(24)

	view := ft.View()
	assert.Contains(t, view, "new1.go")
	assert.Contains(t, view, "new2.go")
	assert.NotContains(t, view, "old.go")
}

func TestFileTree_Update(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "file.go", Status: git.Modified},
	}
	ft := NewFileTree(entries, false)
	ft.SetWidth(80)
	ft.SetHeight(24)

	// Should not panic.
	cmd := ft.Update("some message")
	_ = cmd
}

func TestFileTree_SelectedNode_EmptyTree(t *testing.T) {
	t.Parallel()
	ft := NewFileTree(nil, false)
	assert.Nil(t, ft.SelectedNode())
	assert.Empty(t, ft.SelectedPath())
}

func TestFileTree_ToggleChecked_OnDir(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "dir/file.go", Status: git.Modified},
	}
	ft := NewFileTree(entries, true)
	ft.SetWidth(80)
	ft.SetHeight(24)

	// Cursor is on dir node - toggle should be a no-op.
	ft.ToggleChecked()
	paths := ft.CheckedPaths()
	assert.Empty(t, paths)
}

func TestCountVisible_WithHiddenNodes(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "a.go", Status: git.Modified},
		{Path: "b.go", Status: git.Added},
	}
	nodes := BuildTree(entries, false)
	// Both visible by default.
	assert.Equal(t, 2, countVisible(nodes))
}

func TestCountVisible_Nil(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 0, countVisible(nil))
}
