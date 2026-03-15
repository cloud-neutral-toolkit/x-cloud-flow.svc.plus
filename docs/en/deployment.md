# Deployment

This repository is a multi-component automation monorepo spanning CLI, agent, and infrastructure orchestration responsibilities.

Use this page to standardize deployment prerequisites, supported topologies, operational checks, and rollback notes.

## Current code-aligned notes

- Documentation target: `x-cloud-flow.svc.plus`
- Repo kind: `automation-monorepo`
- Manifest and build evidence: go.mod (`xcloudflow`)
- Primary implementation and ops directories: `cmd/`, `internal/`, `deploy/`, `ansible/`, `scripts/`, `examples/`, `sql/`, `configs/`
- Package scripts snapshot: No package.json scripts were detected.

## Existing docs to reconcile

- No directly matching legacy docs were detected; this page is currently the canonical seed.

## What this page should cover next

- Describe the current implementation rather than an aspirational future-only design.
- Keep terminology aligned with the repository root README, manifests, and actual directories.
- Link deeper runbooks, specs, or subsystem notes from the legacy docs listed above.
- Verify deployment steps against current scripts, manifests, CI/CD flow, and environment contracts before each release.
