# jig fixup - interactive fixup commit creation
#
# Demonstrates: commit list with staged changes, navigate with j/k to
# target commit, Enter to create fixup and autosquash.

Output docs/screenshots/fixup.gif

Set Shell "bash"
Set FontFamily "CommitMono Nerd Font"
Set FontSize 14
Set Width 1200
Set Height 700
Set Theme "OneDark"
Set TypingSpeed 30ms

# --- Hidden setup: temp repo with 4 commits and a staged change ---
Hide
Type `export PATH="$(go env GOPATH)/bin:$PATH"`
Enter
Sleep 500ms
Type "cd $(mktemp -d) && git init && git config user.email demo@jig && git config user.name demo"
Enter
Sleep 500ms

# Commit 1: Add config struct
Type `printf 'package main\n\ntype Config struct {\n\tHost string\n\tPort int\n}\n' > config.go`
Enter
Type "git add -A && git commit -m 'Add config struct'"
Enter
Sleep 500ms

# Commit 2: Add validation
Type `printf 'package main\n\nimport "fmt"\n\ntype Config struct {\n\tHost string\n\tPort int\n}\n\nfunc (c *Config) Validate() error {\n\tif c.Host == "" {\n\t\treturn fmt.Errorf("host required")\n\t}\n\treturn nil\n}\n' > config.go`
Enter
Type "git add -A && git commit -m 'Add validation'"
Enter
Sleep 500ms

# Commit 3: Add server
Type `printf 'package main\n\nimport "net/http"\n\nfunc StartServer(addr string) error {\n\treturn http.ListenAndServe(addr, nil)\n}\n' > server.go`
Enter
Type "git add -A && git commit -m 'Add server'"
Enter
Sleep 500ms

# Commit 4: Add logging
Type `printf 'package main\n\nimport "log"\n\nfunc LogInfo(msg string) {\n\tlog.Println("INFO:", msg)\n}\n' > logger.go`
Enter
Type "git add -A && git commit -m 'Add logging'"
Enter
Sleep 500ms

# Stage a fix to validation (targets "Add validation" commit)
Type `printf 'package main\n\nimport "fmt"\n\ntype Config struct {\n\tHost string\n\tPort int\n}\n\nfunc (c *Config) Validate() error {\n\tif c.Host == "" {\n\t\treturn fmt.Errorf("host required")\n\t}\n\tif c.Port < 1 || c.Port > 65535 {\n\t\treturn fmt.Errorf("invalid port")\n\t}\n\treturn nil\n}\n' > config.go`
Enter
Type "git add config.go"
Enter
Sleep 300ms
Show

# --- Visible demo ---
Type "jig fixup"
Enter
Wait+Screen /Add logging/

Sleep 800ms

# Navigate down to "Add validation" commit
Type "j"
Sleep 600ms

Type "j"
Sleep 800ms

# Press Enter to fixup against "Add validation"
Enter

Sleep 2s
