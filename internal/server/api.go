package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/michaeltillman/airgapdeploy/internal/config"
	"github.com/michaeltillman/airgapdeploy/internal/generate"
	"github.com/michaeltillman/airgapdeploy/internal/runner"
)

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// handleMeta reports server capabilities to the frontend (e.g. live mode).
func (s *Server) handleMeta(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	live := s.live
	out := s.cfg.OutputDir
	s.mu.RUnlock()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"live":      live,
		"outputDir": out,
		"version":   "0.1.0",
	})
}

// handleConfig GETs the current config or POSTs a replacement (normalized).
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.mu.RLock()
		cfg := *s.cfg
		s.mu.RUnlock()
		writeJSON(w, http.StatusOK, &cfg)
	case http.MethodPost:
		cfg, err := decodeConfig(r)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		s.mu.Lock()
		s.cfg = cfg
		s.run = runner.New(cfg.OutputDir)
		s.mu.Unlock()
		writeJSON(w, http.StatusOK, cfg)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleGenerate renders and writes the artifact set from the posted config.
func (s *Server) handleGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	cfg, err := decodeConfig(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	res, err := generate.Run(cfg)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	s.mu.Lock()
	s.cfg = cfg
	s.run = runner.New(cfg.OutputDir)
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, res)
}

// handleFile returns the content of a generated file for display in the UI.
func (s *Server) handleFile(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		writeErr(w, http.StatusBadRequest, "missing name")
		return
	}
	s.mu.RLock()
	root := s.cfg.OutputDir
	s.mu.RUnlock()

	abs, err := safeJoin(root, name)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	b, err := os.ReadFile(abs)
	if err != nil {
		writeErr(w, http.StatusNotFound, "file not found (generate artifacts first)")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"name": name, "content": string(b)})
}

// handlePreflight reports which CLI tools are available.
func (s *Server) handlePreflight(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"tools": runner.Preflight(),
	})
}

// handleRun executes a generated script and streams its output over SSE.
func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	live := s.live
	run := s.run
	s.mu.RUnlock()

	if !live {
		writeErr(w, http.StatusForbidden, "live execution is disabled; start airgapdeploy with --live")
		return
	}
	name := r.URL.Query().Get("name")
	if name == "" {
		writeErr(w, http.StatusBadRequest, "missing name")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "keep-alive")

	lines := make(chan runner.Line, 64)
	go run.Run(r.Context(), name, lines)

	for ln := range lines {
		payload, _ := json.Marshal(ln)
		fmt.Fprintf(w, "data: %s\n\n", payload)
		flusher.Flush()
	}
}

// decodeConfig reads a Config from the request body, applying defaults for
// omitted fields, then normalizing.
func decodeConfig(r *http.Request) (*config.Config, error) {
	cfg := config.Default()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(cfg); err != nil {
		return nil, fmt.Errorf("invalid config JSON: %w", err)
	}
	cfg.Normalize()
	return cfg, nil
}

// safeJoin joins name onto root, rejecting path traversal outside root.
func safeJoin(root, name string) (string, error) {
	clean := filepath.Clean("/" + filepath.FromSlash(name))[1:]
	if clean == "" || strings.HasPrefix(clean, "..") {
		return "", fmt.Errorf("invalid file name")
	}
	abs := filepath.Join(root, clean)
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	absAbs, err := filepath.Abs(abs)
	if err != nil {
		return "", err
	}
	if absAbs != rootAbs && !strings.HasPrefix(absAbs, rootAbs+string(os.PathSeparator)) {
		return "", fmt.Errorf("file is outside the output directory")
	}
	return absAbs, nil
}
