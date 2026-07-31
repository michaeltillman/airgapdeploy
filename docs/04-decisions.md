# 04 — Decision Log

Every explicit decision made during the build, the options weighed, the choice,
and why. Decisions marked **(user)** were chosen by the user via a presented
question; others are engineering defaults chosen and stated.

## D1 — Execution model **(user)**
- **Options:** (a) Live orchestrator that runs everything; (b) Generator +
  guided runbook; (c) Hybrid — generator-first with an optional local run mode.
- **Chosen:** **(c) Hybrid.** Static/offline generator by default, upgrades to
  live execution when started with `--live`.
- **Why:** Maximally shippable (safe to hand a customer, nothing to trust-run by
  default) while still offering real automation on the operator's machine. The
  generator is the source of truth; live mode only executes files the operator
  can read.

## D2 — Stack & packaging **(user)**
- **Options:** Go single binary; React/Vite static site; Node + React.
- **Chosen:** **Go single binary.**
- **Why:** Air-gap favors one self-contained artifact — no runtime deps, runs
  offline, trivial to hand over. The web UI is embedded via `go:embed`.

## D3 — Registry modes **(user)**
- **Options (multi-select):** non-authenticated mirror; authenticated mirror;
  pull-through cache.
- **Chosen:** **Non-authenticated mirror only** (the v1.4.0 default per the
  prerequisites).
- **Why:** Matches the documented default and reduces config surface. The config
  model is structured so authenticated/pull-through can be added later without
  rework. (Recorded as a non-goal in [01-overview](01-overview.md).)

## D4 — Frontend build step (engineering)
- **Chosen:** **No build step.** Hand-written HTML/CSS/vanilla JS, embedded.
- **Why:** So `go build ./...` alone (with vendored deps) produces the binary
  **offline** — critical for a customer building inside an air-gap, and avoids a
  Node toolchain dependency.

## D5 — Dependency & offline builds (engineering)
- **Chosen:** One dependency (`gopkg.in/yaml.v3`), **vendored**.
- **Why:** YAML load/save for the config; vendoring keeps the repo buildable with
  no network access.

## D6 — Cert handling: selectable TLS mode (user-directed)
- **Context:** User first asked to change image-repo cert handling; clarifying
  questions were dismissed; the follow-up directive was to "choose between
  self-signed and custom certs before installation."
- **Chosen:** A **TLS mode selector** (`none` / `self-signed` / `custom-ca`),
  later extended with `acme`. Both cert modes create the CA secret and wire
  `registryCertSecret`; `k0sURLCertSecret` only when the fileserver is HTTPS.
- **Why:** A config-time choice is what "before installation" means; the two cert
  modes share the same plumbing, differing mainly in guidance and which cert the
  operator supplies.

## D7 — Prep-side push verification (user-directed)
- **Chosen:** In cert modes, the skopeo push **verifies** against the CA
  (`--dest-tls-verify=true --dest-cert-dir /certs`); `none` keeps
  `--dest-tls-verify=false` (plain-HTTP registries); `acme` verifies against
  public roots.
- **Why:** Trust the customer CA rather than blindly disabling TLS; keep the
  insecure path only where a plain-HTTP registry genuinely needs it.

## D8 — ACME solver **(user)**
- **Options:** HTTP-01 via ingress; DNS-01; both.
- **Chosen:** **HTTP-01 via ingress.**
- **Why:** Simplest to automate; works with public LE or an internal ACME server;
  DNS-01 solvers are provider-specific. (DNS-01 recorded as a non-goal / possible
  follow-up.)

## D9 — ACME trust **(user)**
- **Options:** Assume publicly trusted (Let's Encrypt); support both via an
  internal-CA toggle.
- **Chosen:** **Assume publicly trusted.**
- **Why:** LE certs chain to public roots the cluster already trusts, so no CA
  secret is wired; ACME behaves like `none` for trust but with verified pushes.
  An internal ACME server URL is still settable.

## D10 — Where checks run (engineering, documented behavior)
- **Chosen:** Live checks run **from the machine running the binary** and reflect
  *its* reachability (registry/version/k0s/ACME) or *its* kubectl context
  (cluster/StorageClass/IngressClass).
- **Why:** That is the machine whose access actually matters. Guidance: run
  "Validate & test access" on the prep box and again on the cluster-side jump
  host; the panel labels say which target each result is about.

## D11 — Git workflow & pushing (policy)
- **Chosen:** Commit each coherent change on `main`; **push only when the user
  asks**. Foundational commits went to `main` (greenfield project the user owns).
- **Why:** The repo is the user's own project intended to live on `main`; pushing
  is outward-facing and was always explicitly requested.

## D12 — Branding source (constraint-driven) **(engineering)**
- **Constraint:** `github.com/k0rdent/k0rdent-ui` is private / 404 from this
  environment.
- **Chosen:** Match the **official public k0rdent brand** (logo + color tokens
  from `github.com/k0rdent/website`).
- **Why:** Same design system the enterprise team builds on; it's the best
  available authoritative source. Flagged so the user can reconcile against the
  private repo's exact tokens/font if needed.
