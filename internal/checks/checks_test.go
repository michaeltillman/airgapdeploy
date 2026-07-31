package checks

import (
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/michaeltillman/airgapdeploy/internal/config"
)

func TestHostPort(t *testing.T) {
	cases := []struct{ in, host, hp string }{
		{"registry.local", "registry.local", "registry.local:443"},
		{"registry.local:5000", "registry.local", "registry.local:5000"},
		{"registry.local/k0rdent", "registry.local", "registry.local:443"},
	}
	for _, c := range cases {
		h, hp := hostPort(c.in, true)
		if h != c.host || hp != c.hp {
			t.Errorf("hostPort(%q) = %q,%q want %q,%q", c.in, h, hp, c.host, c.hp)
		}
	}
}

func TestCheckCACert(t *testing.T) {
	dir := t.TempDir()

	// Missing file -> fail.
	cfg := config.Default()
	cfg.CACertPath = filepath.Join(dir, "nope.crt")
	if r := checkCACert(cfg); r.Status != Fail {
		t.Errorf("missing CA cert should fail, got %v", r)
	}

	// Garbage content -> fail.
	bad := filepath.Join(dir, "bad.crt")
	_ = os.WriteFile(bad, []byte("not a cert"), 0o644)
	cfg.CACertPath = bad
	if r := checkCACert(cfg); r.Status != Fail {
		t.Errorf("invalid PEM should fail, got %v", r)
	}

	// A real cert (from an httptest TLS server) -> pass.
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	good := filepath.Join(dir, "good.crt")
	_ = os.WriteFile(good, certPEM, 0o644)
	cfg.CACertPath = good
	if r := checkCACert(cfg); r.Status != Pass {
		t.Errorf("valid PEM should pass, got %v", r)
	}
}

func TestCheckRegistryReachable(t *testing.T) {
	// TLS server that answers /v2/ with 200; TLS mode "none" skips verification.
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/" {
			w.WriteHeader(200)
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "https://") // host:port
	cfg := config.Default()
	cfg.TLSMode = config.TLSNone
	cfg.RegistryHost = host

	r := checkRegistry(cfg)
	if r.Status != Pass {
		t.Errorf("expected registry reachable, got status=%s detail=%s", r.Status, r.Detail)
	}
	if r.Field != "registryHost" {
		t.Errorf("registry check should map to registryHost, got %q", r.Field)
	}
}

func TestCheckRegistryUnreachable(t *testing.T) {
	cfg := config.Default()
	cfg.TLSMode = config.TLSNone
	cfg.RegistryHost = "127.0.0.1:1" // nothing listening
	if r := checkRegistry(cfg); r.Status != Fail {
		t.Errorf("unreachable registry should fail, got %v", r)
	}
}
