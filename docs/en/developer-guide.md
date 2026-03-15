# Developer Guide

This repository is a multi-component automation monorepo spanning CLI, agent, and infrastructure orchestration responsibilities.

Use this page to document local setup, project structure, test surfaces, and contribution conventions tied to the current codebase.

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
- Keep setup and test commands tied to actual package scripts, Make targets, or language toolchains in this repository.
