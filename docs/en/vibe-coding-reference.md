# Vibe Coding Reference

This repository is a multi-component automation monorepo spanning CLI, agent, and infrastructure orchestration responsibilities.

Use this page to align AI-assisted coding prompts, repo boundaries, safe edit rules, and documentation update expectations.

## Current code-aligned notes

- Documentation target: `x-cloud-flow.svc.plus`
- Repo kind: `automation-monorepo`
- Manifest and build evidence: go.mod (`xcloudflow`)
- Primary implementation and ops directories: `cmd/`, `internal/`, `deploy/`, `ansible/`, `scripts/`, `examples/`, `sql/`, `configs/`
- Package scripts snapshot: No package.json scripts were detected.

## Existing docs to reconcile

- `records/2026-03-14-openclaw-three-agent-architecture.md`
- `stackflow/agent-mode.md`
- `stackflow/mcp.md`

## What this page should cover next

- Describe the current implementation rather than an aspirational future-only design.
- Keep terminology aligned with the repository root README, manifests, and actual directories.
- Link deeper runbooks, specs, or subsystem notes from the legacy docs listed above.
- Review prompt templates and repo rules whenever the project adds new subsystems, protected areas, or mandatory verification steps.
