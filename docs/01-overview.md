# 01 — Overview

## Purpose
Provide an automated UI/UX to deploy **air-gapped k0rdent Enterprise**, following
the Mirantis procedure:
<https://docs.mirantis.com/k0rdent-enterprise/latest/admin/installation/airgap/airgap-install/>

The tool exists so that a Mirantis Principal Solution Architect can either:
1. **Go on-site** at a customer and drive the install from a guided UI, or
2. **Hand the customer this repo/binary** so they can install k0rdent Enterprise
   in their own air-gap without deep tribal knowledge of the procedure.

Repository: <https://github.com/michaeltillman/airgapdeploy>

## The problem it solves
The air-gap install has two sides and many moving parts that are easy to get
wrong:

- **Prep machine (has internet):** download the airgap bundle + k0s binaries,
  verify cosign signatures, `docker load` the bundled skopeo, `skopeo copy` all
  images/charts into a private OCI registry, stage k0s binaries.
- **Air-gapped cluster:** stand up an nginx HTTP fileserver for the k0s binary,
  render a `values.yaml` pointing at the private registry + fileserver,
  `helm install kcm`, verify ~30 pods + Management + templates.

Getting the registry host, fileserver URL, versions, and TLS trust exactly right
— and knowing whether the environment can actually reach each target — is where
installs stall. `airgapdeploy` encodes the procedure, parameterises it, validates
it, and (optionally) runs it.

## Users / personas
- **Solution Architect (primary operator):** runs the binary with `--live`,
  drives the wizard, executes steps, debugs failures.
- **Customer operator (hand-off):** receives the binary or repo, runs it
  (often generator-only), follows the runbook.

## Scope (what it does)
- Guided web wizard (single binary, embedded UI, offline-capable).
- Generates the full artifact set for the non-authenticated-registry air-gap flow.
- Four TLS modes: none, self-signed, custom CA, automated ACME (Let's Encrypt).
- Per-field static validation + live connectivity/verification of every target.
- Access validation (registry + every node) and pull/install diagnostics.
- Optional live execution of the generated scripts with streamed logs.
- k0rdent-branded UI.

## Non-goals (explicitly out of scope, by decision)
- **Authenticated / pull-through registries** — the flow targets the v1.4.0
  non-authenticated registry default. The config model leaves room to add these.
- **Provisioning the management cluster / load balancer / storage** — assumed
  pre-existing per the Mirantis prerequisites.
- **Bundling third-party CLIs** (skopeo/cosign/helm/kubectl) — the operator
  supplies these; the tool's preflight reports which are present.
- **DNS-01 ACME solver** — HTTP-01 via ingress is the implemented default.

## Key constraints that shaped the design
- **Air-gap:** no internet on the target, no CDN, often a single hand-off →
  favor a single self-contained binary that builds and runs offline.
- **Customer-shippable:** must be auditable and buildable by the customer →
  no frontend build step, dependencies vendored.
- **Versions:** defaults target k0rdent Enterprise `1.4.0` / k0s `v1.35.4+k0s.0`;
  all versions are configurable. (One Mirantis doc sub-page showed a stale
  `v1.35.1` example, so versions are never hard-coded blindly.)
