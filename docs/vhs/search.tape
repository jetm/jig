# jig diff - search within diff panel
#
# Demonstrates: Tab to diff panel, / to open search, type pattern,
# Enter to execute, n to cycle through highlighted matches.

Output docs/screenshots/search.gif

Set Shell "bash"
Set FontFamily "CommitMono Nerd Font"
Set FontSize 14
Set Width 1200
Set Height 700
Set Theme "OneDark"
Set TypingSpeed 30ms
Set Padding 0

# --- Hidden setup: temp repo with Go file containing "error" multiple times ---
Hide
Type `export PATH="$(go env GOPATH)/bin:$PATH"`
Enter
Sleep 500ms
Type "cd $(mktemp -d) && git init && git config user.email 'demo@jig' && git config user.name 'demo'"
Enter
Sleep 500ms
Type `base64 -d > main.go << 'B64'`
Enter
Type `cGFja2FnZSBtYWluCgppbXBvcnQgKAoJImZtdCIKCSJvcyIKKQoKZnVuYyBtYWluKCkgewoJZiwgZXJyIDo9IG9zLk9wZW4oImRhdGEudHh0IikKCWlmIGVyciAhPSBuaWwgewoJCWZtdC5QcmludGxuKCJlcnJvcjoiLCBlcnIpCgkJb3MuRXhpdCgxKQoJfQoJZGVmZXIgZi5DbG9zZSgpCglmbXQuUHJpbnRsbigiZG9uZSIpCn0K`
Enter
Type "B64"
Enter
Sleep 300ms
Type "git add -A && git commit -m 'initial'"
Enter
Sleep 1s
Type `base64 -d > main.go << 'B64'`
Enter
Type `cGFja2FnZSBtYWluCgppbXBvcnQgKAoJImZtdCIKCSJsb2ciCgkib3MiCikKCmZ1bmMgbWFpbigpIHsKCWYsIGVyciA6PSBvcy5PcGVuKCJkYXRhLnR4dCIpCglpZiBlcnIgIT0gbmlsIHsKCQlsb2cuRmF0YWxmKCJlcnJvciBvcGVuaW5nIGZpbGU6ICV2IiwgZXJyKQoJfQoJZGVmZXIgZi5DbG9zZSgpCgoJZGF0YSA6PSBtYWtlKFtdYnl0ZSwgMTAyNCkKCW4sIGVyciA6PSBmLlJlYWQoZGF0YSkKCWlmIGVyciAhPSBuaWwgewoJCWxvZy5GYXRhbGYoImVycm9yIHJlYWRpbmcgZGF0YTogJXYiLCBlcnIpCgl9CgoJaWYgbiA9PSAwIHsKCQlmbXQuUHJpbnRsbigiZXJyb3I6IGVtcHR5IGZpbGUiKQoJCW9zLkV4aXQoMSkKCX0KCglmbXQuUHJpbnRmKCJyZWFkICVkIGJ5dGVzXG4iLCBuKQp9Cg==`
Enter
Type "B64"
Enter
Sleep 300ms
Show

# --- Visible demo ---
Type "jig diff"
Enter
Wait+Screen /main.go/

Sleep 800ms

# Switch to diff panel
Tab
Sleep 600ms

# Open search with /
Type "/"
Sleep 500ms

# Type the search pattern
Type "error"
Sleep 500ms

# Execute search
Enter
Sleep 1s

# Cycle to next match
Type "n"
Sleep 500ms

# Cycle again
Type "n"
Sleep 500ms

Sleep 1s

# Quit
Type "q"
Sleep 500ms
