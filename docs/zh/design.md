# 设计

该仓库是多组件自动化单体仓库，横跨 CLI、Agent 与基础设施编排职责。

本页用于汇总设计决策、类似 ADR 的权衡记录，以及与路线图相关的实现说明。

## 与当前代码对齐的说明

- 文档目标仓库: `x-cloud-flow.svc.plus`
- 仓库类型: `automation-monorepo`
- 构建与运行依据: go.mod (`xcloudflow`)
- 主要实现与运维目录: `cmd/`, `internal/`, `deploy/`, `ansible/`, `scripts/`, `examples/`, `sql/`, `configs/`
- `package.json` 脚本快照: No package.json scripts were detected.

## 需要继续归并的现有文档

- `ElasticIACDesign.md`
- `ModuleExecutionDesign.md`
- `MultiCloudIACDesign.md`
- `StateServiceStorageDesign.md`
- `XCloudFlowDataStorageDesign.md`
- `XCloudFlowDesign.md`
- `craftweave-playbook-spec.md`
- `design.md`

## 本页下一步应补充的内容

- 先描述当前已落地实现，再补充未来规划，避免只写愿景不写现状。
- 术语需要与仓库根 README、构建清单和实际目录保持一致。
- 将上方列出的历史 runbook、spec、子系统说明逐步链接并归并到本页。
- 当行为、API 或部署契约发生变化时，把一次性实现笔记提升为可复用设计记录。
