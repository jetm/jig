# jig rebase-interactive - interactive rebase todo editor
#
# Demonstrates: action cycling (pick -> squash) with s, commit reordering
# with J/K, and confirming the rebase plan with w.

Output docs/screenshots/rebase.gif

Set Shell "bash"
Set FontFamily "JetBrains Mono"
Set FontSize 14
Set Width 1200
Set Height 700
Set Theme "Catppuccin Mocha"
Set TypingSpeed 30ms

# --- Hidden setup: temp repo with 5 commits ---
Hide
Type `export PATH="$(go env GOPATH)/bin:$PATH"`
Enter
Sleep 500ms
Type `WORKDIR=$(mktemp -d) && cd "$WORKDIR" && git init && git config user.email 'demo@jig' && git config user.name 'demo'`
Enter
Sleep 500ms

# Commit 1: Add config parser
Type `printf 'package main\n\nimport "encoding/json"\n\ntype Config struct {\n\tHost string\n\tPort int\n}\n\nfunc ParseConfig(data []byte) (*Config, error) {\n\tvar cfg Config\n\tif err := json.Unmarshal(data, &cfg); err != nil {\n\t\treturn nil, err\n\t}\n\treturn &cfg, nil\n}\n' > config.go && git add -A && git commit -m 'Add config parser'`
Enter
Sleep 500ms

# Commit 2: Fix config validation
Type `printf 'package main\n\nimport (\n\t"encoding/json"\n\t"fmt"\n)\n\ntype Config struct {\n\tHost string\n\tPort int\n}\n\nfunc ParseConfig(data []byte) (*Config, error) {\n\tvar cfg Config\n\tif err := json.Unmarshal(data, &cfg); err != nil {\n\t\treturn nil, err\n\t}\n\tif cfg.Host == "" {\n\t\treturn nil, fmt.Errorf("host required")\n\t}\n\treturn &cfg, nil\n}\n' > config.go && git add -A && git commit -m 'Fix config validation'`
Enter
Sleep 500ms

# Commit 3: Add error handling
Type `printf 'package main\n\nimport "net/http"\n\nfunc StartServer(addr string) error {\n\treturn http.ListenAndServe(addr, nil)\n}\n' > server.go && git add -A && git commit -m 'Add error handling'`
Enter
Sleep 500ms

# Commit 4: Update docs
Type `printf '# My App\n\nA simple server with config validation.\n' > README.md && git add -A && git commit -m 'Update docs'`
Enter
Sleep 500ms

# Commit 5: Refactor imports
Type `printf 'package main\n\nimport (\n\t"net/http"\n\t"time"\n)\n\nfunc StartServer(addr string) error {\n\tsrv := &http.Server{\n\t\tAddr: addr,\n\t\tReadTimeout: 10 * time.Second,\n\t}\n\treturn srv.ListenAndServe()\n}\n' > server.go && git add -A && git commit -m 'Refactor imports'`
Enter
Sleep 500ms
Show

# --- Visible demo ---
Type "jig rebase-interactive HEAD~5"
Enter
Wait+Screen /Refactor imports/

Sleep 800ms

# Set first commit action to squash
Type "s"
Sleep 600ms

# Move down
Type "j"
Sleep 600ms

# Move this commit down (reorder)
Type "J"
Sleep 600ms

# Show the reorder result
Sleep 600ms

# Confirm and write the rebase todo
Type "w"

Sleep 2s
