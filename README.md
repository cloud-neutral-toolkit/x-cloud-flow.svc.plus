# ☁️ XCloudFlow Monorepo

XCloudFlow 将多云基础设施、配置编排与边缘执行整合在一个仓库中。仓库内包含三个相互独立、又能协同工作的 CLI：

## 仓库概览

XCloudFlow 是一个面向多云场景的单体仓库，聚合了基础设施管理、配置编排和边缘执行三大能力，并通过三个可协同的 CLI 组件来覆盖从控制平面到终端节点的自动化流程。

## 核心组件

- **xcloud-cli（Go）**：定位为 Terraform/Pulumi 场景的控制平面 CLI，提供 `up`、`down`、`export`、`import`、`ansible` 等命令来统一管理多云部署的全生命周期。
- **xconfig（Go）**：类 Ansible 的任务执行器，支持 `remote` 和 `playbook` 等子命令，可运行多种模块（shell、command、copy、service、template、setup、apt/yum 等）完成批量配置及运维操作。
- **xconfig-agent（Rust）**：轻量级边缘 Agent，按照配置定期从 Git 仓库拉取 Playbook 并在本地执行，执行结果回写到指定目录，适用于无人值守或边缘环境。

| 目录            | 语言 | 说明 |
|-----------------|------|------|
| `xcloud-cli/`   | Go   | 面向 Terraform/Pulumi 场景的控制平面 CLI，统一管理多云部署生命周期。|
| `xconfig/`      | Go   | 类 Ansible 的任务/剧本执行器，提供 `remote`/`playbook` 等命令。|
| `xconfig-agent/`| Rust | 轻量级边缘 Agent，周期性拉取剧本并在本地执行，支撑无人值守环境。|

## 关键特性

- 提供聚合型根目录 Makefile，一键触发各子项目的构建与演示命令，降低多组件协作门槛。
- 文档体系覆盖整体架构、模块化执行框架、弹性基础设施即代码设计及 Playbook DSL 规范，为二次开发和平台扩展提供详实参考。
- 全栈设计强调以 GitOps 为唯一真相源，通过策略路由、依赖图和统一执行引擎，构建多云一致性的计划/差异/审批/审计闭环。
- 补齐 `xcloudflow` CLI/MCP 面，新增 Codex/OpenClaw bridge，把仓库转换成面向 IaC 自动化的智能体工作台。

---

## 🚀 快速开始

### 1. xcloud-cli（控制平面 CLI）
```bash
cd xcloud-cli
make build        # or `go run main.go --env sit up`
```
常用子命令：`up`、`down`、`export`、`import`、`ansible`。详情见 `xcloud-cli/Makefile` 或 `xcloud-cli/cmd/*.go`。

### 2. xconfig（任务编排 CLI）
```bash
cd xconfig
make build
./xconfig remote all -i example/inventory -m shell -a 'id'
```
- `xconfig remote`：远程命令执行（shell/command/copy/service 等模块）。
- `xconfig playbook`：运行 YAML Playbook，支持 `template`、`setup`、`apt/yum` 等模块。
- 更多示例参见 `xconfig/example/` 与 `xconfig/README.md`。

### 3. xconfig-agent（边缘执行 Agent）
```bash
cd xconfig-agent
cargo build --release
./target/release/xconfig-agent oneshot
```
默认配置从 `/etc/xconfig-agent.conf` 拉取 Git 仓库、读取 Playbook，并将执行结果落盘到 `/var/lib/xconfig-agent/`。

### 4. xcloudflow（IaC Agent / MCP Server）
```bash
go run ./cmd/xcloudflow mcp serve --addr :8808 --workspace "$PWD" --env-file .env
go run ./cmd/xcloudflow agent spec --config examples/stackflow/demo.yaml --env prod --env-file .env
go run ./cmd/xcloudflow agent register-openclaw --env-file .env
```
- `xcloudflow mcp serve`：在本地 `:8808` 暴露 MCP tools，保留原有 `stackflow.*` 工具，同时新增 `agent.codex.manifest`、`agent.openclaw.patch`、`iac.terraform.*`、`config.ansible.*`、`edge.ssh.exec`、`terraform.*`、`ansible.*`。
- `xcloudflow agent spec`：组合 StackFlow plan、Codex manifest 和 OpenClaw patch。
- `xcloudflow agent register-openclaw`：读取本地 `.env`，输出可 merge 的 OpenClaw ACP agent patch，默认 agent id 是 `xcloudflow-iac`。

### 5. embedded codex（submodule）
```bash
make codex-init
make codex-status
make codex-home
make codex-render-config
```
- `third_party/codex` 用于保留上游 OpenAI Codex 源码，XCloudFlow 通过 bridge 生成稳定的 ACP runtime manifest，而不是硬编码依赖上游内部构建结构。
- `configs/codex/` 和 `scripts/codex/` 为项目级 `CODEX_HOME`、`config.toml`、本地 MCP 启动和 `codex exec` 包装层提供模板与脚本。
- 如果要让 Codex ACP runtime 通过 OpenAI-compatible gateway 工作，请在启动前提供 `OPENAI_BASE_URL` 与 `OPENAI_API_KEY`，或让脚本通过 `.env` 自动注入。

---

## 🧰 仓库级 Makefile

根目录提供一份聚合 `Makefile`，可快速调用各子项目命令：
```bash
make help
make xcloud-build
make xconfig-playbook
make xconfig-agent-run
make xcloudflow-mcp
make codex-home
make xcloudflow-openclaw-register
```

---

## 📚 设计文档

详见 `docs/` 目录，涵盖：
- `XCloudFlowDesign.md`：整体平台架构
- `ModuleExecutionDesign.md`：模块化执行框架设计
- `ElasticIACDesign.md`：Go + Pulumi 弹性 IAC 架构
- `craftweave-playbook-spec.md`：Xconfig Playbook DSL
- `stackflow/codex-openclaw.md`：Codex 子模块、IaC Agent 与 OpenClaw Gateway 集成
- `plans/2026-03-14-xcloudflow-codex-acp-iac-edge-ssh.md`：Codex ACP + Terraform/Ansible + edge SSH 详细实施方案

---

## 🤝 贡献

1. Fork 并创建功能分支。
2. 在对应子目录内运行 `make test`/`cargo test`（如适用）。
3. 提交 PR 并附上测试记录。

欢迎提出 Issue 或 PR，一起打造云管 + 配置 + 边缘执行的一体化工作流。☁️🧵🦀
