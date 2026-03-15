# Architecture

This repository is a multi-component automation monorepo spanning CLI, agent, and infrastructure orchestration responsibilities.

Use this page as the canonical bilingual overview of system boundaries, major components, and repo ownership.

## Current code-aligned notes

- Documentation target: `x-cloud-flow.svc.plus`
- Repo kind: `automation-monorepo`
- Manifest and build evidence: go.mod (`xcloudflow`)
- Primary implementation and ops directories: `cmd/`, `internal/`, `deploy/`, `ansible/`, `scripts/`, `examples/`, `sql/`, `configs/`
- Package scripts snapshot: No package.json scripts were detected.

## Existing docs to reconcile

- `records/2026-03-14-openclaw-three-agent-architecture.md`

## What this page should cover next

- Describe the current implementation rather than an aspirational future-only design.
- Keep terminology aligned with the repository root README, manifests, and actual directories.
- Link deeper runbooks, specs, or subsystem notes from the legacy docs listed above.
- Keep diagrams and ownership notes synchronized with actual directories, services, and integration dependencies.
