# jig help overlay - keybinding reference composited over TUI
#
# Demonstrates: ? to open help overlay rendered over the background,
# q to dismiss (consumed by overlay, does not quit).

Output docs/screenshots/help-overlay.gif

Set Shell "bash"
Set FontFamily "CommitMono Nerd Font"
Set FontSize 14
Set Width 1200
Set Height 700
Set Theme "OneDark"
Set TypingSpeed 30ms
Set Padding 0

# --- Hidden setup: temp repo with a modified file ---
Hide
Type `export PATH="$(go env GOPATH)/bin:$PATH"`
Enter
Sleep 500ms
Type `cd $(mktemp -d) && git init && git config user.email 'demo@jig' && git config user.name 'demo'`
Enter
Sleep 500ms
Type `printf 'package main\n\nimport "fmt"\n\nfunc main() {\n\tfmt.Println("hello")\n}\n' > main.go`
Enter
Sleep 300ms
Type "git add -A && git commit -m 'initial'"
Enter
Sleep 1s
Type "sed -i 's/hello/hello world/' main.go"
Enter
Sleep 300ms
Show

# --- Visible demo ---
Type "jig add"
Enter
Wait+Screen /main.go/

Sleep 1s

# Open help overlay with ?
Type "?"
Wait+Screen /Navigation/

Sleep 2s

# Dismiss overlay with q (consumed by overlay, TUI stays)
Type "q"

Sleep 1s

# Quit the TUI
Type "q"

Sleep 1s
