# XCloudFlow Codex ACP IaC + Edge SSH

## Goal

在不修改 `third_party/codex` 上游源码的前提下，把 XCloudFlow 扩成一个以 Codex 为 ACP runtime、以 Terraform/Ansible 为默认自动化工具、并能通过外部 `SSH-MCP-server` 触达边缘节点的控制平面。

三个职责层保持分离：

- 控制平面：`xcloudflow` CLI、MCP server、OpenClaw patch 生成。
- 推理执行层：Codex，通过项目级 `CODEX_HOME` 和 `config.toml` 接入。
- 节点执行层：继续使用 `xconfig-agent`，保持轻量执行器定位。

## Repository Layout

- `third_party/codex/`
  - OpenAI Codex git submodule，只读，不做 vendor copy，不在目录内写本地适配代码。
- `configs/codex/`
  - `config.toml.tpl`
  - `mcp-servers.toml.tpl`
  - `env.example`
- `scripts/codex/`
  - `init-home.sh`
  - `render-config.sh`
  - `ensure-mcp.sh`
  - `run-exec.sh`
- `.xcloudflow/codex-home/default/`
  - 生成态目录，承载项目级 `CODEX_HOME`，已在根 `.gitignore` 中忽略。

## Defaults

- 本地 XCloudFlow MCP 端口：`8808`
- OpenClaw agent id：`xcloudflow-iac`
- Terraform repo：`/Users/shenlan/workspaces/cloud-neutral-toolkit/iac_modules`
- Playbooks repo：`/Users/shenlan/workspaces/cloud-neutral-toolkit/playbooks`
- Edge SSH MCP server name：`edge_ssh`

环境变量可覆盖默认值：

- `XCF_MCP_PORT`
- `XCF_MCP_URL`
- `XCF_CODEX_HOME`
- `XCF_TERRAFORM_REPO`
- `XCF_PLAYBOOKS_REPO`
- `XCF_SSH_MCP_URL`
- `XCF_SSH_MCP_BEARER_TOKEN`

## Codex Integration

### Wrapper Model

Codex 只作为“内嵌推理/执行引擎”，本地适配统一放在 XCloudFlow 包装层：

- `scripts/codex/init-home.sh`
  - 创建 `log/`、`prompts/`、`state/`
  - 调用 `render-config.sh` 渲染项目级配置
- `scripts/codex/render-config.sh`
  - 从 `configs/codex/*.tpl` 渲染 `config.toml` 与 `mcp-servers.toml`
- `scripts/codex/ensure-mcp.sh`
  - 确保本地 `xcloudflow mcp serve --addr :8808` 可用
- `scripts/codex/run-exec.sh`
  - 注入 `CODEX_HOME`、AI gateway、Terraform/Playbooks repo、`edge_ssh` MCP 配置后执行 `codex exec`

### Generated `config.toml`

默认渲染结果包含：

- `sandbox_mode = "workspace-write"`
- `sandbox_workspace_write.writable_roots = [workspace, CODEX_HOME]`
- `network_access = true`
- `mcp_servers.xcloudflow.url = http://127.0.0.1:8808/mcp`
- `mcp_servers.edge_ssh.url = $XCF_SSH_MCP_URL`
- `mcp_servers.edge_ssh.bearer_token_env_var = "XCF_SSH_MCP_BEARER_TOKEN"`，仅在配置存在时输出

## MCP Tool Surface

### Legacy tools kept

- `stackflow.validate`
- `stackflow.plan.dns`
- `stackflow.plan.iac`
- `stackflow.codex.manifest`
- `stackflow.openclaw.registration`

### New domain tools

- `agent.codex.manifest`
- `agent.openclaw.patch`
- `iac.terraform.plan`
- `iac.terraform.apply`
- `config.ansible.check`
- `config.ansible.apply`
- `edge.ssh.exec`

### New raw tools

- `terraform.init`
- `terraform.plan`
- `terraform.apply`
- `terraform.destroy`
- `ansible.playbook`
- `ansible.adhoc`

## Execution Guard

所有变更型操作统一走门禁：

- 请求必须带 `confirm = "APPLY"`
- 请求必须带非空 `change_ref`

适用范围：

- `iac.terraform.apply`
- `terraform.apply`
- `terraform.destroy`
- `config.ansible.apply`
- `ansible.playbook` 非 `check` 模式
- `ansible.adhoc` 非 `check` 模式
- `edge.ssh.exec` 中被识别为变更命令的调用

缺少门禁时：

- Terraform/Ansible apply 直接拒绝。
- SSH 默认只允许 read-only 命令；变更命令直接拒绝。

## Terraform and Playbooks

### Terraform

- 领域工具默认从 `iac_modules/example/terraform/<module>` 解析源目录。
- 为了保持外部 repo 只读，`iac.terraform.*` 会把目标模块 staging 到 `.xcloudflow/terraform/` 后再执行 `terraform init/plan/apply`。
- 原生命令 `terraform.*` 直接在显式 `working_dir` 下执行。

### Ansible

- 默认 playbook root 是 `/Users/shenlan/workspaces/cloud-neutral-toolkit/playbooks`
- 默认 inventory 是 `inventory.ini`
- 默认 `ANSIBLE_CONFIG` 指向 playbooks repo 根目录下的 `ansible.cfg`
- 禁用 retry file，避免在外部 repo 产生多余写入

## Edge SSH MCP

- 不在当前仓库实现 SSH transport。
- 使用 `xcloudflow mcp servers add --name edge_ssh --url <ssh-mcp-url> --auth bearer` 注册外部 MCP server。
- `edge.ssh.exec` 会优先从 registry 中查找 `edge_ssh`，否则回退到环境变量 `XCF_SSH_MCP_URL`。
- 远端 tool 名优先顺序：
  - `XCF_SSH_MCP_TOOL`
  - `ssh_execute`
  - `ssh.exec`
  - `edge.ssh.exec`
  - 其它包含 `ssh` 且包含 `exec/execute` 的 tool

## OpenClaw ACP Agent

默认只生成 patch，不直接写 gateway 配置。

核心字段：

- `runtime.type = "acp"`
- `runtime.acp.agent = "codex"`
- `runtime.acp.backend = "acpx"`
- `runtime.acp.mode = "persistent"`
- `runtime.acp.cwd = <repo-root>`
- `workspace = <repo-root>`

环境映射：

- `remote` / `remote-token` -> OpenClaw remote gateway
- `AI-Gateway-Url` / `AI-Gateway-apiKey` -> `OPENAI_BASE_URL` / `OPENAI_API_KEY`
- `XCF_SSH_MCP_URL` / `XCF_SSH_MCP_BEARER_TOKEN` -> edge SSH MCP
- `CODEX_HOME`, `XCF_MCP_PORT`, `XCF_TERRAFORM_REPO`, `XCF_PLAYBOOKS_REPO` -> ACP runtime shell env

## xconfig-agent Evolution

节点侧继续使用 `xconfig-agent`：

- 仍负责 Git 拉取 playbook
- 仍负责本机 shell/script 执行
- 仍负责结果落盘

这次集成不把 Codex/OpenClaw/SSH-MCP transport 引入到节点侧 agent。

## Commands

### Local control plane

```bash
go run ./cmd/xcloudflow mcp serve --addr :8808 --workspace "$PWD" --env-file .env
```

### Codex runtime assets

```bash
./scripts/codex/init-home.sh
./scripts/codex/ensure-mcp.sh
./scripts/codex/run-exec.sh --help
```

### Agent outputs

```bash
go run ./cmd/xcloudflow agent spec --config examples/stackflow/demo.yaml --env prod --env-file .env
go run ./cmd/xcloudflow agent register-openclaw --env-file .env --agent-id xcloudflow-iac
```

### External SSH MCP registry

```bash
go run ./cmd/xcloudflow mcp servers add --name edge_ssh --url http://127.0.0.1:9000/mcp --auth bearer
go run ./cmd/xcloudflow mcp servers refresh-tools --dsn "$DATABASE_URL"
```

## Verification

建议按这个顺序验收：

1. `go test ./...`
2. `./scripts/codex/init-home.sh`
3. `./scripts/codex/ensure-mcp.sh`
4. `go run ./cmd/xcloudflow agent register-openclaw --env-file .env`
5. 用 `edge.ssh.exec` 跑一次只读命令
6. 用 `iac.terraform.plan` 跑一次 Terraform plan
7. 用 `config.ansible.check` 跑一次 playbook check
