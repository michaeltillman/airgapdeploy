// Package server exposes the airgapdeploy web UI and its JSON/SSE API. The
// frontend is embedded so the binary is fully self-contained.
package server

import (
	"embed"
	"io/fs"
	"net/http"
	"sync"

	"github.com/michaeltillman/airgapdeploy/internal/config"
	"github.com/michaeltillman/airgapdeploy/internal/runner"
)

//go:embed web
var webFS embed.FS

// Server holds shared state for the HTTP handlers.
type Server struct {
	mu   sync.RWMutex
	cfg  *config.Config
	live bool
	run  *runner.Runner
}

// New builds a Server. When live is true, generated scripts may be executed.
func New(cfg *config.Config, live bool) *Server {
	return &Server{
		cfg:  cfg,
		live: live,
		run:  runner.New(cfg.OutputDir),
	}
}

// Handler returns the root HTTP handler with all routes mounted.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Static frontend, served from the embedded web/ directory at the root.
	sub, _ := fs.Sub(webFS, "web")
	mux.Handle("/", http.FileServer(http.FS(sub)))

	// JSON / SSE API.
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/generate", s.handleGenerate)
	mux.HandleFunc("/api/file", s.handleFile)
	mux.HandleFunc("/api/preflight", s.handlePreflight)
	mux.HandleFunc("/api/run", s.handleRun)
	mux.HandleFunc("/api/meta", s.handleMeta)

	return withNoStore(mux)
}

// withNoStore prevents the browser from caching API responses (config/preflight
// change between runs) while leaving static assets to the file server.
func withNoStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.URL.Path) >= 5 && r.URL.Path[:5] == "/api/" {
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}
