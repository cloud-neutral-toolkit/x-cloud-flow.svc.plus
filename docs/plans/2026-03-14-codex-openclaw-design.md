# XCloudFlow Codex / OpenClaw Design

日期：2026-03-14

## 目标

在不破坏现有 `xcloud-cli`、`xconfig`、`xconfig-agent` 行为的前提下，为仓库增加三项能力：

1. 以 `openai/codex` 作为内嵌 submodule 保存上游源码。
2. 把 `xcloudflow` 扩展成可供 Codex / ACP runtime 消费的 IaC automation agent 入口。
3. 让仓库可以依据本地 `.env` 生成 OpenClaw Gateway 的 agent 注册 patch。

## 决策

### 决策 1：用 bridge，而不是直接耦合 Codex 上游构建

- 新增 `internal/codex`
- 输出稳定的 `xcloudflow.codex.manifest/v1`
- Manifest 只约定 workspace、repoDir、task、MCP endpoint、system prompt

原因：这样可以保留嵌入式 codex 源码，同时避免把 XCloudFlow 绑定到上游仓库的内部目录结构或构建参数。

### 决策 2：OpenClaw 侧对接 ACP runtime

- 新增 `internal/openclaw`
- 从本地 `.env` 解析 `remote`、`remote-token`、`AI-Gateway-Url`、`AI-Gateway-apiKey`
- 输出 `xcloudflow.openclaw.registration/v1`

原因：OpenClaw 已经支持 Codex ACP harness；XCloudFlow 只需要把 agent/runtime patch 和环境约定生成出来。

### 决策 3：MCP server 增加 agent-oriented tools

新增：

- `stackflow.plan.iac`
- `stackflow.codex.manifest`
- `stackflow.openclaw.registration`

原因：这样 XCloudFlow 不只是一个本地 CLI，也能作为可远程消费的 MCP 服务。

## 风险

- `third_party/codex` 需要额外 `git submodule` 初始化。
- `.env` 当前是混合格式，解析器按宽松 KV 规则处理；如果后续格式大变，需要更新 parser。
- OpenClaw patch 默认不展开 secret，接入时需要在运行环境显式注入 `OPENCLAW_GATEWAY_TOKEN` / `OPENAI_API_KEY`。
