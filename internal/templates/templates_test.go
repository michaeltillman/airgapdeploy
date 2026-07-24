package templates

import (
	"strings"
	"testing"

	"github.com/michaeltillman/airgapdeploy/internal/config"
)

// TestRenderAll ensures every template parses and renders against the default
// config. missingkey=error means an undefined field reference fails here,
// catching template/config drift.
func TestRenderAll(t *testing.T) {
	arts, err := Render(config.Default())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if len(arts) != len(templatePaths) {
		t.Fatalf("expected %d artifacts, got %d", len(templatePaths), len(arts))
	}
	for _, a := range arts {
		if len(a.Content) == 0 {
			t.Errorf("%s rendered empty", a.Path)
		}
		if strings.Contains(string(a.Content), "<no value>") {
			t.Errorf("%s contains an unresolved template value", a.Path)
		}
	}
}

func TestRenderSubstitutesConfig(t *testing.T) {
	c := config.Default()
	c.RegistryHost = "harbor.corp"
	c.RegistryProject = "kre"
	c.FileserverURL = "http://files.corp/kre"
	arts, err := Render(c)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	byPath := map[string]string{}
	for _, a := range arts {
		byPath[a.Path] = string(a.Content)
	}
	values := byPath["03-values.yaml"]
	if !strings.Contains(values, "oci://harbor.corp/kre/charts") {
		t.Errorf("values missing templatesRepoURL substitution:\n%s", values)
	}
	if !strings.Contains(values, `globalK0sURL: "http://files.corp/kre"`) {
		t.Errorf("values missing globalK0sURL substitution")
	}
	if exec := byPath["01-prepare.sh"]; !strings.Contains(exec, "harbor.corp") {
		t.Errorf("prepare.sh missing registry host")
	}
}

func TestTLSModeRendering(t *testing.T) {
	render := func(mode, fsURL string) map[string]string {
		c := config.Default()
		c.TLSMode = mode
		c.FileserverURL = fsURL
		arts, err := Render(c)
		if err != nil {
			t.Fatalf("Render(%s): %v", mode, err)
		}
		m := map[string]string{}
		for _, a := range arts {
			m[a.Path] = string(a.Content)
		}
		return m
	}

	// none: no cert secrets wired anywhere.
	none := render(config.TLSNone, "http://files.local/kre")
	if strings.Contains(none["03-values.yaml"], "registryCertSecret") {
		t.Error("none mode should not emit registryCertSecret")
	}
	if strings.Contains(none["04-install.sh"], "create secret") {
		t.Error("none mode install script should not create a secret")
	}

	// custom-ca + HTTP fileserver: registry secret only, no k0sURLCertSecret.
	http := render(config.TLSCustomCA, "http://files.local/kre")
	if !strings.Contains(http["03-values.yaml"], "registryCertSecret:") {
		t.Error("custom-ca should emit registryCertSecret")
	}
	if strings.Contains(http["03-values.yaml"], "k0sURLCertSecret:") {
		t.Error("HTTP fileserver should not emit the k0sURLCertSecret key")
	}
	if !strings.Contains(http["04-install.sh"], "kubectl create secret generic") {
		t.Error("custom-ca install script should create the CA secret")
	}

	// self-signed + HTTPS fileserver: both cert secrets wired.
	https := render(config.TLSSelfSigned, "https://files.local/kre")
	if !strings.Contains(https["03-values.yaml"], "k0sURLCertSecret:") {
		t.Error("HTTPS fileserver should emit k0sURLCertSecret")
	}

	// Prep-side push: cert modes verify against the CA; none disables verify.
	if !strings.Contains(none["01-prepare.sh"], "--dest-tls-verify=false") {
		t.Error("none mode push should disable TLS verification")
	}
	if strings.Contains(none["01-prepare.sh"], "--dest-cert-dir") {
		t.Error("none mode push should not use --dest-cert-dir")
	}
	prep := http["01-prepare.sh"]
	if !strings.Contains(prep, "--dest-tls-verify=true") || !strings.Contains(prep, "--dest-cert-dir /certs") {
		t.Error("cert mode push should verify against the CA via --dest-cert-dir")
	}
	if strings.Contains(prep, "--dest-tls-verify=false") {
		t.Error("cert mode push should not disable TLS verification")
	}
}

func TestPreflightAndDiagnosticsArtifacts(t *testing.T) {
	c := config.Default()
	c.RegistryHost = "harbor.corp"
	arts, err := Render(c)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	byPath := map[string]string{}
	for _, a := range arts {
		byPath[a.Path] = string(a.Content)
	}
	for _, p := range []string{
		"check-access.sh",
		"check-node-pull/namespace-daemonset.yaml",
		"check-node-pull/run.sh",
		"diagnose.sh",
	} {
		if _, ok := byPath[p]; !ok {
			t.Errorf("missing generated artifact %s", p)
		}
	}
	if !strings.Contains(byPath["check-node-pull/namespace-daemonset.yaml"], "kind: DaemonSet") {
		t.Error("node pull test should be a DaemonSet (per-node coverage)")
	}
	if !strings.Contains(byPath["check-node-pull/namespace-daemonset.yaml"], "harbor.corp") {
		t.Error("node pull test should reference the configured registry")
	}
	if !strings.Contains(byPath["check-access.sh"], "/v2/") {
		t.Error("access check should probe the registry v2 API")
	}
	if !strings.Contains(byPath["diagnose.sh"], "ImagePullBackOff") {
		t.Error("diagnose should detect image pull failures")
	}
}

func TestAccessCheckTLSBranching(t *testing.T) {
	custom := config.Default()
	custom.TLSMode = config.TLSCustomCA
	cArts, _ := Render(custom)
	var access string
	for _, a := range cArts {
		if a.Path == "check-access.sh" {
			access = string(a.Content)
		}
	}
	if !strings.Contains(access, "--cacert") || !strings.Contains(access, "openssl s_client") {
		t.Error("cert mode access check should verify TLS against the CA")
	}

	none := config.Default()
	nArts, _ := Render(none)
	for _, a := range nArts {
		if a.Path == "check-access.sh" && !strings.Contains(string(a.Content), "-k") {
			t.Error("none mode access check should probe insecurely (-k)")
		}
	}
}

func TestShScriptsMarkedExecutable(t *testing.T) {
	arts, _ := Render(config.Default())
	for _, a := range arts {
		wantExec := strings.HasSuffix(a.Path, ".sh")
		if a.Executable != wantExec {
			t.Errorf("%s: Executable=%v, want %v", a.Path, a.Executable, wantExec)
		}
	}
}
