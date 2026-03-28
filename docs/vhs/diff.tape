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
Set Theme "OneDark"
Set TypingSpeed 30ms
Set Padding 0

# --- Hidden setup: temp repo with three modified files ---
Hide
Type `export PATH="$(go env GOPATH)/bin:$PATH"`
Enter
Sleep 500ms
Type "cd $(mktemp -d) && git init && git config user.email demo@jig && git config user.name demo"
Enter
Sleep 500ms
Type `printf 'package main\n\nimport (\n\t"fmt"\n\t"os"\n)\n\nfunc main() {\n\tcfg := LoadConfig("app.yaml")\n\tif cfg == nil {\n\t\tfmt.Fprintln(os.Stderr, "failed to load config")\n\t\tos.Exit(1)\n\t}\n\tsrv := NewServer(cfg)\n\tsrv.Start()\n}\n' > main.go`
Enter
Type `printf 'package main\n\nimport "os"\n\ntype Config struct {\n\tHost string\n\tPort int\n}\n\nfunc LoadConfig(path string) *Config {\n\tif _, err := os.Stat(path); err != nil {\n\t\treturn nil\n\t}\n\treturn &Config{Host: "localhost", Port: 8080}\n}\n' > config.go`
Enter
Type `printf 'package main\n\nimport "net/http"\n\nfunc healthHandler(w http.ResponseWriter, r *http.Request) {\n\tw.WriteHeader(http.StatusOK)\n\tw.Write([]byte("ok"))\n}\n' > handler.go`
Enter
Type "git add -A && git commit -m 'initial'"
Enter
Sleep 1s
Type `sed -i 's/app.yaml/config.yaml/' main.go && sed -i 's/srv.Start()/srv.ListenAndServe()/' main.go`
Enter
Type `sed -i 's/Host string/Host    string/' config.go && sed -i 's/Port int/Port    int/' config.go`
Enter
Type "echo cGFja2FnZSBtYWluCgppbXBvcnQgIm5ldC9odHRwIgoKZnVuYyBoZWFsdGhIYW5kbGVyKHcgaHR0cC5SZXNwb25zZVdyaXRlciwgciAqaHR0cC5SZXF1ZXN0KSB7Cgl3LkhlYWRlcigpLlNldCgiQ29udGVudC1UeXBlIiwgImFwcGxpY2F0aW9uL2pzb24iKQoJdy5Xcml0ZUhlYWRlcihodHRwLlN0YXR1c09LKQoJdy5Xcml0ZShbXWJ5dGUoYHsic3RhdHVzIjogImhlYWx0aHkifWApKQp9Cg== | base64 -d > handler.go"
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
