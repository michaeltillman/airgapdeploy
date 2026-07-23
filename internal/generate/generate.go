// Package generate writes the full rendered artifact set to an output directory.
package generate

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/michaeltillman/airgapdeploy/internal/config"
	"github.com/michaeltillman/airgapdeploy/internal/templates"
)

// Result describes what was written.
type Result struct {
	OutputDir string   `json:"outputDir"`
	Files     []string `json:"files"`
}

// Run renders every artifact for cfg and writes it under cfg.OutputDir. It also
// writes the reloadable config.yaml snapshot. Existing files are overwritten.
func Run(cfg *config.Config) (*Result, error) {
	arts, err := templates.Render(cfg)
	if err != nil {
		return nil, err
	}

	root := cfg.OutputDir
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("creating output dir %s: %w", root, err)
	}

	res := &Result{OutputDir: root}

	// Write the reloadable config snapshot first.
	cfgPath := filepath.Join(root, "config.yaml")
	if err := cfg.Save(cfgPath); err != nil {
		return nil, fmt.Errorf("writing config.yaml: %w", err)
	}
	res.Files = append(res.Files, "config.yaml")

	for _, a := range arts {
		dest := filepath.Join(root, filepath.FromSlash(a.Path))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return nil, fmt.Errorf("creating dir for %s: %w", a.Path, err)
		}
		mode := os.FileMode(0o644)
		if a.Executable {
			mode = 0o755
		}
		if err := os.WriteFile(dest, a.Content, mode); err != nil {
			return nil, fmt.Errorf("writing %s: %w", a.Path, err)
		}
		res.Files = append(res.Files, a.Path)
	}
	return res, nil
}
