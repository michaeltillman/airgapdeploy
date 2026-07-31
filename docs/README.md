# airgapdeploy — Documentation Library

A complete engineering record of the **k0rdent AI Enterprise Air-Gap Deployment**
tool (`airgapdeploy`): what it is, how it was built, every decision and why, how
each piece was verified, and how to deliver and use it.

> **Fidelity note.** This library faithfully reconstructs the substance of the
> build — every request, decision, action, and verification result — from the
> project history and git record. It does not reproduce private model
> chain-of-thought token-for-token, nor every raw byte of terminal output;
> those are not retained verbatim. Commands, observed results, and rationale are
> recorded as they occurred.

## Contents

| # | Document | What it covers |
|---|----------|----------------|
| 01 | [Overview](01-overview.md) | Purpose, users, scope, non-goals |
| 02 | [Architecture](02-architecture.md) | Components, repo layout, data flow, how artifacts are generated |
| 03 | [Build history](03-build-history.md) | **Turn-by-turn** record: every request → reasoning → actions → tool results → commit |
| 04 | [Decision log](04-decisions.md) | Every explicit decision (with the options weighed and why) |
| 05 | [Feature catalog](05-features.md) | Every capability, field, generated artifact, and check |
| 06 | [Verification log](06-verification.md) | How each thing was tested and the results observed |
| 07 | [Changelog](07-changelog.md) | Commit-by-commit history |
| 08 | [Delivery & usage](08-delivery-and-usage.md) | How to build, ship to customers, and run |

## Related in-repo docs
- [Top-level README](../README.md) — quick start
- Generated `RUNBOOK.md` — produced per-config by the tool itself (see the Runbook tab / `artifacts/RUNBOOK.md`)

## The tool in one paragraph
`airgapdeploy` is a single, self-contained Go binary that serves a k0rdent-branded
web wizard for performing an **air-gapped k0rdent Enterprise installation**. It
follows the Mirantis air-gap procedure. You fill in a Configure screen; it
validates every field, actively tests connectivity to each target, then
**generates** the complete artifact set (prep script, k0s fileserver manifests,
Helm values, install/verify scripts, access-validation + diagnostics tooling,
optional cert-manager ACME resources, and a runbook). With `--live` it can also
**execute** those generated scripts and stream logs. It is designed to be handed
to a customer or run on-site.
