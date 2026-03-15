# XCloudFlow + PostgreSQL 状态库落地方案

## 1. 背景与目标

当前仓库已经具备多块可复用能力，但还没有形成一个统一的权威控制面：

- `xcloudflow` 已有 MCP、agent、OpenClaw/Codex bridge 和 PostgreSQL schema 骨架。
- `xcloud-cli` 已有 IaC 执行路径，但 AWS 之外大多仍处于占位或未收敛状态。
- `xconfig` 已有配置执行能力，但 `cmdb` 仍是占位能力，尚未形成统一资源视图。
- `postgresql.svc.plus` 当前明确可用扩展以 `pgvector`、`pgmq`、`pg_trgm`、`hstore`、`uuid-ossp` 为主，尚未真正提供图关系和定时任务增强所需的稳定基线。

因此，第一阶段不再优先扩展多云广度，也不把 Codex 提升为自治主控，而是先完成以下最小闭环：

- 权威状态库
- 统一资源清单
- CMDB 镜像导出
- 漂移检测基础

目标是把 XCloudFlow 从“执行工具集合”收束为“有状态的多云控制面”。

## 2. 目标架构

### 2.1 三层职责

- 控制面：`xcloudflow`
- 执行面：`xcloud-cli`、`xconfig`、`xconfig-agent`
- 状态底座：`postgresql.svc.plus` 中独立数据库 `xcloudflow`

控制面只负责：

- 计划与编排
- 状态写入与查询
- 资源标准化
- CMDB 镜像投影
- 漂移检测与审计

执行面只负责：

- Terraform/Pulumi 执行
- Playbook/Ansible 执行
- 边缘节点执行

Codex/OpenClaw 第一版只作为平台工程师的操作辅助，不作为权威状态源，也不直接绕过门禁执行 apply。

### 2.2 数据库规划

在 `postgresql.svc.plus` 集群中分配独立数据库：

- database: `xcloudflow`

数据库内部按职责拆分 schema：

- `state`
- `inventory`
- `cmdb`
- `ops`

这样做的好处：

- 控制面状态与其它业务库隔离
- 便于后续独立备份、复制、迁移
- 便于把 inventory 或 cmdb 视图联邦到外部系统

### 2.3 State object key 规范

IaC、DNS、配置等状态对象统一采用路径式 object key：

```text
/project/<project>/env/<env>/kind/<resource_kind>/provider/<provider>/region/<region>/stack/<stack>
```

示例：

```text
/project/payments/env/prod/kind/iac/provider/aws/region/ap-northeast-1/stack/network
/project/payments/env/prod/kind/dns/provider/cloudflare/region/global/stack/public-zone
```

第一版先使用普通列与字符串路径实现，不依赖 `ltree`。后续如补充 `ltree`，再迁移索引与查询路径。

## 3. 数据模型

### 3.1 State schema

#### `state.objects`

用于模拟对象存储版本语义，承载 Terraform/Pulumi/DNS/配置状态快照。

核心字段：

- `object_key`
- `version`
- `tool`
- `project`
- `env`
- `resource_scope`
- `content_json`
- `content_bytes`
- `etag`
- `created_at`
- `actor`

#### `state.heads`

记录每个 `object_key` 当前 head 版本。

核心字段：

- `object_key`
- `head_version`
- `updated_at`

#### `state.locks`

记录状态锁，模拟 S3/HTTP backend 的锁语义。

核心字段：

- `object_key`
- `lock_id`
- `owner`
- `created_at`
- `expires_at`

### 3.2 Ops schema

#### `ops.change_sets`

记录一次变更请求的权威实体。

核心字段：

- `change_set_id`
- `project`
- `env`
- `phase`
- `status`
- `actor`
- `summary`
- `inputs`
- `plan`
- `result`
- `created_at`
- `updated_at`

#### `ops.execution_steps`

记录 `change_set` 下的 DAG 步骤。

核心字段：

- `step_id`
- `change_set_id`
- `step_name`
- `step_kind`
- `status`
- `depends_on`
- `inputs`
- `result`
- `started_at`
- `finished_at`

#### `ops.drift_findings`

记录漂移、孤儿资源、状态缺失等结果。

核心字段：

- `finding_id`
- `resource_uid`
- `change_set_id`
- `finding_type`
- `severity`
- `status`
- `details`
- `detected_at`

### 3.3 Inventory schema

#### `inventory.resources_current`

作为当前资源清单的权威表，承载跨执行器统一资源模型。

资源主键与统一字段固定为：

- `resource_uid`
- `resource_type`
- `cloud`
- `region`
- `env`
- `engine`
- `external_id`
- `name`
- `labels`
- `desired_state`
- `observed_state`
- `drift_status`
- `last_change_set_id`

建议补充的系统字段：

- `project`
- `provider`
- `state_object_key`
- `updated_at`
- `last_seen_at`

#### `inventory.resource_events`

追加式资源事件流，记录 create/update/delete/drift/import/reconcile。

核心字段：

- `event_id`
- `resource_uid`
- `change_set_id`
- `event_type`
- `diff`
- `message`
- `created_at`

#### `inventory.resource_edges`

第一版图关系固定采用关系表，不依赖 AGE。

核心字段：

- `src_uid`
- `dst_uid`
- `edge_type`
- `source`
- `created_at`

第一版支持的 `edge_type`：

- `depends_on`
- `managed_by`
- `belongs_to`
- `runs_on`
- `exposes`
- `attached_to`

图查询实现方式：

- `resource_edges` 关系表
- recursive CTE
- JSONB/GIN 索引

明确不依赖：

- Apache AGE
- 外部图数据库

#### `inventory.external_refs`

用于云资源 ID、ARN、DNS record ID、主机 ID 等外部标识映射。

核心字段：

- `resource_uid`
- `ref_type`
- `ref_value`
- `provider`
- `created_at`

### 3.4 CMDB schema

#### `cmdb.export_jobs`

记录一次 CMDB 镜像导出任务。

核心字段：

- `job_id`
- `scope`
- `status`
- `started_at`
- `finished_at`
- `summary`

#### `cmdb.export_records`

记录单个资源镜像导出结果。

核心字段：

- `job_id`
- `resource_uid`
- `export_status`
- `payload`
- `target_system`
- `exported_at`

CMDB 第一版是只读镜像，不是变更入口，不允许反向修改权威状态。

## 4. PostgreSQL 增强建议

`postgresql.svc.plus` 的后续增强分两个优先级推进。

### 优先级 1

- `pgcrypto`
- `ltree`
- `pg_cron`

用途：

- `pgcrypto`：敏感字段加密、摘要、稳定 UUID/哈希处理
- `ltree`：加速 `/project/env/kind/...` 层级路径查询
- `pg_cron`：漂移扫描、CMDB 导出、状态归档定时任务

### 优先级 2

- `postgres_fdw`
- `pg_partman`

用途：

- `postgres_fdw`：联邦外部 CMDB/报表库
- `pg_partman`：事件与审计大表分区

### 约束

- MVP 不阻塞这些扩展
- 第一版先基于标准 PostgreSQL + 当前已有扩展落地
- 路径层级先以普通列和字符串路径实现，后续再迁移到 `ltree`

## 5. 对 XCloudFlow 的接口变更

第一版需要补齐的 CLI/MCP 面：

- `changes.plan`
- `changes.apply`
- `changes.status`
- `state.get`
- `state.put`
- `state.lock`
- `state.unlock`
- `inventory.list`
- `inventory.get`
- `inventory.graph`
- `cmdb.export`
- `drift.scan`

同时固定以下规则：

- 所有 apply 类操作必须绑定 `change_set_id`
- `change_ref` 退化为兼容字段，不再作为主门禁键
- 执行器返回结果必须标准化入库，不能只返回日志

执行链路统一为：

1. 创建 `change_set`
2. 生成 `state_object_key`
3. 执行 plan/apply
4. 写入 `state.objects`
5. 更新 `inventory.resources_current`
6. 记录 `inventory.resource_events` 与 `inventory.resource_edges`
7. 异步触发 `cmdb.export_jobs`
8. 定期执行 `drift.scan`

## 6. 分阶段实施

### Phase 1: 状态库与 schema

- 新增方案文档
- 在 `sql/` 下建立独立 migration/schema 基线
- 引入 `state.objects / heads / locks`
- 引入 `ops.change_sets / execution_steps`
- 引入 `inventory.resources_current / resource_events / resource_edges`

### Phase 2: planner/router/orchestrator 接 `change_set`

- 统一把现有 plan/apply 执行面纳入 `change_set`
- 用 `change_set_id` 贯穿 MCP 和 CLI apply 门禁
- 让执行步骤按统一结构写入 `ops.execution_steps`

### Phase 3: inventory + CMDB exporter

- Terraform/Pulumi 结果写入 `state` + `inventory`
- Ansible/xconfig 至少写入 `resource_events`
- 建立 `cmdb.export_jobs / export_records`
- 输出只读 CMDB 镜像

### Phase 4: drift worker + 查询接口

- 补 `drift.scan`
- 补 `inventory.graph`
- 增加定时任务策略
- 后续在 `postgresql.svc.plus` 中补 `pgcrypto / ltree / pg_cron`

## Test Plan

需要把以下验收标准作为实施检查项：

- 同一 state object 支持多版本写入与 head 读取
- 锁冲突时不会并发 apply
- apply 之后 `state` 和 `inventory` 都有记录
- `resource_edges` 能表达基础依赖关系
- `cmdb` 只读镜像，不反向回写
- 按 `/project/env/kind/...` 能稳定查询
- 非 MVP provider 返回明确 `not supported`，而不是静默跳过

## Assumptions

- 文档主文件放 `docs/plans/`
- 现有 `docs/XCloudFlowDataStorageDesign.md` 保留，作为背景设计，不立即删除
- 第一版只实现 XCloudFlow 仓库内的状态与查询闭环
- `postgresql.svc.plus` 的镜像增强作为后续配套项，不阻塞首轮实现
