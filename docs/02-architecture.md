# 02 — Architecture

## Shape
One statically-linked Go binary that:
1. Serves a hand-written, embedded web UI (no build step; `go:embed`).
2. Exposes a small JSON/SSE API.
3. Renders a set of **artifact templates** from a single `Config` into an output
   directory.
4. In `--live` mode, executes the generated `.sh` scripts and streams output,
   and runs live target-access checks.

Because the frontend is embedded and the one dependency (`gopkg.in/yaml.v3`) is
vendored, `go build ./...` produces the binary with **no network access** — the
property that makes it viable inside an air-gap and auditable by a customer.

## Repository layout
```
airgapdeploy/
  go.mod                       # module + single dep (yaml.v3), go 1.18
  Makefile                     # build / dist (cross-compile) / run / test
  README.md                    # quick start
  cmd/airgapdeploy/main.go     # flags (--addr --out --live --config --generate --version), server lifecycle
  internal/
    config/config.go           # Config struct: defaults, Normalize, Validate, ValidateAll, helpers
    templates/
      templates.go             # go:embed assets, render + conditional skip (tls-acme only in acme mode)
      assets/                   # the artifact templates (see below)
    generate/generate.go       # write the rendered artifact set + config.yaml to --out
    checks/checks.go           # live target-access probes (registry/version/k0s/fileserver/cluster/storageclass/ingressclass/acme/images/outputdir/cacert)
    runner/
      preflight.go             # detect CLI tools + versions (bash/tar/wget/cosign/docker/skopeo/helm/kubectl)
      runner.go                # exec a generated script, stream stdout/stderr, path-traversal safe
    server/
      server.go                # routes + embed web/
      api.go                   # /api/{meta,config,validate,check,generate,file,preflight,run}
      web/                     # index.html, styles.css, app.js, favicon.svg (embedded, no build)
  docs/                        # this documentation library
  vendor/                      # gopkg.in/yaml.v3 (offline builds)
```

## The `assets/` artifact templates
Rendered from `Config` via Go `text/template` (with `missingkey=error`, so a
template referencing an undefined field fails at render — catching drift).

| Template | Output | Side | Role |
|----------|--------|------|------|
| `01-prepare.sh.tmpl` | `01-prepare.sh` | prep (internet) | download+verify bundle, skopeo-push images/charts, fetch+verify k0s binaries |
| `02-fileserver/*.tmpl` | `02-fileserver/*.yaml`, `load-k0s.sh` | cluster | nginx HTTP fileserver for the k0s binary |
| `03-values.yaml.tmpl` | `03-values.yaml` | cluster | kcm Helm values (registry + fileserver + component overrides + cert secrets) |
| `04-install.sh.tmpl` | `04-install.sh` | cluster | `helm install kcm …` (+ CA secret creation in cert modes) |
| `05-verify.sh.tmpl` | `05-verify.sh` | cluster | pod counts, Management ready, templates valid |
| `check-access.sh.tmpl` | `check-access.sh` | any | registry reachability/TLS/`/v2/`/images + fileserver |
| `check-node-pull/*.tmpl` | DaemonSet + `run.sh` | cluster | prove EVERY node can pull from the registry |
| `diagnose.sh.tmpl` | `diagnose.sh` | cluster | surface ImagePull/CrashLoop + events + helm/template status |
| `tls-acme/*.tmpl` | ClusterIssuer + Certificate | cluster | **acme mode only** — cert-manager automated issuance |
| `RUNBOOK.md.tmpl` | `RUNBOOK.md` | — | human runbook tying both sides together |

`config.yaml` (a reloadable snapshot of the inputs) is also written.

## The `Config` model
A single struct is the source of truth for every template and every check. Key
fields: versions (`KcmVersion`, `K0sVersion`, `SkopeoVersion`, `Architectures`),
registry (`RegistryHost`, `RegistryProject`), fileserver (`FileserverURL`,
`FileserverNamespace`, `StorageClass`, `PVCSize`, `NginxImage`, `BusyboxImage`),
install (`Namespace`), sources (`BundleBaseURL`, `OutputDir`), and TLS
(`TLSMode`, `CACertPath`, `CACertSecretName`, `ACMEServer`, `ACMEEmail`,
`ACMEIngressClass`). Helper methods derive `Registry()`, `ChartsRepoURL()`,
`BundleName()`, `K0sBinaryName()`, `TLSDomains()`, `UsesCustomTLS()`,
`UsesACME()`, `FileserverIsHTTPS()`, etc.

Lifecycle: `Default()` → (fill from form/JSON) → `Normalize()` (trim, backfill
optional defaults) → `Validate()` (first error, CLI) / `ValidateAll()` (all
errors keyed by field, UI).

## Request/data flow
```
Browser (Configure form)
   │  POST /api/validate  → config.ValidateAll()      → {field: reason}
   │  POST /api/check     → checks.Run(cfg)           → [{status,label,field,detail}]  (concurrent probes)
   │  POST /api/generate  → generate.Run(cfg)         → writes artifacts to --out
   │  GET  /api/file      → read a generated file       (path-traversal safe)
   │  GET  /api/preflight → runner.Preflight()        → CLI presence/versions
   │  POST /api/run (SSE) → runner.Runner.Run()       → stream a generated script   [--live only]
   ▼
Embedded web UI renders results; live logs stream over Server-Sent Events.
```

## Safety model
- **Generator is the source of truth.** Every "Run" executes a file the operator
  can read first — no hidden execution path.
- **Live execution gated on `--live`.** `/api/run` returns 403 otherwise.
- **Path-traversal safe.** `/api/file` and the runner confine access to the
  output directory and to `.sh` scripts.
- **Read-only checks.** Validation and access probes make no changes (they only
  read registry APIs, HEAD URLs, and run read-only `kubectl get`).
- **Localhost bind by default** (`127.0.0.1:8080`); remote use is via SSH tunnel.

## Notable design choices
- **No frontend build step** — hand-written HTML/CSS/vanilla-JS embedded, so the
  repo builds with only Go, offline.
- **Concurrent checks** — `checks.Run` fans out all probes with a `WaitGroup`
  and collects results in stable order (~5–8s wall time instead of ~20s).
- **Conditional artifacts** — the cert-manager `tls-acme/` files render only in
  `acme` mode (filtered in `templates.Render`).
