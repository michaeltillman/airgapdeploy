# 03 — Build History (turn by turn)

A chronological record of every request in the project, the reasoning/approach
taken, the actions performed (tools, commands), the key results/verification
observed, and the outcome (commit). Reconstructed faithfully from the session
and git history.

Legend: **Req** = user request · **Why** = reasoning/approach · **Did** =
actions/tools · **Saw** = key results/verification · **Out** = outcome/commit.

---

## Turn 1 — Build the air-gap installer
- **Req:** Build an automated UI/UX to deploy air-gapped k0rdent Enterprise
  following the Mirantis docs; store the code in the GitHub repo; goal is to use
  on customer sites or hand the repo to customers.
- **Why:** Large greenfield task → research the procedure first, then plan before
  coding. Entered plan mode.
- **Did:** `WebFetch` on the airgap install page + sub-pages (prereq, bundles,
  HTTP fileserver, pull-through, verify-install) to extract exact commands.
  Inspected the empty repo (`git log`, `ls`). Asked 3 architecture questions via
  `AskUserQuestion` (execution model, stack/packaging, registry modes). Wrote a
  plan file; got approval.
- **Decisions (see [04-decisions](04-decisions.md)):** Hybrid generator+live;
  Go single binary; non-authenticated registry only.
- **Did (build):** Scaffolded `go.mod` (go 1.18, yaml.v3), `internal/config`
  (Config + Normalize + Validate + load/save), the artifact templates
  (`01-prepare.sh` … `RUNBOOK.md`), `internal/templates` + `internal/generate`,
  `internal/runner` (preflight + SSE runner), `internal/server` (+ embedded web
  UI: index.html/styles.css/app.js), `cmd/airgapdeploy/main.go`, `Makefile`,
  `README.md`, `.gitignore`. Vendored deps.
- **Saw / verified:** `go vet` + `go build` clean; headless `--generate`
  produced 11 files with correct exec bits; `bash -n` passed on all scripts; YAML
  validated with the vendored yaml.v3; live server smoke test hit
  `/api/{meta,generate,preflight,file}` and streamed a real script over SSE;
  path-traversal rejected; visual check of the Configure screen in-browser.
- **Out:** Committed **d86d9de** "Add airgapdeploy: guided installer UI for
  airgapped k0rdent Enterprise".

## Turn 2 — Push to GitHub
- **Req:** "push it to github".
- **Why:** Explicit request to publish → push. Confirmed working tree clean.
- **Did:** `git push` over HTTPS → failed (no credentials). Checked `gh` (absent),
  SSH keys. Default `ssh -T git@github.com` denied. Surfaced options to the user
  and printed the `id_github.pub` to register.
- **Saw:** HTTPS `could not read Username`; SSH `Permission denied (publickey)`.
- **Out:** Blocked on auth; asked the user to register the key.

## Turn 3 — Retry push after adding key
- **Req:** "added id_github try and push again".
- **Did:** Verified `ssh -i ~/.ssh/id_github … -T git@github.com` now
  authenticates ("Hi michaeltillman!"). Switched remote to SSH
  (`git@github.com:michaeltillman/airgapdeploy.git`), `git push -u origin main`.
- **Saw:** `699316d..d86d9de main -> main`.
- **Out:** Live on GitHub. Updated project memory (SSH remote + key noted).

## Turn 4 — (Cert handling change — clarification)
- **Req:** "change how the image repository is handled because some customers
  will have custom certs."
- **Why:** Genuinely branching (registry-only vs +fileserver, supply method,
  push TLS). Asked 3 clarifying questions via `AskUserQuestion`.
- **Out:** User **dismissed** the questions ("wait for next instruction"). Held.

## Turn 5 — Selectable TLS mode
- **Req:** "have the ability to choose between self-signed certs and custom certs
  before installation."
- **Why:** Replace the `selfSignedCA` boolean with a **TLS mode** selector
  (none / self-signed / custom-ca). Both cert modes generate the CA secret and
  wire `registryCertSecret`; correctness win: emit `k0sURLCertSecret` only when
  the fileserver URL is HTTPS.
- **Did:** `config.go` (TLSMode enum + CACertPath + helpers + validation/
  normalization); branched `03-values.yaml`, `04-install.sh`, `RUNBOOK.md`
  templates; UI radio selector with CA fields revealed only for a cert mode;
  tests for the three modes + HTTPS-gated fileserver secret.
- **Saw:** All three modes generated correctly (none → no secrets; custom+HTTP →
  registryCertSecret only; self-signed+HTTPS → both); `bash -n` + YAML valid;
  UI toggle verified via JS (3 radios, fields reveal/hide, correct hints).
- **Out:** Committed **a3f228d**, pushed.

## Turn 6 — Verify prep push against the CA
- **Req:** "push the verify against the customer ca" (make the prep-side skopeo
  push verify the registry cert against the CA instead of `--dest-tls-verify=false`).
- **Why:** In cert modes, stage the CA into a temp dir and pass skopeo
  `--dest-tls-verify=true --dest-cert-dir /certs`; keep the insecure flag only
  for `none` (plain-HTTP registries).
- **Did:** Branched `01-prepare.sh` (CA var + pre-check + cert-dir mount);
  updated runbook; added a push-branch test.
- **Saw:** Custom-ca push block staged the cert and used `--dest-cert-dir /certs`;
  none kept `--dest-tls-verify=false`; `bash -n` + tests green.
- **Out:** Committed **84ca6d5**, pushed.

## Turn 7 — Access authorization + debug tooling
- **Req:** "check that services and nodes have proper authorization to access and
  pull images … tests/validation to debug if an image doesn't pull or install."
- **Why:** Two complementary things: (a) access validation before install,
  (b) diagnostics after failure. Added a **Preflight** phase and **Diagnostics**
  phase.
- **Did:** Generated `check-access.sh` (registry TCP+TLS(+CA)+`/v2/`+image
  presence + fileserver), `check-node-pull/` (DaemonSet pulling a canary image on
  EVERY node + per-node failure reporting), `diagnose.sh` (ImagePull/CrashLoop
  scan + events + helm/template status). Registered templates; added UI phases;
  runbook validation + troubleshooting + node-CA-trust note; tests.
- **Saw / fixed:** Running `check-access.sh` live caught **two real bash bugs** —
  empty-array expansion under `set -u`, and a doubled HTTP code from
  `|| echo 000`. Both fixed (added an `http_code` helper + safe array
  expansion). All three modes then `bash -n` clean; DaemonSet YAML valid; SSE run
  streamed correct PASS/FAIL output; nav phases confirmed in served `app.js`.
- **Out:** Committed **f9f08a8**, pushed.

## Turn 8 — Confirm push
- **Req:** "push these changes to github please."
- **Did:** Verified working tree clean and `main` in sync — already pushed in
  Turn 7. Reported status.
- **Out:** No-op (already on origin).

## Turn 9 — Reset frozen UI
- **Req:** "reset the test ui it seems to be frozen."
- **Did:** Killed stale `airgapdeploy` processes, rebuilt, restarted a clean
  instance, reopened the browser pane.
- **Out:** Server back up; explained the pane freeze was the browser, not the
  server.

## Turn 10 — Navigable tabs + latent-bug fix
- **Req:** "I can't click the tabs — do I have to complete Configure first?"
- **Why:** Tabs were locked until generation. Unlock navigation; show a
  "generate first" prompt on ungenerated phases.
- **Did:** Removed the nav lock; added the prompt + link back to Configure;
  replaced `.locked` with a subtler `.pending` style.
- **Saw / fixed:** Discovered a **latent bug** — the Preflight and Diagnostics
  phases were added to the nav in Turn 7 but their `<section>` panels were never
  added to `index.html`, so those two tabs rendered nothing (and threw once
  navigation was unlocked). Added the missing panels. Verified via JS: all 7 tabs
  navigate before generation (prompt shown) and render their files after
  (Preflight = 3 files with Run buttons, Diagnostics = diagnose.sh).
- **Out:** Committed **9d9d127**, pushed.

## Turn 11 — Confirm push
- **Req:** "push to github please."
- **Did:** Confirmed clean + in sync (9d9d127 already on origin).
- **Out:** No-op.

## Turn 12 — Rebrand to k0rdent
- **Req:** "structure the UI and brand it the same as the enterprise team
  (github.com/k0rdent/k0rdent-ui)."
- **Why:** The private `k0rdent/k0rdent-ui` repo is **404 from this environment**
  (no access). Matched the **official public k0rdent brand** instead (from
  `github.com/k0rdent/website`): primary `#1AAAFF`, gradient `#1DA7FD → #08F5DA`,
  dark chrome, the k0rdent logo mark.
- **Did:** Extracted brand tokens from the public site CSS + logo SVGs; embedded
  the logo mark (gradient fill) in the topbar; rewrote `styles.css` to the k0rdent
  palette; gradient active nav + primary button, blue chips/legends, mint run
  buttons + live badge; branded SVG favicon; `white-space:nowrap` badge fix.
- **Saw:** Verified live — gradient logo, dark topbar with gradient underline,
  gradient active tab; primary button confirmed to use the gradient; run buttons
  mint; generation still worked (15 files).
- **Out:** Committed **5144af3**, pushed. Noted the private-repo access gap in
  memory.

## Turn 13 — Confirm push
- **Req:** "check all of that into my github repo."
- **Did:** Confirmed clean + in sync (5144af3 on origin).
- **Out:** No-op.

## Turn 14 — Question: nginx role
- **Req:** "is nginx used only for the k0s registry?"
- **Did:** `grep` for nginx across the repo. Explained: **nginx is not a
  registry** — it is the k0s binary HTTP fileserver only; the image registry
  (Harbor/etc.) is customer-provided and separate. nginx's own image is pulled
  *from* that registry.
- **Out:** Explanation, no code change.

## Turn 15 — Question: delivery & usage
- **Req:** "how do I deliver this to customers and how do they bring up the UI?"
- **Did:** Explained the two delivery models (prebuilt binary via `make dist`
  vs hand over the repo), how to run (`./airgapdeploy --live`), the SSH-tunnel
  pattern for remote hosts, the wizard flow, the two-machine reality, and
  operator prerequisites. Offered to produce dist binaries + a customer
  quickstart.
- **Out:** Explanation, no code change. (See [08-delivery-and-usage](08-delivery-and-usage.md).)

## Turn 16 — Rename the heading
- **Req:** rename the top-of-UI name to **"k0rdent AI Enterprise Air-Gap
  Deployment"**.
- **Did:** Updated the `<h1>` (and synced the browser-tab `<title>`); simplified
  the subtitle to "Guided air-gapped installer".
- **Saw:** Fits one line at desktop width (wraps only in the narrow preview pane).
- **Out:** Committed **ac8ce64**, pushed.

## Turn 17 — Configure-page validation + access checks
- **Req:** "error check on the configuration page … populate the airgap version …
  check the execution has access to the target … highlight/report the reason for
  failures … for all the fields."
- **Why:** Turn the Configure page into a real preflight: per-field static
  validation (all problems at once) + live probing of the targets from the
  machine running the tool.
- **Did:** `config.ValidateAll()` (per-field map); new `internal/checks` package
  (registry TCP+TLS-trust+`/v2/`, version bundle upstream, fileserver, kubectl
  cluster `/readyz`, CA cert file); `/api/validate` + `/api/check`; UI inline
  field errors + "Validate & test access" button + results panel; auto-highlight
  fields tied to failures; validation gates Generate. Tests for ValidateAll +
  checks (httptest TLS server, CA cert, host:port parsing). Made checks
  concurrent (~5s).
- **Saw:** validate returned all 5 field reasons; check returned real results —
  registry FAIL (DNS), images WARN, **version PASS (1.4.0 confirmed upstream)**,
  fileserver WARN, cluster FAIL (real kubectl session error). UI blocked on bad
  fields, rendered the color-coded panel, highlighted the registry field.
- **Out:** Committed **8d8c4cb**, pushed.

## Turn 18 — Restart server
- **Req:** "I am unable to load 127.0.0.1:18085."
- **Did:** The server had been killed during the commit step. Rebuilt + restarted.
- **Out:** Back up; explained the background-process lifecycle.

## Turn 19 — Automated ACME (Let's Encrypt) TLS mode
- **Req:** "in the TLS/certificates portion … choose to use something like Let's
  Encrypt that you can automate."
- **Why:** Add a fourth TLS mode `acme` that generates cert-manager
  `ClusterIssuer` + `Certificate` (cert-manager ships with k0rdent) to automate
  issuance/renewal. Flagged the air-gap tension (public LE needs reachability).
  Asked 2 questions.
- **Decisions:** HTTP-01 via ingress; assume publicly trusted (see
  [04-decisions](04-decisions.md)).
- **Did:** `config.go` (TLSMode acme + ACMEServer/Email/IngressClass, TLSDomains
  helper, validation); `tls-acme/clusterissuer.yaml` + `certificate.yaml`
  (rendered only in acme mode); prep push + check-access verify against public
  roots in acme mode; new "ACME server reachable" check; UI ACME radio + fields;
  runbook ACME section with an explicit air-gap warning; tests.
- **Saw:** acme mode produced 17 files with a correct issuer (server/email/
  http01) and cert (dnsNames from registry+fileserver hosts); non-acme modes
  omit the cert-manager files; YAML valid; `01-prepare.sh` `bash -n` with
  `--dest-tls-verify=true`; UI toggled ACME fields, validation flagged missing
  email.
- **Out:** Committed **ffb6ff7**, pushed. Fixed a "1 field need→needs" grammar nit.

## Turn 20 — Restart server
- **Req:** "the server isn't running again can you restart it."
- **Did:** Rebuilt + restarted on 18085.
- **Out:** Back up.

## Turn 21 — Validate + verify EVERY field
- **Req:** "error checks in every possible field, and connectivity to each of
  those fields once populated — validation and verification on every field."
- **Why:** Close the coverage gaps. Add static validation for the remaining
  fields and a live check for every field that points at a real target.
- **Did:** `ValidateAll` additions (skopeoVersion, nginxImage, busyboxImage,
  storageClass, acmeIngressClass). New checks: k0s binaries upstream (per arch),
  nginx/busybox exact-tag presence in the registry, StorageClass exists,
  IngressClass exists, output dir writable. Distinguished genuine NotFound from
  unreachable-cluster in the kubectl checks. Tests for image-tag splitting,
  output-dir writability, expanded validation.
- **Saw:** validate flagged every newly-covered field; check returned the full
  per-field matrix and **confirmed both k0s arch binaries exist upstream**;
  StorageClass/IngressClass now warn (not falsely "not found") when the cluster
  is unreachable; tests + vet + gofmt green.
- **Out:** Committed **a911e54**, pushed.

## Turn 22 — Restart server
- **Req:** "the server isn't running restart."
- **Did:** Rebuilt + restarted on 18085.
- **Out:** Back up.

## Turn 23 — This documentation library
- **Req:** "give me a full document library / conversation history / tool results
  / verbose output / reasoning / repeated every turn for everything done."
- **Did:** Gathered exact git history + file inventory; wrote this `docs/`
  library.
- **Out:** Committed the docs library (see [07-changelog](07-changelog.md)).
