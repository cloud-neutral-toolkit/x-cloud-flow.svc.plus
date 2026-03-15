# Vibe Coding 参考

该仓库是多组件自动化单体仓库，横跨 CLI、Agent 与基础设施编排职责。

本页用于统一 AI 辅助开发提示词、仓库边界、安全编辑规则与文档同步要求。

## 与当前代码对齐的说明

- 文档目标仓库: `x-cloud-flow.svc.plus`
- 仓库类型: `automation-monorepo`
- 构建与运行依据: go.mod (`xcloudflow`)
- 主要实现与运维目录: `cmd/`, `internal/`, `deploy/`, `ansible/`, `scripts/`, `examples/`, `sql/`, `configs/`
- `package.json` 脚本快照: No package.json scripts were detected.

## 需要继续归并的现有文档

- `records/2026-03-14-openclaw-three-agent-architecture.md`
- `stackflow/agent-mode.md`
- `stackflow/mcp.md`

## 本页下一步应补充的内容

- 先描述当前已落地实现，再补充未来规划，避免只写愿景不写现状。
- 术语需要与仓库根 README、构建清单和实际目录保持一致。
- 将上方列出的历史 runbook、spec、子系统说明逐步链接并归并到本页。
- 当项目新增子系统、受保护目录或强制验证步骤时，同步更新提示模板与仓库规则。
