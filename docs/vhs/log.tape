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

# --- Hidden setup: temp repo with 6 commits ---
Hide
Type `export PATH="$(go env GOPATH)/bin:$PATH"`
Enter
Sleep 500ms
Type "cd $(mktemp -d) && git init && git config user.email 'demo@jig' && git config user.name 'demo'"
Enter
Sleep 500ms

# Commit 1: Add initial config
Type "cat > config.go << 'EOF'"
Enter
Type "package main"
Enter
Type ""
Enter
Type "type Config struct {"
Enter
Type "	Host string"
Enter
Type "	Port int"
Enter
Type "}"
Enter
Type "EOF"
Enter
Type "git add -A && git commit -m 'Add initial config'"
Enter
Sleep 500ms

# Commit 2: Fix validation logic
Type "cat > config.go << 'EOF'"
Enter
Type "package main"
Enter
Type ""
Enter
Type "import \"fmt\""
Enter
Type ""
Enter
Type "type Config struct {"
Enter
Type "	Host string"
Enter
Type "	Port int"
Enter
Type "}"
Enter
Type ""
Enter
Type "func (c *Config) Validate() error {"
Enter
Type "	if c.Host == \"\" {"
Enter
Type "		return fmt.Errorf(\"host is required\")"
Enter
Type "	}"
Enter
Type "	if c.Port < 1 || c.Port > 65535 {"
Enter
Type "		return fmt.Errorf(\"invalid port: %d\", c.Port)"
Enter
Type "	}"
Enter
Type "	return nil"
Enter
Type "}"
Enter
Type "EOF"
Enter
Type "git add -A && git commit -m 'Fix validation logic'"
Enter
Sleep 500ms

# Commit 3: Add error handling
Type "cat > server.go << 'EOF'"
Enter
Type "package main"
Enter
Type ""
Enter
Type "import ("
Enter
Type "	\"fmt\""
Enter
Type "	\"net/http\""
Enter
Type ")"
Enter
Type ""
Enter
Type "func StartServer(cfg *Config) error {"
Enter
Type "	if err := cfg.Validate(); err != nil {"
Enter
Type "		return fmt.Errorf(\"config error: %w\", err)"
Enter
Type "	}"
Enter
Type "	addr := fmt.Sprintf(\"%s:%d\", cfg.Host, cfg.Port)"
Enter
Type "	return http.ListenAndServe(addr, nil)"
Enter
Type "}"
Enter
Type "EOF"
Enter
Type "git add -A && git commit -m 'Add error handling'"
Enter
Sleep 500ms

# Commit 4: Refactor parser
Type "cat > parser.go << 'EOF'"
Enter
Type "package main"
Enter
Type ""
Enter
Type "import ("
Enter
Type "	\"encoding/json\""
Enter
Type "	\"os\""
Enter
Type ")"
Enter
Type ""
Enter
Type "func ParseConfig(path string) (*Config, error) {"
Enter
Type "	data, err := os.ReadFile(path)"
Enter
Type "	if err != nil {"
Enter
Type "		return nil, err"
Enter
Type "	}"
Enter
Type "	var cfg Config"
Enter
Type "	if err := json.Unmarshal(data, &cfg); err != nil {"
Enter
Type "		return nil, err"
Enter
Type "	}"
Enter
Type "	return &cfg, nil"
Enter
Type "}"
Enter
Type "EOF"
Enter
Type "git add -A && git commit -m 'Refactor parser'"
Enter
Sleep 500ms

# Commit 5: Update dependencies
Type "cat > server.go << 'EOF'"
Enter
Type "package main"
Enter
Type ""
Enter
Type "import ("
Enter
Type "	\"context\""
Enter
Type "	\"fmt\""
Enter
Type "	\"net/http\""
Enter
Type "	\"time\""
Enter
Type ")"
Enter
Type ""
Enter
Type "func StartServer(cfg *Config) error {"
Enter
Type "	if err := cfg.Validate(); err != nil {"
Enter
Type "		return fmt.Errorf(\"config error: %w\", err)"
Enter
Type "	}"
Enter
Type "	srv := &http.Server{"
Enter
Type "		Addr:         fmt.Sprintf(\"%s:%d\", cfg.Host, cfg.Port),"
Enter
Type "		ReadTimeout:  15 * time.Second,"
Enter
Type "		WriteTimeout: 15 * time.Second,"
Enter
Type "	}"
Enter
Type "	ctx := context.Background()"
Enter
Type "	_ = ctx"
Enter
Type "	return srv.ListenAndServe()"
Enter
Type "}"
Enter
Type "EOF"
Enter
Type "git add -A && git commit -m 'Update dependencies'"
Enter
Sleep 500ms

# Commit 6: Add logging
Type "cat > logger.go << 'EOF'"
Enter
Type "package main"
Enter
Type ""
Enter
Type "import ("
Enter
Type "	\"log\""
Enter
Type "	\"os\""
Enter
Type ")"
Enter
Type ""
Enter
Type "var logger = log.New(os.Stderr, \"[app] \", log.LstdFlags)"
Enter
Type ""
Enter
Type "func LogInfo(msg string) {"
Enter
Type "	logger.Printf(\"INFO: %s\", msg)"
Enter
Type "}"
Enter
Type ""
Enter
Type "func LogError(msg string, err error) {"
Enter
Type "	logger.Printf(\"ERROR: %s: %v\", msg, err)"
Enter
Type "}"
Enter
Type "EOF"
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
