# jig log - interactive commit log browser
#
# Demonstrates: commit list navigation with j/k, diff preview updating
# per commit, six commits with realistic messages.

Output docs/screenshots/log.gif

Set Shell "bash"
Set FontFamily "CommitMono Nerd Font"
Set FontSize 14
Set Width 1200
Set Height 700
Set Theme "OneDark"
Set TypingSpeed 30ms
Set Padding 0

# --- Hidden setup: temp repo with 6 commits ---
Hide
Type `export PATH="$(go env GOPATH)/bin:$PATH"`
Enter
Sleep 500ms
Type "REPO=$(mktemp -d) && cd $REPO && git init && git config user.email demo@jig && git config user.name demo"
Enter
Sleep 500ms

# Commit 1: Add initial config
Type `printf 'package main\n\ntype Config struct {\n\tHost string\n\tPort int\n}\n' > config.go`
Enter
Type "git add -A && git commit -m 'Add initial config'"
Enter
Sleep 500ms

# Commit 2: Fix validation logic
Type `printf 'package main\n\nimport "fmt"\n\ntype Config struct {\n\tHost string\n\tPort int\n}\n\nfunc (c *Config) Validate() error {\n\tif c.Host == "" {\n\t\treturn fmt.Errorf("host is required")\n\t}\n\tif c.Port < 1 || c.Port > 65535 {\n\t\treturn fmt.Errorf("invalid port")\n\t}\n\treturn nil\n}\n' > config.go`
Enter
Type "git add -A && git commit -m 'Fix validation logic'"
Enter
Sleep 500ms

# Commit 3: Add error handling
Type `printf 'package main\n\nimport (\n\t"fmt"\n\t"net/http"\n)\n\nfunc StartServer(cfg *Config) error {\n\tif err := cfg.Validate(); err != nil {\n\t\treturn fmt.Errorf("config: %%w", err)\n\t}\n\taddr := fmt.Sprintf("%%s:%%d", cfg.Host, cfg.Port)\n\treturn http.ListenAndServe(addr, nil)\n}\n' > server.go`
Enter
Type "git add -A && git commit -m 'Add error handling'"
Enter
Sleep 500ms

# Commit 4: Refactor parser
Type `printf 'package main\n\nimport (\n\t"encoding/json"\n\t"os"\n)\n\nfunc ParseConfig(path string) (*Config, error) {\n\tdata, err := os.ReadFile(path)\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\tvar cfg Config\n\tif err := json.Unmarshal(data, &cfg); err != nil {\n\t\treturn nil, err\n\t}\n\treturn &cfg, nil\n}\n' > parser.go`
Enter
Type "git add -A && git commit -m 'Refactor parser'"
Enter
Sleep 500ms

# Commit 5: Update dependencies
Type `printf 'package main\n\nimport (\n\t"context"\n\t"fmt"\n\t"net/http"\n\t"time"\n)\n\nfunc StartServer(cfg *Config) error {\n\tif err := cfg.Validate(); err != nil {\n\t\treturn fmt.Errorf("config: %%w", err)\n\t}\n\tsrv := &http.Server{\n\t\tAddr:         fmt.Sprintf("%%s:%%d", cfg.Host, cfg.Port),\n\t\tReadTimeout:  15 * time.Second,\n\t\tWriteTimeout: 15 * time.Second,\n\t}\n\tctx := context.Background()\n\t_ = ctx\n\treturn srv.ListenAndServe()\n}\n' > server.go`
Enter
Type "git add -A && git commit -m 'Update dependencies'"
Enter
Sleep 500ms

# Commit 6: Add logging
Type `printf 'package main\n\nimport (\n\t"log"\n\t"os"\n)\n\nvar logger = log.New(os.Stderr, "[app] ", log.LstdFlags)\n\nfunc LogInfo(msg string) {\n\tlogger.Printf("INFO: %%s", msg)\n}\n\nfunc LogError(msg string, err error) {\n\tlogger.Printf("ERROR: %%s: %%v", msg, err)\n}\n' > logger.go`
Enter
Type "git add -A && git commit -m 'Add logging'"
Enter
Sleep 500ms
Show

# --- Visible demo ---
Type "jig log"
Enter
Wait+Screen /Add logging/

Sleep 800ms

# Navigate down through commits with j
Type "j"
Sleep 600ms

Type "j"
Sleep 600ms

Type "j"
Sleep 600ms

Type "j"
Sleep 600ms

Sleep 1s

# Quit
Type "q"
Sleep 500ms
