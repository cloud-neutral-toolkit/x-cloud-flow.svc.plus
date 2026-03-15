package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"xcloudflow/internal/defaults"
)

type Store struct {
	pool *pgxpool.Pool
}

func Open(ctx context.Context, dsn string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	ctxPing, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(ctxPing); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() { s.pool.Close() }

// ExecSQL executes raw SQL (used for schema bootstrap).
// Caller is responsible for idempotency (schema.sql should be).
func (s *Store) ExecSQL(ctx context.Context, sql string) error {
	_, err := s.pool.Exec(ctx, sql)
	return err
}

func (s *Store) CreateRun(ctx context.Context, r Run) (string, error) {
	if r.RunID == "" {
		r.RunID = uuid.NewString()
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO xcf.runs (run_id, stack, env, phase, status, actor, config_ref, inputs, plan, result)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9::jsonb,$10::jsonb)
	`, r.RunID, r.Stack, r.Env, r.Phase, r.Status, nullIfEmpty(r.Actor), nullIfEmpty(r.ConfigRef),
		jsonOrEmpty(r.InputsJSON), jsonOrEmpty(r.PlanJSON), jsonOrEmpty(r.ResultJSON),
	)
	if err != nil {
		return "", err
	}
	return r.RunID, nil
}

func (s *Store) FinishRun(ctx context.Context, runID string, status string, resultJSON []byte) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE xcf.runs
		SET status=$2, finished_at=now(), result=$3::jsonb
		WHERE run_id=$1
	`, runID, status, jsonOrEmpty(resultJSON))
	return err
}

func (s *Store) UpsertMCPServer(ctx context.Context, srv MCPServer) (string, error) {
	if srv.ServerID == "" {
		srv.ServerID = uuid.NewString()
	}
	if srv.Kind == "" {
		srv.Kind = "generic"
	}
	if srv.AuthType == "" {
		srv.AuthType = "none"
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO xcf.mcp_servers (server_id, name, base_url, kind, auth_type, audience, enabled)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (name) DO UPDATE SET
		  base_url=EXCLUDED.base_url,
		  kind=EXCLUDED.kind,
		  auth_type=EXCLUDED.auth_type,
		  audience=EXCLUDED.audience,
		  enabled=EXCLUDED.enabled,
		  updated_at=now()
	`, srv.ServerID, srv.Name, srv.BaseURL, srv.Kind, srv.AuthType, nullIfEmpty(srv.Audience), srv.Enabled)
	if err != nil {
		return "", err
	}
	return srv.ServerID, nil
}

func (s *Store) ListMCPServers(ctx context.Context) ([]MCPServer, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT server_id, name, base_url, kind, auth_type, COALESCE(audience,''), enabled, created_at, updated_at
		FROM xcf.mcp_servers
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MCPServer
	for rows.Next() {
		var srv MCPServer
		if err := rows.Scan(&srv.ServerID, &srv.Name, &srv.BaseURL, &srv.Kind, &srv.AuthType, &srv.Audience, &srv.Enabled, &srv.CreatedAt, &srv.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, srv)
	}
	return out, rows.Err()
}

func (s *Store) UpdateMCPToolsCache(ctx context.Context, serverID string, toolsJSON []byte, etag string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO xcf.mcp_tools_cache (server_id, tools, etag)
		VALUES ($1,$2::jsonb,$3)
		ON CONFLICT (server_id) DO UPDATE SET
		  tools=EXCLUDED.tools,
		  etag=EXCLUDED.etag,
		  fetched_at=now()
	`, serverID, jsonOrArray(toolsJSON), nullIfEmpty(etag))
	return err
}

func (s *Store) AddSkillSource(ctx context.Context, src SkillSource) (string, error) {
	if src.SourceID == "" {
		src.SourceID = uuid.NewString()
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO xcf.skill_sources (source_id, name, type, uri, ref, base_path, enabled)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (name) DO UPDATE SET
		  type=EXCLUDED.type,
		  uri=EXCLUDED.uri,
		  ref=EXCLUDED.ref,
		  base_path=EXCLUDED.base_path,
		  enabled=EXCLUDED.enabled,
		  updated_at=now()
	`, src.SourceID, src.Name, src.Type, src.URI, nullIfEmpty(src.Ref), nullIfEmpty(src.BasePath), src.Enabled)
	if err != nil {
		return "", err
	}
	return src.SourceID, nil
}

func (s *Store) ListSkillSources(ctx context.Context) ([]SkillSource, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT source_id, name, type, uri, COALESCE(ref,''), COALESCE(base_path,''), enabled
		FROM xcf.skill_sources
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SkillSource
	for rows.Next() {
		var src SkillSource
		if err := rows.Scan(&src.SourceID, &src.Name, &src.Type, &src.URI, &src.Ref, &src.BasePath, &src.Enabled); err != nil {
			return nil, err
		}
		out = append(out, src)
	}
	return out, rows.Err()
}

func (s *Store) UpsertSkillDoc(ctx context.Context, sourceID string, path string, sha256 string, content string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO xcf.skill_docs (source_id, path, sha256, content)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (source_id, path) DO UPDATE SET
		  sha256=EXCLUDED.sha256,
		  content=EXCLUDED.content,
		  fetched_at=now()
	`, sourceID, path, sha256, content)
	return err
}

func (s *Store) PutStateObject(ctx context.Context, obj StateObject) (*StateObject, error) {
	obj.TenantID = normalizeTenantID(obj.TenantID)
	if obj.ObjectKey == "" {
		return nil, fmt.Errorf("missing object key")
	}
	if obj.Tool == "" {
		obj.Tool = "generic"
	}
	if len(obj.ContentJSON) == 0 {
		obj.ContentJSON = []byte("{}")
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var version int64
	err = tx.QueryRow(ctx, `
		INSERT INTO state.heads (tenant_id, object_key, head_version)
		VALUES ($1, $2, 1)
		ON CONFLICT (tenant_id, object_key) DO UPDATE
		SET head_version = state.heads.head_version + 1,
		    updated_at = now()
		RETURNING head_version
	`, obj.TenantID, obj.ObjectKey).Scan(&version)
	if err != nil {
		return nil, err
	}

	var createdAt time.Time
	err = tx.QueryRow(ctx, `
		INSERT INTO state.objects (
		  tenant_id, object_key, version, tool, project, env, resource_scope,
		  content_json, content_bytes, etag, actor
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9,$10,$11)
		RETURNING created_at
	`,
		obj.TenantID, obj.ObjectKey, version, obj.Tool, nullIfEmpty(obj.Project), nullIfEmpty(obj.Env), nullIfEmpty(obj.ResourceScope),
		string(obj.ContentJSON), bytesOrNil(obj.ContentBytes), nullIfEmpty(obj.ETag), nullIfEmpty(obj.Actor),
	).Scan(&createdAt)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	obj.Version = version
	obj.CreatedAt = createdAt
	return &obj, nil
}

func (s *Store) GetStateObject(ctx context.Context, tenantID string, objectKey string, version int64) (*StateObject, error) {
	tenantID = normalizeTenantID(tenantID)
	if objectKey == "" {
		return nil, fmt.Errorf("missing object key")
	}

	query := `
		SELECT tenant_id, object_key, version, COALESCE(tool,''), COALESCE(project,''), COALESCE(env,''),
		       COALESCE(resource_scope,''), content_json::text, content_bytes, COALESCE(etag,''), COALESCE(actor,''), created_at
		FROM state.objects
		WHERE tenant_id = $1 AND object_key = $2
	`
	args := []any{tenantID, objectKey}
	if version > 0 {
		query += ` AND version = $3`
		args = append(args, version)
	} else {
		query += ` ORDER BY version DESC LIMIT 1`
	}

	var obj StateObject
	var contentJSON string
	err := s.pool.QueryRow(ctx, query, args...).Scan(
		&obj.TenantID, &obj.ObjectKey, &obj.Version, &obj.Tool, &obj.Project, &obj.Env,
		&obj.ResourceScope, &contentJSON, &obj.ContentBytes, &obj.ETag, &obj.Actor, &obj.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	obj.ContentJSON = []byte(contentJSON)
	return &obj, nil
}

func (s *Store) AcquireStateLock(ctx context.Context, lock StateLock) (*StateLock, error) {
	lock.TenantID = normalizeTenantID(lock.TenantID)
	if lock.ObjectKey == "" {
		return nil, fmt.Errorf("missing object key")
	}
	if lock.LockID == "" {
		lock.LockID = uuid.NewString()
	}
	if lock.Owner == "" {
		lock.Owner = "unknown"
	}
	if lock.ExpiresAt.IsZero() {
		lock.ExpiresAt = time.Now().UTC().Add(15 * time.Minute)
	}

	var out StateLock
	err := s.pool.QueryRow(ctx, `
		WITH cleared AS (
		  DELETE FROM state.locks
		  WHERE tenant_id = $1 AND object_key = $2 AND expires_at <= now()
		)
		INSERT INTO state.locks (tenant_id, object_key, lock_id, owner, expires_at)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (tenant_id, object_key) DO NOTHING
		RETURNING tenant_id, object_key, lock_id, owner, created_at, expires_at
	`, lock.TenantID, lock.ObjectKey, lock.LockID, lock.Owner, lock.ExpiresAt).Scan(
		&out.TenantID, &out.ObjectKey, &out.LockID, &out.Owner, &out.CreatedAt, &out.ExpiresAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("state object is locked: %s", lock.ObjectKey)
		}
		return nil, err
	}
	return &out, nil
}

func (s *Store) ReleaseStateLock(ctx context.Context, tenantID string, objectKey string, lockID string) error {
	tenantID = normalizeTenantID(tenantID)
	if objectKey == "" {
		return fmt.Errorf("missing object key")
	}
	cmd, err := s.pool.Exec(ctx, `
		DELETE FROM state.locks
		WHERE tenant_id = $1
		  AND object_key = $2
		  AND ($3 = '' OR lock_id = $3)
	`, tenantID, objectKey, lockID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("lock not found for object key: %s", objectKey)
	}
	return nil
}

func (s *Store) CreateChangeSet(ctx context.Context, cs ChangeSet) (string, error) {
	cs.TenantID = normalizeTenantID(cs.TenantID)
	if cs.ChangeSetID == "" {
		cs.ChangeSetID = uuid.NewString()
	}
	if cs.Status == "" {
		cs.Status = "planned"
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO ops.change_sets (
		  tenant_id, change_set_id, project, env, phase, status, actor, summary, inputs, plan, result
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,$10::jsonb,$11::jsonb)
	`, cs.TenantID, cs.ChangeSetID, cs.Project, cs.Env, cs.Phase, cs.Status, nullIfEmpty(cs.Actor), nullIfEmpty(cs.Summary),
		jsonOrEmpty(cs.InputsJSON), jsonOrEmpty(cs.PlanJSON), jsonOrEmpty(cs.ResultJSON),
	)
	if err != nil {
		return "", err
	}
	return cs.ChangeSetID, nil
}

func (s *Store) ChangeSetExists(ctx context.Context, tenantID string, changeSetID string) (bool, error) {
	tenantID = normalizeTenantID(tenantID)
	if changeSetID == "" {
		return false, nil
	}
	var exists bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM ops.change_sets WHERE tenant_id = $1 AND change_set_id = $2)
	`, tenantID, changeSetID).Scan(&exists)
	return exists, err
}

func (s *Store) UpsertResourceCurrent(ctx context.Context, rec ResourceRecord) error {
	rec.TenantID = normalizeTenantID(rec.TenantID)
	if rec.ResourceUID == "" {
		return fmt.Errorf("missing resource uid")
	}
	if rec.ResourceType == "" {
		return fmt.Errorf("missing resource type")
	}
	if len(rec.LabelsJSON) == 0 {
		rec.LabelsJSON = []byte("{}")
	}
	if len(rec.DesiredStateJSON) == 0 {
		rec.DesiredStateJSON = []byte("{}")
	}
	if len(rec.ObservedStateJSON) == 0 {
		rec.ObservedStateJSON = []byte("{}")
	}
	if rec.DriftStatus == "" {
		rec.DriftStatus = "unknown"
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO inventory.resources_current (
		  tenant_id, resource_uid, project, resource_type, cloud, region, env, engine, provider,
		  external_id, name, labels, desired_state, observed_state, drift_status,
		  state_object_key, last_change_set_id, last_seen_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12::jsonb,$13::jsonb,$14::jsonb,$15,$16,$17,$18)
		ON CONFLICT (tenant_id, resource_uid) DO UPDATE SET
		  project = EXCLUDED.project,
		  resource_type = EXCLUDED.resource_type,
		  cloud = EXCLUDED.cloud,
		  region = EXCLUDED.region,
		  env = EXCLUDED.env,
		  engine = EXCLUDED.engine,
		  provider = EXCLUDED.provider,
		  external_id = EXCLUDED.external_id,
		  name = EXCLUDED.name,
		  labels = EXCLUDED.labels,
		  desired_state = EXCLUDED.desired_state,
		  observed_state = EXCLUDED.observed_state,
		  drift_status = EXCLUDED.drift_status,
		  state_object_key = EXCLUDED.state_object_key,
		  last_change_set_id = EXCLUDED.last_change_set_id,
		  last_seen_at = EXCLUDED.last_seen_at,
		  updated_at = now()
	`, rec.TenantID, rec.ResourceUID, rec.Project, rec.ResourceType, rec.Cloud, rec.Region, rec.Env, rec.Engine, rec.Provider,
		nullIfEmpty(rec.ExternalID), rec.Name, string(rec.LabelsJSON), string(rec.DesiredStateJSON), string(rec.ObservedStateJSON),
		rec.DriftStatus, nullIfEmpty(rec.StateObjectKey), nullIfEmpty(rec.LastChangeSetID), rec.LastSeenAt)
	return err
}

func (s *Store) InsertResourceEvent(ctx context.Context, evt ResourceEvent) error {
	evt.TenantID = normalizeTenantID(evt.TenantID)
	if evt.ResourceUID == "" {
		return fmt.Errorf("missing resource uid")
	}
	if evt.EventType == "" {
		return fmt.Errorf("missing event type")
	}
	if evt.EventID == "" {
		evt.EventID = uuid.NewString()
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO inventory.resource_events (
		  tenant_id, event_id, resource_uid, change_set_id, event_type, diff, message
		)
		VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7)
	`, evt.TenantID, evt.EventID, evt.ResourceUID, nullIfEmpty(evt.ChangeSetID), evt.EventType, jsonOrEmpty(evt.DiffJSON), nullIfEmpty(evt.Message))
	return err
}

func bytesOrNil(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}

func normalizeTenantID(tenantID string) string {
	if tenantID == "" {
		return defaults.TenantID()
	}
	return tenantID
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func jsonOrEmpty(b []byte) string {
	if len(b) == 0 {
		return "{}"
	}
	return string(b)
}

func jsonOrArray(b []byte) string {
	if len(b) == 0 {
		return "[]"
	}
	return string(b)
}
