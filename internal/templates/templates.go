// Package templates embeds the artifact templates and renders them against a
// Config. All templates live under assets/ and are embedded at build time so
// the final binary is fully self-contained (no runtime file dependencies).
package templates

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"strings"
	"text/template"

	"github.com/michaeltillman/airgapdeploy/internal/config"
)

//go:embed all:assets
var assetsFS embed.FS

// Artifact is a single rendered output file.
type Artifact struct {
	// Path is the output path relative to the artifacts root (e.g.
	// "02-fileserver/namespace-pvc.yaml").
	Path string
	// Content is the rendered file content.
	Content []byte
	// Executable indicates the file should be written with the exec bit set.
	Executable bool
}

// templatePaths is the ordered set of templates to render. Order controls the
// order artifacts are listed to the UI.
var templatePaths = []string{
	"assets/01-prepare.sh.tmpl",
	"assets/02-fileserver/namespace-pvc.yaml.tmpl",
	"assets/02-fileserver/binary-loader.yaml.tmpl",
	"assets/02-fileserver/configmap-nginx.yaml.tmpl",
	"assets/02-fileserver/deployment-service.yaml.tmpl",
	"assets/02-fileserver/load-k0s.sh.tmpl",
	"assets/03-values.yaml.tmpl",
	"assets/04-install.sh.tmpl",
	"assets/05-verify.sh.tmpl",
	"assets/check-access.sh.tmpl",
	"assets/check-node-pull/namespace-daemonset.yaml.tmpl",
	"assets/check-node-pull/run.sh.tmpl",
	"assets/diagnose.sh.tmpl",
	"assets/RUNBOOK.md.tmpl",
}

// outputPath maps an embedded template path to its output path: it strips the
// "assets/" prefix and the ".tmpl" suffix.
func outputPath(tmplPath string) string {
	p := strings.TrimPrefix(tmplPath, "assets/")
	return strings.TrimSuffix(p, ".tmpl")
}

// Render renders every artifact template against cfg and returns them in order.
// The config is normalized and validated before rendering.
func Render(cfg *config.Config) ([]Artifact, error) {
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	artifacts := make([]Artifact, 0, len(templatePaths))
	for _, tp := range templatePaths {
		raw, err := fs.ReadFile(assetsFS, tp)
		if err != nil {
			return nil, fmt.Errorf("reading template %s: %w", tp, err)
		}
		t, err := template.New(tp).Option("missingkey=error").Parse(string(raw))
		if err != nil {
			return nil, fmt.Errorf("parsing template %s: %w", tp, err)
		}
		var buf bytes.Buffer
		if err := t.Execute(&buf, cfg); err != nil {
			return nil, fmt.Errorf("rendering template %s: %w", tp, err)
		}
		out := outputPath(tp)
		artifacts = append(artifacts, Artifact{
			Path:       out,
			Content:    buf.Bytes(),
			Executable: strings.HasSuffix(out, ".sh"),
		})
	}
	return artifacts, nil
}
