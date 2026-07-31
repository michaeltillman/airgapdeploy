# 08 — Delivery & Usage

## Delivering to a customer

### Model A (recommended) — prebuilt binary
The whole UI is embedded, so the tool compiles to one self-contained executable.
```bash
make dist
# -> dist/airgapdeploy-linux-amd64   (usually this one)
#    dist/airgapdeploy-linux-arm64
#    dist/airgapdeploy-darwin-amd64
#    dist/airgapdeploy-darwin-arm64
```
Hand over the single binary matching the operator's OS/arch. Because the target
is air-gapped, carry it in on media/internal transfer (the customer can't pull
from GitHub on the isolated box).

### Model B — hand over the repo
For customers who want to audit/build it themselves. Dependencies are vendored,
so it builds **offline** with only Go:
```bash
git clone git@github.com:michaeltillman/airgapdeploy.git && cd airgapdeploy
make build      # -> bin/airgapdeploy   (go build ./... also works offline)
```

## Running the UI
```bash
./airgapdeploy                 # generator mode, http://127.0.0.1:8080
./airgapdeploy --live          # enable Run buttons + live access checks
./airgapdeploy --out ./out     # choose the artifacts output directory
```
- Binds to **localhost only** by design. On a remote/jump host, open it over an
  SSH tunnel rather than exposing it:
  ```bash
  ssh -L 8080:localhost:8080 user@operator-box
  ```
  then browse `http://localhost:8080`.
- **Generator mode** (default): safe anywhere; produces artifacts you run.
- **Live mode** (`--live`): adds Run buttons that execute the generated `.sh`
  scripts on that machine and stream output, plus live target-access checks.

### Headless generation (no UI)
```bash
./airgapdeploy --generate --config my-config.yaml --out ./out
```

## The wizard flow
1. **Configure** — fill fields; click **Validate & test access** to check every
   field and probe the targets; then **Generate artifacts**.
2. **Prep machine** — run `01-prepare.sh` on the internet-connected box.
3. **Preflight** — `check-access.sh` + `check-node-pull/run.sh` to prove
   registry + every node can pull.
4. **Install** — review `03-values.yaml`, run `04-install.sh`.
5. **Verify** — `05-verify.sh` after 5–10 min.
6. **Diagnostics** — `diagnose.sh` if anything fails.
7. **Runbook** — the generated `RUNBOOK.md` ties it all together.

The generated `artifacts/` folder is plain scripts + YAML — fully portable, so
the UI is a convenience, not a requirement.

## The two-machine reality
| Phase | Where | Needs |
|-------|-------|-------|
| Prep (`01-prepare.sh`) | machine with **internet + registry** access | docker, cosign, wget, tar |
| Fileserver → Verify | machine with **cluster** access (inside air-gap) | kubectl, helm ≥3.16.3 |

Run **Validate & test access** on *each* machine — the checks reflect that
machine's reachability. The Preflight panel's CLI detection shows what's missing.

## Operator prerequisites (not bundled)
- **Prep machine:** bash ≥4.2, GNU coreutils, tar ≥1.34, wget, docker (runs the
  bundled skopeo image), cosign ≥2.4.1.
- **Cluster side:** kubectl + helm ≥3.16.3 against a management cluster
  (Kubernetes ≥1.30) with a StorageClass and a LoadBalancer provider.

## Local dev / operational notes
- The dev server runs as a foreground/background process; rebuilding kills it —
  restart with the run command above. (A `make serve` target can be added.)
- Versions default to k0rdent `1.4.0` / k0s `v1.35.4+k0s.0`; confirm against
  `get.mirantis.com` before a real run.
- ACME mode in a true air-gap: point `acmeServer` at an internal ACME endpoint
  (step-ca / Vault / Windows CA), not public Let's Encrypt.
