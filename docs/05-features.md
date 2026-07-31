# 05 — Feature Catalog

Every capability, config field, generated artifact, and check.

## Wizard phases (UI)
`Configure → Prep machine → Fileserver → Preflight → Install → Verify →
Diagnostics → Runbook`. Tabs are navigable at any time; ungenerated phases show a
"generate first" prompt. Live-mode phases add **Run** buttons that stream output.

## Configure fields — validation & verification coverage
Every field is statically validated. Fields with a real target are also verified
live via **Validate & test access**. (Full matrix in
[06-verification](06-verification.md).)

| Field | Static check | Live verification |
|-------|--------------|-------------------|
| kcmVersion | semantic version | bundle exists upstream |
| k0sVersion | `v…` format | each k0s binary exists upstream (per arch) |
| skopeoVersion | `v…` format | — |
| architectures | amd64/arm64 | via k0s-binary check |
| registryHost | host[:port] | registry TCP + TLS trust + `/v2/` |
| registryProject | non-empty | via image checks |
| fileserverURL | http(s) URL | fileserver serves the k0s binary |
| fileserverNamespace | DNS label | — |
| storageClass | name format | StorageClass exists (or a default exists) |
| pvcSize | quantity (`10Gi`) | — |
| nginxImage | `name:tag` | exact tag present in registry |
| busyboxImage | `name:tag` | exact tag present in registry |
| namespace | DNS label | — |
| bundleBaseURL | http(s) URL | via version/k0s HEAD |
| outputDir | non-empty | directory is writable |
| tlsMode | enum (none/self-signed/custom-ca/acme) | — |
| caCertPath | required in cert modes | file is a valid PEM |
| caCertSecretName | DNS label | — |
| acmeServer | URL | ACME directory reachable |
| acmeEmail | email format | — |
| acmeIngressClass | name format | IngressClass exists |

## TLS / certificate modes
- **none** — plain HTTP or publicly-trusted; no CA secret; push skips TLS verify.
- **self-signed** — operator supplies the registry's self-signed cert; secret
  created; `registryCertSecret` wired; push verifies against it.
- **custom-ca** — operator supplies the internal CA bundle; same plumbing as
  self-signed.
- **acme** — cert-manager `ClusterIssuer` + `Certificate` (HTTP-01 ingress
  solver) automate issuance/renewal; certs assumed publicly trusted (no CA
  secret); push verifies against public roots. Explicit air-gap warning (use an
  internal ACME endpoint for a true air-gap).

## Generated artifacts
- `config.yaml` — reloadable snapshot of inputs.
- `01-prepare.sh` — prep machine: download+cosign-verify bundle, `docker load`
  skopeo, `skopeo copy` images/charts (TLS behavior per mode), fetch+verify k0s
  binaries.
- `02-fileserver/` — `namespace-pvc.yaml`, `binary-loader.yaml`,
  `configmap-nginx.yaml`, `deployment-service.yaml`, `load-k0s.sh`.
- `03-values.yaml` — kcm Helm values: `templatesRepoURL`, `globalRegistry`,
  `globalK0sURL`, component overrides (flux2, cert-manager, cluster-api-operator,
  velero, rbac-manager, k0rdent-ui), cert secrets per mode.
- `04-install.sh` — `helm install kcm …` (+ CA secret creation in cert modes).
- `05-verify.sh` — pod counts (~30 in kcm-system, ~10 in projectsveltos),
  Management READY, provider/cluster templates VALID.
- `check-access.sh` — registry reachability/TLS/`/v2/`/image presence + fileserver.
- `check-node-pull/` — DaemonSet that pulls a canary image on EVERY node +
  `run.sh` that reports per-node failures with events.
- `diagnose.sh` — scan all namespaces for ImagePull/CrashLoop, print offending
  image+node+events, helm status, Management/template readiness, warning events,
  likely causes.
- `tls-acme/` (acme only) — `clusterissuer.yaml`, `certificate.yaml`.
- `RUNBOOK.md` — human runbook for both sides, TLS-mode-aware, with air-gap
  notes and node-CA-trust guidance.

## Live target-access checks (`internal/checks`)
Concurrent probes, each returning `pass|fail|warn|skip` + a concrete reason and
the field to highlight:
`outputDir writable` · `CA cert file` (cert modes) · `ACME server reachable` +
`ingress class exists` (acme) · `registry reachable & trusted` · `nginx image
present` · `busybox image present` · `version bundle available` · `k0s binaries
available` · `k0s fileserver reachable` · `StorageClass available` ·
`Kubernetes cluster access`.

## Preflight CLI detection (`internal/runner`)
Detects presence + version of `bash`, `tar`, `wget`, `cosign`, `docker`,
`skopeo` (prep) and `helm`, `kubectl` (cluster), labeled by which side needs them.

## API surface (`internal/server`)
`GET /api/meta` · `GET|POST /api/config` · `POST /api/validate` ·
`POST /api/check` · `POST /api/generate` · `GET /api/file` · `GET /api/preflight`
· `POST /api/run` (SSE, `--live` only).

## CLI flags
`--addr` (default `127.0.0.1:8080`) · `--out` (default `artifacts`) · `--live` ·
`--config <yaml>` · `--generate` (headless, no server) · `--version`.

## Branding
Official k0rdent brand: logo mark (gradient), primary `#1AAAFF`, gradient
`#1DA7FD → #08F5DA`, dark chrome, mint accents, SVG favicon. Theme-aware
(light/dark). Heading: "k0rdent AI Enterprise Air-Gap Deployment".
