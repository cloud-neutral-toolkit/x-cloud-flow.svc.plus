# XCloudFlow 数据存储设计

> Refined in: [docs/plans/2026-03-15-xcloudflow-postgresql-state-design.md](/Users/shenlan/workspaces/cloud-neutral-toolkit/x-cloud-flow.svc.plus/docs/plans/2026-03-15-xcloudflow-postgresql-state-design.md)

本文档描述在 XCloudFlow 中构建自托管基础设施状态与资源图谱服务的设计方案。方案坚持只使用 Terraform SDK 与 Pulumi SDK 及其模块生态，
自研状态服务与数据持久层，不依赖官方默认的对象存储或 SaaS 后端。

## 1. 设计目标

- **统一状态来源**：Terraform 与 Pulumi 执行结果统一写入 PostgreSQL，形成强一致的权威状态视图。
- **细粒度审计与回滚**：记录每次计划/执行的资源级事件、字段级变更，支持版本追踪与回滚。
- **多云全局感知**：抽象跨栈、跨云的资源清单与依赖关系，支撑漂移检测、孤儿资源识别和影响分析。
- **可扩展联邦查询**：通过 SQL/图查询与外部系统（CMDB、工单、监控、GraphDB）联动，打通自动化治理链路。
- **高可用与安全**：利用 PostgreSQL 生态实现分区、加密、租户隔离与自动化运维。

## 2. 技术栈

- **核心数据库**：PostgreSQL 14+，启用 JSONB/GiN、行级安全（RLS）。
- **扩展组件**：
  - `pgcrypto`：字段级加密与脱敏。
  - `uuid-ossp` 或 `pgcrypto` 的 `gen_random_uuid()`：生成稳定的资源标识。
  - `btree_gin` 与 `jsonb_path_ops`：优化 JSONB 查询。
  - `pg_partman` 或 TimescaleDB：事件与审计数据的时间分区管理。
  - `pg_cron`：定时任务（漂移巡检、云侧盘点）。
  - `LISTEN/NOTIFY`：驱动增量解析器。
  - （可选）Apache AGE/AgensGraph：在 PostgreSQL 内运行 Cypher 图查询。
  - （可选）`postgres_fdw`：联邦外部 CMDB/监控数据库。

## 3. 模块化服务拆分

| 服务 | 职责 | 说明 |
| --- | --- | --- |
| 状态服务 (State Service) | 对接 Terraform/Pulumi HTTP Backend，完成状态 CRUD、锁、版本管理 | Go 实现，使用 Terraform/Pulumi SDK 自带的序列化格式 |
| 解析器 (Plan/Event Parser) | 监听状态写入事件，解析 plan/apply 输出，生成资源事件 | 以 Go 协程消费 LISTEN/NOTIFY 或轻量队列 |
| 清单同步 (Inventory Builder) | 基于事件重放维护 `resources_current` 表 | 保证幂等，支持补偿重放 |
| 依赖图构建 (Graph Builder) | 提取 dependsOn/属性引用，写入依赖边表或 AGE 图 | 统一多云资源关系 |
| 漂移巡检 (Drift Worker) | 定期调用云厂商 SDK 列举真实资源对比 | 输出孤儿/漂移记录 |
| 外部联动 (Integration Bridge) | 通过 FDW 或 Webhook 驱动 CMDB/工单/监控 | 可扩展插件机制 |

## 4. 数据模型

```sql
-- 1) 状态版本记录
CREATE TABLE iac.states (
  id           BIGSERIAL PRIMARY KEY,
  tool         TEXT NOT NULL,              -- terraform | pulumi
  project      TEXT NOT NULL,
  stack        TEXT NOT NULL,
  version      BIGINT NOT NULL,
  state_json   JSONB NOT NULL,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  actor        TEXT,
  etag         TEXT NOT NULL,              -- sha256(state_json)
  UNIQUE(tool, project, stack, version)
);

CREATE TABLE iac.state_heads (
  tool TEXT, project TEXT, stack TEXT,
  head_version BIGINT NOT NULL,
  PRIMARY KEY (tool, project, stack)
);

-- 2) 状态锁
CREATE TABLE iac.state_locks (
  tool TEXT, project TEXT, stack TEXT,
  lock_id TEXT, who TEXT,
  created_at TIMESTAMPTZ DEFAULT now(),
  PRIMARY KEY (tool, project, stack)
);

-- 3) 资源事件（字段级变更）
CREATE TABLE iac.resource_events (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  run_id      UUID NOT NULL,
  stable_uid  UUID NOT NULL,
  provider    TEXT NOT NULL,
  resource_type TEXT NOT NULL,
  addr        TEXT NOT NULL,
  action      TEXT NOT NULL CHECK (action IN ('create','update','delete','no-op','drift')),
  patch_json  JSONB,
  before_hash TEXT,
  after_hash  TEXT,
  cloud       TEXT,
  region      TEXT,
  stack       TEXT,
  at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 4) 当前资源清单
CREATE TABLE iac.resources_current (
  stable_uid  UUID PRIMARY KEY,
  provider    TEXT,
  resource_type TEXT,
  addr        TEXT,
  cloud       TEXT,
  region      TEXT,
  stack       TEXT,
  attrs       JSONB,
  attrs_hash  TEXT,
  last_run_id UUID,
  updated_at  TIMESTAMPTZ DEFAULT now()
);

-- 5) 依赖边
CREATE TABLE iac.dependency_edges (
  src_uid UUID NOT NULL,
  dst_uid UUID NOT NULL,
  kind    TEXT NOT NULL,
  PRIMARY KEY (src_uid, dst_uid, kind)
);

CREATE INDEX iac_state_idx ON iac.states(tool, project, stack, version DESC);
CREATE INDEX iac_resource_patch_gin ON iac.resource_events USING GIN (patch_json jsonb_path_ops);
```

## 5. 状态服务交互流程

1. **Terraform HTTP Backend**
   - `GET /terraform/state/{project}/{stack}`：读取最新版本，返回 JSONB。
   - `POST /terraform/state/...`：在事务中插入新版本 → 更新 `state_heads` → 触发 `NOTIFY`。
   - `POST /terraform/lock/...` 与 `/unlock/...`：映射到 `pg_try_advisory_lock` / `pg_advisory_unlock`，并更新 `state_locks`。
2. **Pulumi Shim**
   - 执行前：`pulumi stack export` → 写临时文件 → `pulumi stack import` 指向 HTTP 服务。
   - 执行后：`pulumi stack export` → 上传 JSON（同样走版本写入/锁逻辑）。
3. **事务保障**
   - 所有状态写入在单事务内完成，失败回滚。
   - 版本号采用 `state_heads.head_version + 1` 自增，`etag` 确保内容完整性。

## 6. 资源事件解析

- Terraform：使用 Terraform SDK 的 `plans`/`states` 包解码 plan/apply JSON，按资源地址生成 `resource_events`。
- Pulumi：利用 Automation API 获取 `Result.Steps`，映射到统一事件结构。
- 事件重放：对每个 `stable_uid` 以时间顺序重放补丁，更新 `resources_current` 与 `dependency_edges`。
- `stable_uid` 生成策略：
  - 优先使用资源自身的全局标识（如 ARN/ResourceID），否则基于 provider+type+addr 的哈希。

## 7. 多云全局感知

- **漂移检测**：`resources_current` 与云侧盘点表 `cloud_inventory` 对比（pg_cron 周期任务 + 云 SDK）。
- **孤儿资源**：云侧存在但状态库无记录；写入 `cloud_orphans` 审计表，并推送工单。
- **依赖分析**：
  - 递归 CTE：计算爆炸半径与影响面。
  - AGE/Cypher：提供图查询接口供可视化前端使用。
- **跨系统联动**：
  - CMDB：通过 FDW 将 CMDB 表映射到 PostgreSQL，支持 JOIN。
  - 工单：由触发器或 `LISTEN/NOTIFY` 将风险事件投递至自动化工单系统。
  - 监控/告警：以 `NOTIFY` 或 Webhook 推送给 Prometheus Alertmanager / Slack / 飞书。

## 8. 安全与合规

- **访问控制**：启用 PostgreSQL RLS，按组织/项目/环境过滤数据；服务端通过 OIDC/JWT 映射租户。
- **敏感数据处理**：
  - `attrs` 字段写入前对密码、密钥等做脱敏或 `pgcrypto` 加密。
  - 管控平面与执行平面分离，状态服务不持有长期云凭证。
- **审计**：所有 API 请求与数据库变更记录在 `audit_logs` 表，支持回放。
- **备份与高可用**：使用 WAL-G 或原生逻辑备份，主从/Patroni 保证故障切换。

## 9. 运维实践

- **分区策略**：`resource_events`、`cloud_audit` 按月或按 `run_id` 时间分区，降低大表膨胀。
- **性能优化**：
  - 对常用 JSON 路径创建表达式索引（例如 `attrs->>'arn'`）。
  - 使用 `VACUUM (ANALYZE)` 与 `auto_explain` 监控慢查询。
- **可观测性**：
  - 状态写入量、解析延迟、漂移结果等指标通过 Prometheus 导出。
  - 结合 Grafana 看板展示多云资源总览与事件趋势。
- **回压策略**：`NOTIFY` 仅作为触发信号，解析器从队列批量消费，防止高峰时阻塞事务。

## 10. 与 XCloudFlow 的集成

- CLI 层通过 Terraform/Pulumi SDK 执行栈，HTTP Backend 地址与凭证由 `init --dbconfig` 下发。
- 执行结束后自动调用状态服务写入，触发事件解析与清单更新。
- `modules` 目录可扩展成插件：
  - `state` 插件：封装 Terraform/Pulumi Backend 客户端。
  - `inventory` 插件：封装云侧盘点逻辑，可按云厂商拆分。
  - `graph` 插件：输出依赖边与图查询接口。
- 将 `resources_current` 与依赖图提供给前端（或 Grafana）用于实时展示多云拓扑、漂移列表。

## 11. 演进路线

1. **阶段一**：落地 PostgreSQL 状态服务 + 解析器，替换默认后端，实现版本/锁/审计。
2. **阶段二**：补全资源清单、依赖图与漂移检测，打通 CMDB/工单。
3. **阶段三**：引入 AGE 图扩展、成本/风险分析算法，提供可视化洞察。
4. **阶段四**：探索跨区域 HA、多活部署，以及与策略引擎（OPA、Open Policy Agent）集成，实现合规前置。

## 12. 总结

通过 PostgreSQL 及其扩展打造自托管的状态与资源图谱服务，XCloudFlow 可以在不依赖第三方后端的情况下，实现强一致的状态管理、精细化审计、
多云全局视图与自动化治理。配合 Terraform/Pulumi SDK 现有能力，该方案兼顾生态兼容与企业级可控性，为后续云资源治理提供坚实基础。
