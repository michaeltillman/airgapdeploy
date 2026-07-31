# 06 — Verification Log

How each part was tested and the results observed. The project has automated
tests plus repeated manual/live verification.

## Standing gates (run on every change)
- `go build ./...` — compiles offline (vendored deps, embedded UI).
- `go vet ./...` — clean.
- `gofmt -l` (excluding vendor) — clean.
- `go test ./...` — all packages pass.

## Automated tests
`internal/config/config_test.go`
- Default config normalizes and validates.
- Derived values (`Registry`, `ChartsRepoURL`, `PrimaryK0sBinary`).
- `Validate` rejects bad input (versions, arch, host, URL, namespace).
- `Normalize` fills defaults / trims.
- TLS modes: none needs no CA; self-signed/custom-ca validate with defaults;
  unknown mode rejected; `FileserverIsHTTPS`.
- `ValidateAll` reports **all** bad fields at once (incl. skopeoVersion,
  nginxImage, storageClass, pvcSize, namespace, fileserverURL).

`internal/templates/templates_test.go`
- Every template renders against defaults with no `<no value>` (missingkey=error).
- Config substitution appears in rendered `03-values.yaml` / `01-prepare.sh`.
- `.sh` outputs marked executable.
- TLS-mode rendering: none → no cert secrets; custom+HTTP → registryCertSecret
  only; self-signed+HTTPS → both; push verifies vs CA in cert modes, insecure in
  none.
- Preflight/diagnostics artifacts present; DaemonSet is a DaemonSet; access
  check probes `/v2/`; diagnose detects ImagePullBackOff.
- ACME: artifacts emitted only in acme mode; issuer has server/email/http01;
  certificate has dnsNames from config; no registryCertSecret; push verifies.

`internal/checks/checks_test.go`
- `hostPort` parsing (with/without port, with path).
- `checkCACert`: missing file / garbage / real PEM (from an httptest TLS cert).
- `checkRegistry`: reachable (httptest TLS server, none mode) → pass, field maps
  to registryHost; unreachable → fail.
- `splitImageTag`; `checkOutputDir` writable vs non-creatable.

## Live / manual verification (representative results)
- **Headless generate:** `--generate` wrote the full artifact set (11 → 15 → 17
  files as features landed); correct exec bits; `bash -n` clean on all scripts;
  YAML validated with the vendored yaml.v3; `kubectl apply --dry-run=client`
  parsed manifests offline (`--validate=false` when no cluster reachable).
- **Live API smoke:** `/api/meta`, `/api/generate`, `/api/preflight` (correctly
  reported cosign/skopeo missing), `/api/file`, and **SSE `/api/run`** streaming
  real script output; path-traversal contained.
- **`check-access.sh` live run** caught two real bash bugs (empty-array under
  `set -u`; doubled HTTP code) — both fixed and re-verified.
- **Validate endpoint:** returned per-field reasons for every bad field
  (e.g. `"latest" is not a valid version`, `"ftp://x" must start with http…`,
  `"10gigs" is not a valid size`).
- **Check endpoint:** returned the real per-field target matrix — registry FAIL
  (DNS), **version PASS ("bundle for 1.4.0 exists upstream")**, **k0s PASS
  (amd64+arm64 confirmed upstream)**, fileserver WARN (not deployed), cluster
  FAIL (real kubectl session error), StorageClass distinguishes NotFound vs
  unreachable. Checks run concurrently (~5s measured vs ~20s sequential).
- **UI behavior (verified via browser + JS):** all 7 tabs navigate; ungenerated
  phases prompt; after generate, Preflight shows 3 files + Diagnostics shows
  diagnose.sh with Run buttons; TLS radios reveal/hide the CA and ACME field
  groups with correct hints; validation blocks Generate and highlights fields;
  brand chrome (gradient logo/nav/button, mint badge) confirmed at desktop width.

## Known caveats surfaced during verification
- Live checks reflect the **machine running the binary** — run on both the prep
  box and the cluster jump host (see [04-decisions](04-decisions.md) D10).
- Public Let's Encrypt is not usable in a true air-gap — the ACME artifacts and
  UI/runbook carry an explicit warning to use an internal ACME endpoint.
- Versions default to k0rdent `1.4.0` / k0s `v1.35.4+k0s.0`; confirm against
  `get.mirantis.com` (a doc sub-page showed a stale `v1.35.1`).
