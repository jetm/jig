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
Set Theme "Catppuccin Mocha"
Set TypingSpeed 30ms

# --- Hidden setup: temp repo with 4 commits and a staged change ---
Hide
Type `export PATH="$(go env GOPATH)/bin:$PATH"`
Enter
Sleep 500ms
Type "cd $(mktemp -d) && git init && git config user.email 'demo@jig' && git config user.name 'demo'"
Enter
Sleep 500ms

# Commit 1: Add config struct
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
Type "git add -A && git commit -m 'Add config struct'"
Enter
Sleep 500ms

# Commit 2: Add validation
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
Type "		return fmt.Errorf(\"host required\")"
Enter
Type "	}"
Enter
Type "	return nil"
Enter
Type "}"
Enter
Type "EOF"
Enter
Type "git add -A && git commit -m 'Add validation'"
Enter
Sleep 500ms

# Commit 3: Add server
Type "cat > server.go << 'EOF'"
Enter
Type "package main"
Enter
Type ""
Enter
Type "import \"net/http\""
Enter
Type ""
Enter
Type "func StartServer(addr string) error {"
Enter
Type "	return http.ListenAndServe(addr, nil)"
Enter
Type "}"
Enter
Type "EOF"
Enter
Type "git add -A && git commit -m 'Add server'"
Enter
Sleep 500ms

# Commit 4: Add logging
Type "cat > logger.go << 'EOF'"
Enter
Type "package main"
Enter
Type ""
Enter
Type "import \"log\""
Enter
Type ""
Enter
Type "func LogInfo(msg string) {"
Enter
Type "	log.Printf(\"INFO: %s\", msg)"
Enter
Type "}"
Enter
Type "EOF"
Enter
Type "git add -A && git commit -m 'Add logging'"
Enter
Sleep 500ms

# Stage a fix to validation (targets "Add validation" commit)
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
Type "		return fmt.Errorf(\"host required\")"
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
