# 三仓库专职 Agent + 专属 MCP + OpenClaw Gateway 主控方案

## 决策摘要

本次方案保留三个独立仓库，不新增第四个总控仓库。

- `x-ops-agent.svc.plus` 对外身份固定为 `xops-agent`
- `x-cloud-flow.svc.plus` 对外身份固定为 `x-automation-agent`
- `x-scope-hub.svc.plus` 对外身份固定为 `x-observability-agent`

唯一主控是 `openclaw-gateway`。它负责 agent 注册表、外部入口路由和多 agent 协调。当前仓库只承担 automation 域能力，不承担全局调度。

## 当前仓库定位

- 仓库：`x-cloud-flow.svc.plus`
- 默认 `OPENCLAW_AGENT_ID`：`x-automation-agent`
- 默认角色：automation 专职 agent
- 主责任：IaC、配置编排、playbook、基础设施自动化

当前仓库继续保留 StackFlow / IaC / Codex bridge 的既有能力，但 OpenClaw 接入方式统一到真实注册命令，不再把 `register-openclaw` 仅当成 patch 输出器。

## 三仓库总体分工

- `xops-agent`
  - 仓库：`x-ops-agent.svc.plus`
  - 事故分析、根因判断、处置建议
- `x-automation-agent`
  - 仓库：`x-cloud-flow.svc.plus`
  - IaC / automation / playbook / deployment
- `x-observability-agent`
  - 仓库：`x-scope-hub.svc.plus`
  - logs / metrics / traces / topology / alert insight

边界约束固定如下：

- automation 侧可以生成 plan、manifest、playbook、deployment automation 结果
- automation 侧不直接给出 incident root cause 或指挥性处置结论
- automation 侧不伪造观测证据

## Gateway 主控边界

`openclaw-gateway` 固定承担：

1. agent 注册表
2. 外部入口路由
3. 多 agent 协调

当前仓库不是 orchestration 引擎，不做全局任务仲裁，不把 OPS judgment 或 observability evidence 下沉到本地实现。

## 统一接入契约

当前仓库与另外两个仓库统一采用：

- `OPENCLAW_GATEWAY_URL`
- `OPENCLAW_GATEWAY_TOKEN`
- `OPENCLAW_GATEWAY_PASSWORD`
- `OPENCLAW_AGENT_ID`
- `OPENCLAW_AGENT_NAME`
- `OPENCLAW_AGENT_WORKSPACE`
- `OPENCLAW_AGENT_MODEL`
- `OPENCLAW_REGISTER_ON_START`
- `AI_GATEWAY_URL`
- `AI_GATEWAY_API_KEY`

统一入口面：

- 真实注册命令：`xcloudflow agent register-openclaw`
- MCP HTTP 入口：`POST /mcp`
- 业务 HTTP API：当前仓库已有 server 入口继续承载 automation 域能力
- Codex runtime 注入面：通过 `AI_GATEWAY_URL` / `AI_GATEWAY_API_KEY` 映射到 `OPENAI_BASE_URL` / `OPENAI_API_KEY`
- A2A HTTP 入口：
  - `POST /a2a/v1/negotiate`
  - `POST /a2a/v1/tasks`
  - `GET /a2a/v1/tasks/{task_id}`

## A2A 标准最小协议

当前仓库与另外两仓统一采用：

请求字段：

- `from_agent_id`
- `to_agent_id`
- `request_id`
- `intent`
- `goal`
- `context`
- `artifacts`
- `constraints`

响应字段：

- `status`
- `owner_agent_id`
- `summary`
- `required_inputs`
- `result`

允许的 `status` 固定为：

- `accepted`
- `declined`
- `needs_input`
- `completed`

所有 A2A 流转必须保留原始 `request_id`，便于 gateway 和各 agent 日志统一追踪。

## 当前仓库接口约定

### MCP

- `POST /mcp`
- 领域工具保持 automation 范围：
  - `stackflow.validate`
  - `stackflow.plan.dns`
  - `stackflow.plan.iac`
  - `stackflow.codex.manifest`
  - `stackflow.openclaw.registration`

### 真实注册

`xcloudflow agent register-openclaw --env-file .env` 现在直接调用 gateway RPC：

- `agents.list`
- `agents.create`
- `agents.update`

仍然保留 `agent spec` 与 `stackflow.openclaw.registration` 用于生成 manifest / registration payload，但这两者不再代替真实注册命令。

### A2A

当前仓库实现的是 automation 侧 A2A 协商器：

- 收到 incident / root cause / runbook 指令时，拒绝吞并执行并 handoff 到 `xops-agent`
- 收到 observability 证据需求时，返回 `needs_input` 并请求 `x-observability-agent`
- 收到 terraform / pulumi / dns / playbook / IaC 计划类目标时，本域接受并生成 automation 侧结果

## 推荐命令

```bash
make xcloudflow-mcp
make xcloudflow-openclaw-register
go run ./cmd/xcloud-server --env-file .env
go run ./cmd/xcloudflow agent register-openclaw --env-file .env
```

## 验收标准

1. `register-openclaw` 连续执行两次：第一次 `created`，第二次 `updated`
2. `/mcp` 仅暴露 automation 域工具
3. `/a2a/v1/negotiate` 对 incident / observability / automation 三类请求给出不同处理
4. 默认 agent 身份固定为 `x-automation-agent`，不再使用 `xcloudflow-codex`
5. Codex 运行时通过 AI gateway 注入模型访问，不把模型配置硬编码在 tool 层
