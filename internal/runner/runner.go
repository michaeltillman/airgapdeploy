package runner

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// Runner executes generated scripts confined to a single output directory and
// streams their combined output line by line. It is only wired up when the
// server is started with --live.
type Runner struct {
	root string

	mu      sync.Mutex
	running map[string]context.CancelFunc
}

// New returns a Runner confined to the given output directory.
func New(outputDir string) *Runner {
	return &Runner{root: outputDir, running: map[string]context.CancelFunc{}}
}

// Line is a single streamed output line.
type Line struct {
	Text string
	// Done is true for the terminal line; Err is set if the script failed.
	Done bool
	Err  string
}

// resolve validates that name refers to an executable script inside root and
// returns its absolute path. It rejects path traversal.
func (r *Runner) resolve(name string) (string, error) {
	clean := filepath.Clean("/" + filepath.FromSlash(name))[1:] // strip leading sep, kill ..
	if clean == "" || strings.HasPrefix(clean, "..") {
		return "", fmt.Errorf("invalid script name %q", name)
	}
	abs := filepath.Join(r.root, clean)
	rootAbs, err := filepath.Abs(r.root)
	if err != nil {
		return "", err
	}
	absAbs, err := filepath.Abs(abs)
	if err != nil {
		return "", err
	}
	if absAbs != rootAbs && !strings.HasPrefix(absAbs, rootAbs+string(os.PathSeparator)) {
		return "", fmt.Errorf("script %q is outside the output directory", name)
	}
	if !strings.HasSuffix(absAbs, ".sh") {
		return "", fmt.Errorf("only .sh scripts may be run")
	}
	info, err := os.Stat(absAbs)
	if err != nil {
		return "", fmt.Errorf("script %q not found (generate artifacts first)", name)
	}
	if info.IsDir() {
		return "", fmt.Errorf("%q is a directory", name)
	}
	return absAbs, nil
}

// Run executes the named script under `bash`, sending each output line to out.
// It blocks until the script exits or ctx is cancelled, then closes out.
func (r *Runner) Run(ctx context.Context, name string, out chan<- Line) {
	defer close(out)

	abs, err := r.resolve(name)
	if err != nil {
		out <- Line{Done: true, Err: err.Error()}
		return
	}

	ctx, cancel := context.WithCancel(ctx)
	r.mu.Lock()
	r.running[name] = cancel
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.running, name)
		r.mu.Unlock()
		cancel()
	}()

	cmd := exec.CommandContext(ctx, "bash", abs)
	cmd.Dir = filepath.Dir(abs)
	cmd.Env = os.Environ()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		out <- Line{Done: true, Err: err.Error()}
		return
	}
	cmd.Stderr = cmd.Stdout // interleave stderr into the same stream

	if err := cmd.Start(); err != nil {
		out <- Line{Done: true, Err: err.Error()}
		return
	}

	scan := bufio.NewScanner(stdout)
	scan.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	stream(scan, out)

	waitErr := cmd.Wait()
	if waitErr != nil {
		out <- Line{Done: true, Err: fmt.Sprintf("script exited: %v", waitErr)}
		return
	}
	out <- Line{Done: true}
}

func stream(scan *bufio.Scanner, out chan<- Line) {
	for scan.Scan() {
		out <- Line{Text: scan.Text()}
	}
}

// Stop cancels a running script by name, if present.
func (r *Runner) Stop(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if cancel, ok := r.running[name]; ok {
		cancel()
		return true
	}
	return false
}
