# jig hunk-add - partial hunk staging
#
# Demonstrates: hunk list with 2 hunks, Space to toggle first hunk (staged),
# skip second hunk, Enter to apply only the staged hunk.

Output docs/screenshots/hunk-add.gif

Set Shell "bash"
Set FontFamily "CommitMono Nerd Font"
Set FontSize 14
Set Width 1200
Set Height 700
Set Theme "Catppuccin Mocha"
Set TypingSpeed 30ms

# --- Hidden setup: temp repo with two separated hunks ---
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
Type "func greet() {"
Enter
Type "	fmt.Println(\"hello\")"
Enter
Type "}"
Enter
Type ""
Enter
Type "func pad() string {"
Enter
Type "	return \"    \""
Enter
Type "}"
Enter
Type ""
Enter
Type "func sep() string {"
Enter
Type "	return \"---\""
Enter
Type "}"
Enter
Type ""
Enter
Type "func footer() {"
Enter
Type "	fmt.Println(\"done\")"
Enter
Type "}"
Enter
Type "EOF"
Enter
Type "git add -A && git commit -m 'initial'"
Enter
Sleep 1s
Type "sed -i 's/\"hello\"/\"hi there\"/' main.go"
Enter
Type "sed -i 's/\"done\"/\"finished\"/' main.go"
Enter
Sleep 300ms
Show

# --- Visible demo ---
Type "jig hunk-add"
Enter
Wait+Screen /hunk/

Sleep 1s

# Toggle first hunk (staged)
Space

Sleep 1s

# Move to second hunk (leave unstaged)
Type "j"

Sleep 1500ms

# Apply only the staged hunk
Enter

Sleep 2s
