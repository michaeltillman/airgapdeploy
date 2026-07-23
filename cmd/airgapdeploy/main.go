// Command airgapdeploy serves a guided web UI that generates (and optionally
// runs) the artifacts for an airgapped k0rdent Enterprise installation.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/michaeltillman/airgapdeploy/internal/config"
	"github.com/michaeltillman/airgapdeploy/internal/generate"
	"github.com/michaeltillman/airgapdeploy/internal/server"
)

func main() {
	var (
		addr        = flag.String("addr", "127.0.0.1:8080", "address to listen on")
		out         = flag.String("out", config.DefaultOutputDir, "directory for generated artifacts")
		live        = flag.Bool("live", false, "enable live execution of generated scripts")
		cfgPath     = flag.String("config", "", "load initial config from a YAML file")
		genOnly     = flag.Bool("generate", false, "generate artifacts from --config and exit (no server)")
		showVersion = flag.Bool("version", false, "print version and exit")
	)
	flag.Parse()

	if *showVersion {
		fmt.Println("airgapdeploy 0.1.0")
		return
	}

	cfg := config.Default()
	if *cfgPath != "" {
		loaded, err := config.Load(*cfgPath)
		if err != nil {
			log.Fatalf("loading config: %v", err)
		}
		cfg = loaded
	}
	if *out != "" {
		cfg.OutputDir = *out
	}
	cfg.Normalize()

	// Headless generation mode: render artifacts and exit.
	if *genOnly {
		res, err := generate.Run(cfg)
		if err != nil {
			log.Fatalf("generate: %v", err)
		}
		fmt.Printf("Wrote %d files to %s/\n", len(res.Files), res.OutputDir)
		for _, f := range res.Files {
			fmt.Println("  -", f)
		}
		return
	}

	srv := server.New(cfg, *live)
	httpSrv := &http.Server{
		Addr:              *addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	mode := "generator (read-only)"
	if *live {
		mode = "LIVE — scripts can be executed"
	}
	log.Printf("airgapdeploy listening on http://%s  [%s]", *addr, mode)
	log.Printf("artifacts output directory: %s", cfg.OutputDir)

	// Graceful shutdown on SIGINT/SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down…")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)
}
