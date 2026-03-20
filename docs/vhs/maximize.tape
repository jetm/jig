# jig maximize - full-width diff panel toggle
#
# Demonstrates: F to maximize diff panel (hides file list),
# j to navigate files while maximized, F again to restore two-panel layout.

Output docs/screenshots/maximize.gif

Set Shell "bash"
Set FontFamily "JetBrains Mono"
Set FontSize 14
Set Width 1200
Set Height 700
Set Theme "Catppuccin Mocha"
Set TypingSpeed 30ms

# --- Hidden setup: temp repo with modified Go files ---
Hide
Type `export PATH="$(go env GOPATH)/bin:$PATH"`
Enter
Sleep 500ms
Type `cd $(mktemp -d) && git init && git config user.email 'demo@jig' && git config user.name 'demo'`
Enter
Sleep 500ms
Type `printf 'package main\n\nimport (\n\t"fmt"\n\t"net/http"\n)\n\ntype Server struct {\n\tHost string\n\tPort int\n}\n\nfunc (s *Server) Start() error {\n\taddr := fmt.Sprintf("%%s:%%d", s.Host, s.Port)\n\treturn http.ListenAndServe(addr, nil)\n}\n\nfunc (s *Server) Stop() {\n\tfmt.Println("shutting down")\n}\n' > server.go`
Enter
Sleep 300ms
Type `printf 'package main\n\nimport "net/http"\n\nfunc SetupRoutes(mux *http.ServeMux) {\n\tmux.HandleFunc("/health", healthCheck)\n\tmux.HandleFunc("/api/v1/status", statusHandler)\n}\n\nfunc healthCheck(w http.ResponseWriter, r *http.Request) {\n\tw.WriteHeader(http.StatusOK)\n}\n\nfunc statusHandler(w http.ResponseWriter, r *http.Request) {\n\tw.Write([]byte("running"))\n}\n' > router.go`
Enter
Sleep 300ms
Type `git add -A && git commit -m 'initial'`
Enter
Sleep 1s
Type `sed -i 's/Host string/Host    string/' server.go`
Enter
Type `sed -i 's/Port int/Port    int\n\tTimeout int/' server.go`
Enter
Type `sed -i 's/shutting down/graceful shutdown initiated/' server.go`
Enter
Type `sed -i 's|/health|/healthz|' router.go`
Enter
Type `sed -i 's|/api/v1/status|/api/v2/status|' router.go`
Enter
Type `sed -i '/StatusOK/{n;s/}/\tw.Write([]byte("ok"))\n}/}' router.go`
Enter
Sleep 300ms
Show

# --- Visible demo ---
Type "jig diff"
Enter
Wait+Screen /server.go/

# Show normal two-panel layout
Sleep 1s

# Maximize diff panel with F
Type "F"
Sleep 1.5s

# Navigate to next file while maximized
Type "j"
Sleep 1s

# Restore two-panel layout with F
Type "F"
Sleep 1s

# Quit
Type "q"
Sleep 500ms
