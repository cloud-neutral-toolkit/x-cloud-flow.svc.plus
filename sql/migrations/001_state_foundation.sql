BEGIN;

CREATE SCHEMA IF NOT EXISTS state;
CREATE SCHEMA IF NOT EXISTS inventory;
CREATE SCHEMA IF NOT EXISTS cmdb;
CREATE SCHEMA IF NOT EXISTS ops;

CREATE TABLE IF NOT EXISTS state.objects (
  tenant_id      TEXT NOT NULL DEFAULT 'default',
  object_key     TEXT NOT NULL,
  version        BIGINT NOT NULL,
  tool           TEXT NOT NULL DEFAULT 'generic',
  project        TEXT,
  env            TEXT,
  resource_scope TEXT,
  content_json   JSONB NOT NULL DEFAULT '{}'::jsonb,
  content_bytes  BYTEA,
  etag           TEXT,
  actor          TEXT,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, object_key, version)
);

CREATE INDEX IF NOT EXISTS state_objects_project_env_created
  ON state.objects(tenant_id, project, env, created_at DESC);

CREATE TABLE IF NOT EXISTS state.heads (
  tenant_id     TEXT NOT NULL DEFAULT 'default',
  object_key    TEXT NOT NULL,
  head_version  BIGINT NOT NULL,
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, object_key)
);

CREATE TABLE IF NOT EXISTS state.locks (
  tenant_id    TEXT NOT NULL DEFAULT 'default',
  object_key   TEXT NOT NULL,
  lock_id      TEXT NOT NULL,
  owner        TEXT NOT NULL,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at   TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (tenant_id, object_key)
);

CREATE INDEX IF NOT EXISTS state_locks_expires_at
  ON state.locks(tenant_id, expires_at);

CREATE TABLE IF NOT EXISTS ops.change_sets (
  tenant_id     TEXT NOT NULL DEFAULT 'default',
  change_set_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project       TEXT NOT NULL DEFAULT '',
  env           TEXT NOT NULL DEFAULT '',
  phase         TEXT NOT NULL DEFAULT '',
  status        TEXT NOT NULL DEFAULT 'planned',
  actor         TEXT,
  summary       TEXT,
  inputs        JSONB NOT NULL DEFAULT '{}'::jsonb,
  plan          JSONB NOT NULL DEFAULT '{}'::jsonb,
  result        JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS change_sets_tenant_change_set
  ON ops.change_sets(tenant_id, change_set_id);

CREATE INDEX IF NOT EXISTS change_sets_project_env_created
  ON ops.change_sets(tenant_id, project, env, created_at DESC);

CREATE TABLE IF NOT EXISTS ops.execution_steps (
  step_id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id      TEXT NOT NULL DEFAULT 'default',
  change_set_id  UUID NOT NULL,
  step_name      TEXT NOT NULL,
  step_kind      TEXT NOT NULL,
  status         TEXT NOT NULL DEFAULT 'pending',
  depends_on     JSONB NOT NULL DEFAULT '[]'::jsonb,
  inputs         JSONB NOT NULL DEFAULT '{}'::jsonb,
  result         JSONB NOT NULL DEFAULT '{}'::jsonb,
  started_at     TIMESTAMPTZ,
  finished_at    TIMESTAMPTZ,
  CONSTRAINT execution_steps_change_set_tenant_fkey
    FOREIGN KEY (tenant_id, change_set_id)
    REFERENCES ops.change_sets(tenant_id, change_set_id)
    ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS ops.drift_findings (
  tenant_id       TEXT NOT NULL DEFAULT 'default',
  finding_id      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  resource_uid    TEXT NOT NULL,
  change_set_id   UUID,
  finding_type    TEXT NOT NULL,
  severity        TEXT NOT NULL DEFAULT 'info',
  status          TEXT NOT NULL DEFAULT 'open',
  details         JSONB NOT NULL DEFAULT '{}'::jsonb,
  detected_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS inventory.resources_current (
  tenant_id            TEXT NOT NULL DEFAULT 'default',
  resource_uid         TEXT NOT NULL,
  project              TEXT NOT NULL DEFAULT '',
  resource_type        TEXT NOT NULL,
  cloud                TEXT NOT NULL DEFAULT '',
  region               TEXT NOT NULL DEFAULT '',
  env                  TEXT NOT NULL DEFAULT '',
  engine               TEXT NOT NULL DEFAULT '',
  provider             TEXT NOT NULL DEFAULT '',
  external_id          TEXT,
  name                 TEXT NOT NULL DEFAULT '',
  labels               JSONB NOT NULL DEFAULT '{}'::jsonb,
  desired_state        JSONB NOT NULL DEFAULT '{}'::jsonb,
  observed_state       JSONB NOT NULL DEFAULT '{}'::jsonb,
  drift_status         TEXT NOT NULL DEFAULT 'unknown',
  state_object_key     TEXT,
  last_change_set_id   UUID,
  updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_seen_at         TIMESTAMPTZ,
  PRIMARY KEY (tenant_id, resource_uid)
);

CREATE INDEX IF NOT EXISTS resources_current_project_env
  ON inventory.resources_current(tenant_id, project, env, updated_at DESC);

CREATE INDEX IF NOT EXISTS resources_current_labels_gin
  ON inventory.resources_current USING GIN (labels);

CREATE TABLE IF NOT EXISTS inventory.resource_events (
  tenant_id        TEXT NOT NULL DEFAULT 'default',
  event_id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  resource_uid     TEXT NOT NULL,
  change_set_id    UUID,
  event_type       TEXT NOT NULL,
  diff             JSONB NOT NULL DEFAULT '{}'::jsonb,
  message          TEXT,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS resource_events_uid_created
  ON inventory.resource_events(tenant_id, resource_uid, created_at DESC);

CREATE TABLE IF NOT EXISTS inventory.resource_edges (
  tenant_id     TEXT NOT NULL DEFAULT 'default',
  src_uid       TEXT NOT NULL,
  dst_uid       TEXT NOT NULL,
  edge_type     TEXT NOT NULL,
  source        TEXT NOT NULL DEFAULT 'system',
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, src_uid, dst_uid, edge_type)
);

CREATE TABLE IF NOT EXISTS inventory.external_refs (
  tenant_id      TEXT NOT NULL DEFAULT 'default',
  resource_uid   TEXT NOT NULL,
  ref_type       TEXT NOT NULL,
  ref_value      TEXT NOT NULL,
  provider       TEXT NOT NULL DEFAULT '',
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, resource_uid, ref_type, ref_value)
);

CREATE TABLE IF NOT EXISTS cmdb.export_jobs (
  tenant_id     TEXT NOT NULL DEFAULT 'default',
  job_id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  scope         TEXT NOT NULL DEFAULT 'all',
  status        TEXT NOT NULL DEFAULT 'pending',
  summary       TEXT,
  started_at    TIMESTAMPTZ,
  finished_at   TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS cmdb_export_jobs_tenant_job
  ON cmdb.export_jobs(tenant_id, job_id);

CREATE TABLE IF NOT EXISTS cmdb.export_records (
  tenant_id      TEXT NOT NULL DEFAULT 'default',
  job_id         UUID NOT NULL,
  resource_uid   TEXT NOT NULL,
  export_status  TEXT NOT NULL DEFAULT 'pending',
  payload        JSONB NOT NULL DEFAULT '{}'::jsonb,
  target_system  TEXT NOT NULL DEFAULT 'cmdb',
  exported_at    TIMESTAMPTZ,
  PRIMARY KEY (tenant_id, job_id, resource_uid),
  CONSTRAINT export_records_job_tenant_fkey
    FOREIGN KEY (tenant_id, job_id)
    REFERENCES cmdb.export_jobs(tenant_id, job_id)
    ON DELETE CASCADE
);

COMMIT;
