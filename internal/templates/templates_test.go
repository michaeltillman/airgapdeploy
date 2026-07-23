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

func TestShScriptsMarkedExecutable(t *testing.T) {
	arts, _ := Render(config.Default())
	for _, a := range arts {
		wantExec := strings.HasSuffix(a.Path, ".sh")
		if a.Executable != wantExec {
			t.Errorf("%s: Executable=%v, want %v", a.Path, a.Executable, wantExec)
		}
	}
}
