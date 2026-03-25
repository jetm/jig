# jig add - interactive staging with diff preview
#
# Demonstrates: file list with checkboxes, Space to select, Enter to stage,
# syntax-highlighted diff panel.

Output docs/screenshots/add.gif

Set Shell "bash"
Set FontFamily "CommitMono Nerd Font"
Set FontSize 14
Set Width 1200
Set Height 700
Set Theme "OneDark"
Set TypingSpeed 30ms

# --- Hidden setup: temp repo with two modified files ---
Hide
Type `export PATH="$(go env GOPATH)/bin:$PATH"`
Enter
Sleep 500ms
Type "cd $(mktemp -d) && git init && git config user.email 'demo@jig' && git config user.name 'demo'"
Enter
Sleep 500ms
Type "cat > main.go << 'EOF'"
Enter
Type "package main"
Enter
Type ""
Enter
Type "import \"fmt\""
Enter
Type ""
Enter
Type "func main() {"
Enter
Type "	fmt.Println(\"hello\")"
Enter
Type "}"
Enter
Type "EOF"
Enter
Type "cat > utils.go << 'EOF'"
Enter
Type "package main"
Enter
Type ""
Enter
Type "func add(a, b int) int {"
Enter
Type "	return a + b"
Enter
Type "}"
Enter
Type "EOF"
Enter
Type "git add -A && git commit -m 'initial'"
Enter
Sleep 1s
Type "sed -i 's/hello/hello world/' main.go"
Enter
Type "sed -i 's/a + b/a + b + 1/' utils.go"
Enter
Sleep 300ms
Show

# --- Visible demo ---
Type "jig add"
Enter
Wait+Screen /main.go/

Sleep 1s

# Select the first file
Space

Sleep 1s

# Stage the selected file
Enter

Sleep 2s
