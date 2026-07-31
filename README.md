# airgapdeploy

A single-binary, self-contained tool that walks an operator through an
**airgapped [k0rdent Enterprise](https://docs.mirantis.com/k0rdent-enterprise/)
installation**. It serves a guided web UI that:

1. Collects your environment settings (registry, k0s fileserver, versions).
2. **Generates** the complete, readable artifact set — prep script, Kubernetes
   manifests, Helm values, install/verify scripts, and a runbook.
3. Optionally **runs** those same scripts for you (with `--live`), streaming logs
   live.

It follows the Mirantis airgap procedure:
<https://docs.mirantis.com/k0rdent-enterprise/latest/admin/installation/airgap/airgap-install/>

📚 **Full documentation library:** [`docs/`](docs/README.md) — overview,
architecture, complete build history, decision log, feature catalog, verification
log, changelog, and delivery/usage guide.

The generator is the source of truth — every "Run" button in live mode executes
a file you can read first. Nothing is hidden.

> Scope: this release targets the **non-authenticated registry** flow (the
> v1.4.0 default). Authenticated and pull-through registries are not yet
> generated. Defaults target k0rdent Enterprise `1.4.0` / k0s `v1.35.4+k0s.0` —
> confirm exact versions against `get.mirantis.com` before a real run.

## Why single-binary

Airgap means no internet, no CDN, and often a single hand-off. `airgapdeploy`
is one statically-linked Go binary with the entire web UI embedded. Copy it in,
run it, done — no Node, no runtime dependencies, no network calls.

## Build

Requires Go ≥ 1.18. The frontend has **no build step** (hand-written HTML/CSS/JS
embedded via `go:embed`), so `go build` alone produces the binary.

```bash
make build          # -> bin/airgapdeploy
# or cross-compile release binaries:
make dist           # -> dist/airgapdeploy-{linux,darwin}-{amd64,arm64}
```

Dependencies are vendored (`vendor/`), so the repo builds **offline**:

```bash
go build ./...      # works with no network access
```

## Run

```bash
./bin/airgapdeploy                 # generator mode, http://127.0.0.1:8080
./bin/airgapdeploy --live          # enable script execution on this machine
./bin/airgapdeploy --out ./out     # choose the artifacts output directory
```

Then open the printed URL and fill in the **Configure** screen. Use
**Validate & test access** there to check every field for errors *and* actively
probe the targets from this machine — registry reachability + TLS trust against
your CA, whether the specified version's bundle exists upstream, the k0s
fileserver, and Kubernetes cluster access — with a per-field reason for anything
that fails.

- **Generator mode** (default): safe to run anywhere. Produces artifacts you run
  yourself. Ideal when handing the repo to a customer.
- **Live mode** (`--live`): adds Run buttons that execute the generated `.sh`
  scripts on the machine running the binary, streaming output to the UI. Run the
  binary in `--live` on the prep machine (for `01-prepare.sh`) and on a machine
  with cluster access (for the fileserver/install/verify scripts).

### Headless generation (no UI)

```bash
./bin/airgapdeploy --generate --config my-config.yaml --out ./out
```

## What it generates

| Artifact | Side | Purpose |
|----------|------|---------|
| `config.yaml` | — | Reloadable snapshot of your inputs |
| `01-prepare.sh` | prep (internet) | Download + cosign-verify bundle, skopeo-push images/charts, fetch k0s binaries |
| `02-fileserver/*.yaml` + `load-k0s.sh` | cluster | nginx HTTP fileserver for the k0s binary |
| `03-values.yaml` | cluster | kcm Helm values pointing at your registry + fileserver |
| `04-install.sh` | cluster | `helm install kcm …` |
| `05-verify.sh` | cluster | Pod / Management / template readiness checks |
| `RUNBOOK.md` | — | Human runbook tying both sides together |

## Operator prerequisites

`airgapdeploy` does not bundle third-party CLIs. The operator machine(s) supply:

- **Prep machine:** bash ≥4.2, GNU coreutils, tar ≥1.34, wget, docker (runs the
  bundled skopeo image), cosign ≥2.4.1.
- **Cluster side:** kubectl + helm ≥3.16.3 against a management cluster
  (Kubernetes ≥1.30) with a `StorageClass` and a LoadBalancer provider.

Live mode's **Preflight** panel reports which of these are present.

## Layout

```
cmd/airgapdeploy      entrypoint + flags
internal/config       Config model, defaults, validation, YAML load/save
internal/templates    embedded artifact templates (assets/) + renderer
internal/generate     writes the artifact set to --out
internal/runner       preflight CLI checks + live SSE execution
internal/server       HTTP API + embedded web UI (web/)
```
