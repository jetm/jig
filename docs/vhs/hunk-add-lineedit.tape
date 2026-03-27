# jig hunk-add line edit - selective line staging within a hunk
#
# Demonstrates: Enter to enter line-edit mode, Space to deselect individual
# lines, j to navigate, Esc to exit, w to apply partial hunk.

Output docs/screenshots/hunk-add-lineedit.gif

Set Shell "bash"
Set FontFamily "CommitMono Nerd Font"
Set FontSize 14
Set Width 1200
Set Height 700
Set Theme "OneDark"
Set TypingSpeed 30ms

# --- Hidden setup: temp repo with a single hunk of multiple changed lines ---
Hide
Type `export PATH="$(go env GOPATH)/bin:$PATH"`
Enter
Sleep 500ms
Type "cd $(mktemp -d) && git init && git config user.email demo@jig && git config user.name demo"
Enter
Sleep 500ms
Type `printf 'package main\n\nfunc serve() {\n\thost := "localhost"\n\tport := 8080\n\ttimeout := 30\n\tretries := 3\n\tdebug := false\n\tstart(host, port, timeout, retries, debug)\n}\n' > server.go`
Enter
Type "git add -A && git commit -m 'initial'"
Enter
Sleep 1s
Type `printf 'package main\n\nfunc serve() {\n\thost := "0.0.0.0"\n\tport := 9090\n\ttimeout := 60\n\tretries := 5\n\tdebug := true\n\tstart(host, port, timeout, retries, debug)\n}\n' > server.go`
Enter
Sleep 300ms
Show

# --- Visible demo ---
Type "jig hunk-add"
Enter
Wait+Screen /server.go/

Sleep 800ms

# Enter line-edit mode for the current hunk
Enter

Sleep 800ms

# Deselect first changed line (host) - keep it unstaged
Space

Sleep 500ms

# Move down to next line
Type "j"

Sleep 400ms

# Skip port (leave selected), move to timeout
Type "j"

Sleep 400ms

# Deselect timeout - keep it unstaged
Space

Sleep 500ms

# Exit line-edit mode back to hunk list
Escape

Sleep 600ms

# Apply the partially selected hunk
Type "w"

Sleep 2s
