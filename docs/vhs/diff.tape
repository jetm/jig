# jig diff - interactive side-by-side diff viewer
#
# Demonstrates: file list navigation with j/k, Tab to switch to diff panel,
# scrolling through diff content, three modified Go files.

Output docs/screenshots/diff.gif

Set Shell "bash"
Set FontFamily "CommitMono Nerd Font"
Set FontSize 14
Set Width 1200
Set Height 700
Set Theme "Catppuccin Mocha"
Set TypingSpeed 30ms

# --- Hidden setup: temp repo with three modified files ---
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
Type "import ("
Enter
Type "	\"fmt\""
Enter
Type "	\"os\""
Enter
Type ")"
Enter
Type ""
Enter
Type "func main() {"
Enter
Type "	cfg := LoadConfig(\"app.yaml\")"
Enter
Type "	if cfg == nil {"
Enter
Type "		fmt.Fprintln(os.Stderr, \"failed to load config\")"
Enter
Type "		os.Exit(1)"
Enter
Type "	}"
Enter
Type "	srv := NewServer(cfg)"
Enter
Type "	srv.Start()"
Enter
Type "}"
Enter
Type "EOF"
Enter
Type "cat > config.go << 'EOF'"
Enter
Type "package main"
Enter
Type ""
Enter
Type "import \"os\""
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
Type "func LoadConfig(path string) *Config {"
Enter
Type "	if _, err := os.Stat(path); err != nil {"
Enter
Type "		return nil"
Enter
Type "	}"
Enter
Type "	return &Config{Host: \"localhost\", Port: 8080}"
Enter
Type "}"
Enter
Type "EOF"
Enter
Type "cat > handler.go << 'EOF'"
Enter
Type "package main"
Enter
Type ""
Enter
Type "import \"net/http\""
Enter
Type ""
Enter
Type "func healthHandler(w http.ResponseWriter, r *http.Request) {"
Enter
Type "	w.WriteHeader(http.StatusOK)"
Enter
Type "	w.Write([]byte(\"ok\"))"
Enter
Type "}"
Enter
Type "EOF"
Enter
Type "git add -A && git commit -m 'initial'"
Enter
Sleep 1s
Type "sed -i 's/\"app.yaml\"/os.Getenv(\"CONFIG_PATH\")/' main.go"
Enter
Type "sed -i 's/cfg == nil/cfg == nil || cfg.Host == \"\"/' main.go"
Enter
Type "sed -i 's/srv.Start()/srv.ListenAndServe()/' main.go"
Enter
Type "sed -i 's/Host string/Host    string/' config.go"
Enter
Type "sed -i 's/Port int/Port    int\\n\\tTimeout int/' config.go"
Enter
Type "sed -i 's/Port: 8080/Port: 8080, Timeout: 30/' config.go"
Enter
Type "sed -i 's/StatusOK/StatusOK\\n\\tw.Header().Set(\"Content-Type\", \"application\\/json\")/' handler.go"
Enter
Type "sed -i 's/\"ok\"/\"{\\\\\"status\\\\\": \\\\\"healthy\\\\\"}\"/' handler.go"
Enter
Sleep 300ms
Show

# --- Visible demo ---
Type "jig diff"
Enter
Wait+Screen /main.go/

Sleep 800ms

# Navigate down through file list with j
Type "j"
Sleep 800ms

Type "j"
Sleep 800ms

# Switch to diff panel
Tab
Sleep 600ms

# Scroll through the diff
Down
Sleep 400ms
Down
Sleep 400ms
Down
Sleep 400ms
Down
Sleep 400ms

Sleep 1s

# Quit
Type "q"
Sleep 500ms
