// Package runner handles optional live execution of generated scripts and the
// preflight checks that report which required CLIs are present.
package runner

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// Tool describes one CLI dependency and whether it was found.
type Tool struct {
	Name    string `json:"name"`
	Purpose string `json:"purpose"`
	// Side indicates where the tool is needed: "prep" (internet machine),
	// "cluster" (airgapped cluster side), or "both".
	Side    string `json:"side"`
	Found   bool   `json:"found"`
	Path    string `json:"path,omitempty"`
	Version string `json:"version,omitempty"`
}

// probe is the definition of a tool and how to read its version.
type probe struct {
	name        string
	purpose     string
	side        string
	versionArgs []string
}

var probes = []probe{
	{"bash", "Run the generated shell scripts", "both", []string{"--version"}},
	{"tar", "Extract the airgap bundle", "prep", []string{"--version"}},
	{"wget", "Download bundle and k0s binaries", "prep", []string{"--version"}},
	{"cosign", "Verify bundle/binary signatures", "prep", []string{"version"}},
	{"docker", "Run the bundled skopeo image", "prep", []string{"--version"}},
	{"skopeo", "Copy images/charts (optional; bundled as an image)", "prep", []string{"--version"}},
	{"helm", "Install the kcm chart", "cluster", []string{"version", "--short"}},
	{"kubectl", "Apply manifests and verify the cluster", "cluster", []string{"version", "--client", "--output=yaml"}},
}

// Preflight probes every known CLI and returns their status.
func Preflight() []Tool {
	out := make([]Tool, 0, len(probes))
	for _, p := range probes {
		t := Tool{Name: p.name, Purpose: p.purpose, Side: p.side}
		if path, err := exec.LookPath(p.name); err == nil {
			t.Found = true
			t.Path = path
			t.Version = firstLine(runVersion(p.name, p.versionArgs))
		}
		out = append(out, t)
	}
	return out
}

func runVersion(name string, args []string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	b, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	return string(b)
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}
