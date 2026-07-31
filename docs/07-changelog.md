# 07 — Changelog (commit by commit)

Chronological, oldest first. Hashes are on `main` at
<https://github.com/michaeltillman/airgapdeploy>.

| Date | Hash | Summary |
|------|------|---------|
| 2026-07-23 | `699316d` | Initial commit (README stub) |
| 2026-07-23 | `d86d9de` | Add airgapdeploy: guided installer UI for airgapped k0rdent Enterprise |
| 2026-07-24 | `a3f228d` | Add selectable TLS mode: none, self-signed, or custom CA |
| 2026-07-24 | `84ca6d5` | Verify prep-side skopeo push against the CA in cert modes |
| 2026-07-24 | `f9f08a8` | Add registry/node access validation and pull/install diagnostics |
| 2026-07-24 | `9d9d127` | Make wizard tabs navigable anytime; add missing preflight/diagnostics panels |
| 2026-07-29 | `5144af3` | Rebrand UI to the k0rdent design system |
| 2026-07-29 | `ac8ce64` | Rename UI heading to "k0rdent AI Enterprise Air-Gap Deployment" |
| 2026-07-31 | `8d8c4cb` | Add Configure-page validation and live target-access checks |
| 2026-07-31 | `ffb6ff7` | Add automated ACME (Let's Encrypt) TLS mode |
| 2026-07-31 | `a911e54` | Validate and verify every Configure field |

## Details

### d86d9de — Initial implementation (45 files, +13,916)
Full app: config model, artifact templates (`01-prepare` … `RUNBOOK`),
templates/generate packages, runner (preflight + SSE), server + embedded web UI,
main.go, Makefile, README, vendored yaml.v3. Non-authenticated registry flow.

### a3f228d — Selectable TLS mode (9 files)
`selfSignedCA` bool → `TLSMode` (none/self-signed/custom-ca) + `CACertPath`.
Values/install/runbook branch on mode; UI radio + revealed CA fields;
`k0sURLCertSecret` only when fileserver is HTTPS. Tests for all modes.

### 84ca6d5 — Verified prep push (3 files)
Cert modes stage the CA and push with `--dest-tls-verify=true --dest-cert-dir`;
none keeps insecure. Runbook + test updated.

### f9f08a8 — Access validation + diagnostics (8 files)
`check-access.sh`, `check-node-pull/` (per-node DaemonSet + runner), `diagnose.sh`;
Preflight + Diagnostics UI phases; runbook validation/troubleshooting + node-CA
note. (Fixed two bash bugs found by running the script.)

### 9d9d127 — Navigable tabs + panel fix (3 files)
Unlock nav with a "generate first" prompt; add the missing Preflight/Diagnostics
`<section>` panels (latent bug from f9f08a8).

### 5144af3 — k0rdent rebrand (3 files)
Logo mark (gradient), `#1AAAFF` + `#1DA7FD→#08F5DA` gradient, dark chrome, mint
accents, favicon. Matched the public k0rdent brand (private k0rdent-ui
inaccessible).

### ac8ce64 — Heading rename (1 file)
Top-bar `<h1>` + tab title → "k0rdent AI Enterprise Air-Gap Deployment".

### 8d8c4cb — Configure validation + access checks (10 files)
`config.ValidateAll` (per-field); `internal/checks` (registry/version/fileserver/
cluster/CA); `/api/validate` + `/api/check`; UI inline field errors + "Validate &
test access" panel; concurrent checks. Tests + README note.

### ffb6ff7 — ACME TLS mode (11 files)
`TLSMode acme` + ACMEServer/Email/IngressClass + TLSDomains; `tls-acme/`
cert-manager artifacts (acme-only); public-root verification for push/checks;
"ACME server reachable" check; UI + runbook with air-gap warning; tests.

### a911e54 — Validate/verify every field (4 files)
Static checks for the remaining fields; live checks for k0s binaries (per arch),
nginx/busybox exact tags, StorageClass, IngressClass, output-dir writability;
NotFound-vs-unreachable distinction. Tests.

### (this doc set) — Documentation library
`docs/` — overview, architecture, build history, decisions, features,
verification, changelog, delivery & usage.
