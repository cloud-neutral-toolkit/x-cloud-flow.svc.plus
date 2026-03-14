# Codex / OpenClaw / Edge SSH

当前仓库已按“Codex 作为内嵌 ACP runtime + Terraform/Ansible 作为默认自动化工具 + 外部 SSH MCP 作为边缘 SSH 能力”实现。

## 当前形态

- `third_party/codex/`
  - OpenAI Codex submodule，只读，不改 upstream 源码。
- `configs/codex/`
  - 项目级 `CODEX_HOME` 模板：`config.toml.tpl`、`mcp-servers.toml.tpl`、`env.example`
- `scripts/codex/`
  - `init-home.sh`
  - `render-config.sh`
  - `ensure-mcp.sh`
  - `run-exec.sh`
- `xcloudflow mcp serve --addr :8808`
  - 暴露 StackFlow、Codex manifest/OpenClaw patch、Terraform、Ansible、edge SSH 相关 MCP tools
- `xcloudflow agent register-openclaw`
  - 输出 OpenClaw ACP agent patch，不直接写 gateway

## 关键约定

- 默认 OpenClaw agent id: `xcloudflow-iac`
- 默认本地 MCP 端口: `8808`
- 默认 Terraform repo: `/Users/shenlan/workspaces/cloud-neutral-toolkit/iac_modules`
- 默认 Playbooks repo: `/Users/shenlan/workspaces/cloud-neutral-toolkit/playbooks`
- 默认 edge SSH MCP server name: `edge_ssh`

## 推荐入口

```bash
make codex-init
make codex-home
make xcloudflow-mcp
go run ./cmd/xcloudflow agent register-openclaw --env-file .env --agent-id xcloudflow-iac
```

详细设计与实施记录见：

- `docs/plans/2026-03-14-xcloudflow-codex-acp-iac-edge-ssh.md`
