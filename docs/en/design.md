# Design

This repository is a multi-component automation monorepo spanning CLI, agent, and infrastructure orchestration responsibilities.

Use this page to consolidate design decisions, ADR-style tradeoffs, and roadmap-sensitive implementation notes.

## Current code-aligned notes

- Documentation target: `x-cloud-flow.svc.plus`
- Repo kind: `automation-monorepo`
- Manifest and build evidence: go.mod (`xcloudflow`)
- Primary implementation and ops directories: `cmd/`, `internal/`, `deploy/`, `ansible/`, `scripts/`, `examples/`, `sql/`, `configs/`
- Package scripts snapshot: No package.json scripts were detected.

## Existing docs to reconcile

- `ElasticIACDesign.md`
- `ModuleExecutionDesign.md`
- `MultiCloudIACDesign.md`
- `StateServiceStorageDesign.md`
- `XCloudFlowDataStorageDesign.md`
- `XCloudFlowDesign.md`
- `craftweave-playbook-spec.md`
- `design.md`

## What this page should cover next

- Describe the current implementation rather than an aspirational future-only design.
- Keep terminology aligned with the repository root README, manifests, and actual directories.
- Link deeper runbooks, specs, or subsystem notes from the legacy docs listed above.
- Promote one-off implementation notes into reusable design records when behavior, APIs, or deployment contracts change.
