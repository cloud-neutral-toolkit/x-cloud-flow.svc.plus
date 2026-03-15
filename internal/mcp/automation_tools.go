package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"strings"
	"time"

	"xcloudflow/internal/automation"
	"xcloudflow/internal/defaults"
	"xcloudflow/internal/store"
)

type terraformToolInput struct {
	Module      string         `json:"module"`
	WorkingDir  string         `json:"working_dir"`
	VarsFile    string         `json:"vars_file"`
	Vars        map[string]any `json:"vars"`
	Workspace   string         `json:"workspace"`
	Confirm     string         `json:"confirm"`
	ChangeRef   string         `json:"change_ref"`
	ChangeSetID string         `json:"change_set_id"`
	TenantID    string         `json:"tenant_id"`
}

type ansibleToolInput struct {
	Playbook    string         `json:"playbook"`
	Inventory   string         `json:"inventory"`
	Limit       string         `json:"limit"`
	ExtraVars   map[string]any `json:"extra_vars"`
	Check       bool           `json:"check"`
	Confirm     string         `json:"confirm"`
	ChangeRef   string         `json:"change_ref"`
	ChangeSetID string         `json:"change_set_id"`
	TenantID    string         `json:"tenant_id"`
}

type ansibleAdhocInput struct {
	Inventory   string `json:"inventory"`
	Target      string `json:"target"`
	Module      string `json:"module"`
	Args        string `json:"args"`
	Check       bool   `json:"check"`
	Confirm     string `json:"confirm"`
	ChangeRef   string `json:"change_ref"`
	ChangeSetID string `json:"change_set_id"`
	TenantID    string `json:"tenant_id"`
}

type sshExecInput struct {
	Target      string            `json:"target"`
	Command     string            `json:"command"`
	Cwd         string            `json:"cwd"`
	Env         map[string]string `json:"env"`
	TimeoutSec  int               `json:"timeout_sec"`
	Confirm     string            `json:"confirm"`
	ChangeRef   string            `json:"change_ref"`
	ChangeSetID string            `json:"change_set_id"`
	TenantID    string            `json:"tenant_id"`
}

func (s *Server) callTerraformDomain(ctx context.Context, action string, args json.RawMessage) (any, error) {
	var in terraformToolInput
	if err := json.Unmarshal(args, &in); err != nil {
		return nil, fmt.Errorf("invalid args: %w", err)
	}
	if err := s.ensureChangeSetReference(ctx, in.TenantID, in.ChangeSetID); err != nil {
		return nil, err
	}

	prepared, err := automation.PrepareTerraformWorkingDir(s.workspaceDir, automation.TerraformOptions{
		WorkingDir: in.WorkingDir,
		Module:     in.Module,
	})
	if err != nil {
		return nil, err
	}

	out := map[string]any{
		"source_dir":  prepared["source_dir"],
		"working_dir": prepared["working_dir"],
		"action":      action,
	}

	initRes, initErr := automation.RunTerraform(ctx, "init", automation.TerraformOptions{
		WorkingDir: prepared["working_dir"],
	})
	out["init"] = initRes
	if initErr != nil {
		return finalizeExecutionResult(out, initErr)
	}

	runRes, runErr := automation.RunTerraform(ctx, action, automation.TerraformOptions{
		WorkingDir: prepared["working_dir"],
		VarsFile:   in.VarsFile,
		Vars:       in.Vars,
		Workspace:  in.Workspace,
		Gate: automation.Gate{
			Confirm:     in.Confirm,
			ChangeRef:   in.ChangeRef,
			ChangeSetID: in.ChangeSetID,
		},
	})
	out[action] = runRes
	s.persistTerraformExecution(ctx, out, action, prepared["working_dir"], prepared["source_dir"], in, runRes)
	return finalizeExecutionResult(out, runErr)
}

func (s *Server) callTerraformRaw(ctx context.Context, action string, args json.RawMessage) (any, error) {
	var in terraformToolInput
	if err := json.Unmarshal(args, &in); err != nil {
		return nil, fmt.Errorf("invalid args: %w", err)
	}
	if strings.TrimSpace(in.WorkingDir) == "" {
		return nil, fmt.Errorf("missing working_dir")
	}
	if err := s.ensureChangeSetReference(ctx, in.TenantID, in.ChangeSetID); err != nil {
		return nil, err
	}

	out := map[string]any{
		"working_dir": in.WorkingDir,
		"action":      action,
	}
	runRes, runErr := automation.RunTerraform(ctx, action, automation.TerraformOptions{
		WorkingDir: in.WorkingDir,
		VarsFile:   in.VarsFile,
		Vars:       in.Vars,
		Workspace:  in.Workspace,
		Gate: automation.Gate{
			Confirm:     in.Confirm,
			ChangeRef:   in.ChangeRef,
			ChangeSetID: in.ChangeSetID,
		},
	})
	out["result"] = runRes
	s.persistTerraformExecution(ctx, out, action, in.WorkingDir, in.WorkingDir, in, runRes)
	return finalizeExecutionResult(out, runErr)
}

func (s *Server) callAnsibleDomain(ctx context.Context, check bool, args json.RawMessage) (any, error) {
	var in ansibleToolInput
	if err := json.Unmarshal(args, &in); err != nil {
		return nil, fmt.Errorf("invalid args: %w", err)
	}
	if err := s.ensureChangeSetReference(ctx, in.TenantID, in.ChangeSetID); err != nil {
		return nil, err
	}
	runRes, runErr := automation.RunAnsiblePlaybook(ctx, check, automation.AnsibleOptions{
		Playbook:  in.Playbook,
		Inventory: in.Inventory,
		Limit:     in.Limit,
		ExtraVars: in.ExtraVars,
		Gate: automation.Gate{
			Confirm:     in.Confirm,
			ChangeRef:   in.ChangeRef,
			ChangeSetID: in.ChangeSetID,
		},
	})
	s.persistAnsiblePlaybookExecution(ctx, runRes, in, check)
	return finalizeExecutionResult(runRes, runErr)
}

func (s *Server) callAnsiblePlaybook(ctx context.Context, args json.RawMessage) (any, error) {
	var in ansibleToolInput
	if err := json.Unmarshal(args, &in); err != nil {
		return nil, fmt.Errorf("invalid args: %w", err)
	}
	if err := s.ensureChangeSetReference(ctx, in.TenantID, in.ChangeSetID); err != nil {
		return nil, err
	}
	runRes, runErr := automation.RunAnsiblePlaybook(ctx, in.Check, automation.AnsibleOptions{
		Playbook:  in.Playbook,
		Inventory: in.Inventory,
		Limit:     in.Limit,
		ExtraVars: in.ExtraVars,
		Gate: automation.Gate{
			Confirm:     in.Confirm,
			ChangeRef:   in.ChangeRef,
			ChangeSetID: in.ChangeSetID,
		},
	})
	s.persistAnsiblePlaybookExecution(ctx, runRes, in, in.Check)
	return finalizeExecutionResult(runRes, runErr)
}

func (s *Server) callAnsibleAdhoc(ctx context.Context, args json.RawMessage) (any, error) {
	var in ansibleAdhocInput
	if err := json.Unmarshal(args, &in); err != nil {
		return nil, fmt.Errorf("invalid args: %w", err)
	}
	if err := s.ensureChangeSetReference(ctx, in.TenantID, in.ChangeSetID); err != nil {
		return nil, err
	}
	runRes, runErr := automation.RunAnsibleAdhoc(ctx, in.Check, in.Inventory, in.Target, in.Module, in.Args, automation.Gate{
		Confirm:     in.Confirm,
		ChangeRef:   in.ChangeRef,
		ChangeSetID: in.ChangeSetID,
	})
	s.persistAnsibleAdhocExecution(ctx, runRes, in)
	return finalizeExecutionResult(runRes, runErr)
}

func (s *Server) callEdgeSSH(ctx context.Context, args json.RawMessage) (any, error) {
	var in sshExecInput
	if err := json.Unmarshal(args, &in); err != nil {
		return nil, fmt.Errorf("invalid args: %w", err)
	}
	if strings.TrimSpace(in.Target) == "" || strings.TrimSpace(in.Command) == "" {
		return nil, fmt.Errorf("missing target or command")
	}
	if automation.LooksLikeMutatingCommand(in.Command) {
		if err := automation.RequireApplyGate(automation.Gate{
			Confirm:     in.Confirm,
			ChangeRef:   in.ChangeRef,
			ChangeSetID: in.ChangeSetID,
		}); err != nil {
			return nil, err
		}
	}

	baseURL, bearerToken, err := s.edgeSSHServerConfig(ctx)
	if err != nil {
		return nil, err
	}
	client := NewClientWithOptions(baseURL, ClientOptions{BearerToken: bearerToken})
	tools, err := client.ToolsList(ctx)
	if err != nil {
		return finalizeExecutionResult(map[string]any{
			"server_name": defaults.DefaultSSHMCPServerName,
			"server_url":  baseURL,
		}, err)
	}

	tool, err := resolveEdgeSSHRemoteTool(tools)
	if err != nil {
		return nil, err
	}
	callArgs := buildEdgeSSHArgs(tool, in)
	result, callErr := client.ToolsCall(ctx, tool.Name, callArgs)
	return finalizeExecutionResult(map[string]any{
		"server_name": defaults.DefaultSSHMCPServerName,
		"server_url":  baseURL,
		"remote_tool": tool.Name,
		"arguments":   callArgs,
		"result":      result,
	}, callErr)
}

func (s *Server) edgeSSHServerConfig(ctx context.Context) (string, string, error) {
	if s.store != nil {
		srvs, err := s.store.ListMCPServers(ctx)
		if err != nil {
			return "", "", err
		}
		for _, srv := range srvs {
			if srv.Name == defaults.DefaultSSHMCPServerName && srv.Enabled {
				token := ""
				if strings.EqualFold(srv.AuthType, "bearer") {
					token = defaults.SSHMCPBearerToken()
				}
				return srv.BaseURL, token, nil
			}
		}
	}
	if url := defaults.SSHMCPURL(); url != "" {
		return url, defaults.SSHMCPBearerToken(), nil
	}
	return "", "", fmt.Errorf("edge ssh mcp server is not configured")
}

func resolveEdgeSSHRemoteTool(tools []Tool) (Tool, error) {
	if wanted := defaults.SSHMCPToolName(); wanted != "" {
		for _, tool := range tools {
			if tool.Name == wanted {
				return tool, nil
			}
		}
	}
	preferred := []string{"ssh_execute", "ssh.exec", "edge.ssh.exec", "ssh.execute"}
	for _, name := range preferred {
		for _, tool := range tools {
			if tool.Name == name {
				return tool, nil
			}
		}
	}
	for _, tool := range tools {
		lower := strings.ToLower(tool.Name)
		if strings.Contains(lower, "ssh") && (strings.Contains(lower, "exec") || strings.Contains(lower, "execute")) {
			return tool, nil
		}
	}
	return Tool{}, fmt.Errorf("no compatible edge ssh tool found on external MCP server")
}

func buildEdgeSSHArgs(tool Tool, in sshExecInput) map[string]any {
	props := schemaProperties(tool.InputSchema)
	args := map[string]any{}
	setFirstPresent(args, props, []string{"server", "target", "host", "node"}, in.Target)
	setFirstPresent(args, props, []string{"command", "cmd"}, in.Command)
	if strings.TrimSpace(in.Cwd) != "" {
		setFirstPresent(args, props, []string{"cwd", "working_dir"}, in.Cwd)
	}
	if len(in.Env) > 0 {
		setFirstPresent(args, props, []string{"env"}, in.Env)
	}
	if in.TimeoutSec > 0 {
		if _, ok := props["timeout_sec"]; ok {
			args["timeout_sec"] = in.TimeoutSec
		} else if _, ok := props["timeout_ms"]; ok {
			args["timeout_ms"] = in.TimeoutSec * 1000
		} else {
			args["timeout"] = in.TimeoutSec * 1000
		}
	}
	return args
}

func schemaProperties(raw json.RawMessage) map[string]json.RawMessage {
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		return map[string]json.RawMessage{}
	}
	if schema.Properties == nil {
		return map[string]json.RawMessage{}
	}
	return schema.Properties
}

func setFirstPresent(target map[string]any, properties map[string]json.RawMessage, keys []string, value any) {
	if value == nil {
		return
	}
	for _, key := range keys {
		if len(properties) == 0 {
			target[keys[0]] = value
			return
		}
		if _, ok := properties[key]; ok {
			target[key] = value
			return
		}
	}
}

func finalizeExecutionResult(out map[string]any, err error) (map[string]any, error) {
	out["ok"] = err == nil
	if err != nil {
		out["error"] = err.Error()
	}
	return out, nil
}

func (s *Server) ensureChangeSetReference(ctx context.Context, tenantID string, changeSetID string) error {
	if strings.TrimSpace(changeSetID) == "" || s.store == nil {
		return nil
	}
	exists, err := s.store.ChangeSetExists(ctx, tenantID, changeSetID)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("unknown change_set_id: %s", changeSetID)
	}
	return nil
}

func (s *Server) persistTerraformExecution(ctx context.Context, out map[string]any, action string, workingDir string, sourceDir string, in terraformToolInput, runRes map[string]any) {
	if s.store == nil {
		return
	}
	statePath := filepath.Join(workingDir, "terraform.tfstate")
	records, err := terraformRecordsFromStateFile(statePath, in, action)
	if err != nil || len(records) == 0 {
		record := syntheticTerraformRecord(in, action, workingDir, sourceDir, runRes)
		records = []store.ResourceRecord{record}
	}

	var persisted []string
	for _, rec := range records {
		if err := s.store.UpsertResourceCurrent(ctx, rec); err == nil {
			persisted = append(persisted, rec.ResourceUID)
			_ = s.store.InsertResourceEvent(ctx, store.ResourceEvent{
				TenantID:    in.TenantID,
				ResourceUID: rec.ResourceUID,
				ChangeSetID: in.ChangeSetID,
				EventType:   "terraform_" + action,
				DiffJSON:    rec.ObservedStateJSON,
				Message:     fmt.Sprintf("terraform %s completed", action),
			})
		}
	}
	if len(persisted) > 0 {
		out["inventory"] = map[string]any{
			"resource_uids": persisted,
			"change_set_id": in.ChangeSetID,
			"tenant_id":     normalizedTenantID(in.TenantID),
		}
	}
}

func (s *Server) persistAnsiblePlaybookExecution(ctx context.Context, runRes map[string]any, in ansibleToolInput, check bool) {
	if s.store == nil {
		return
	}
	rec, evt := syntheticAnsiblePlaybookRecord(in, runRes, check)
	if err := s.store.UpsertResourceCurrent(ctx, rec); err == nil {
		_ = s.store.InsertResourceEvent(ctx, evt)
	}
}

func (s *Server) persistAnsibleAdhocExecution(ctx context.Context, runRes map[string]any, in ansibleAdhocInput) {
	if s.store == nil {
		return
	}
	rec, evt := syntheticAnsibleAdhocRecord(in, runRes)
	if err := s.store.UpsertResourceCurrent(ctx, rec); err == nil {
		_ = s.store.InsertResourceEvent(ctx, evt)
	}
}

type terraformState struct {
	Resources []struct {
		Mode      string `json:"mode"`
		Type      string `json:"type"`
		Name      string `json:"name"`
		Provider  string `json:"provider"`
		Instances []struct {
			Attributes map[string]any `json:"attributes"`
		} `json:"instances"`
	} `json:"resources"`
}

func terraformRecordsFromStateFile(path string, in terraformToolInput, action string) ([]store.ResourceRecord, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var state terraformState
	if err := json.Unmarshal(content, &state); err != nil {
		return nil, err
	}

	records := make([]store.ResourceRecord, 0)
	for _, res := range state.Resources {
		for idx, inst := range res.Instances {
			attrs := inst.Attributes
			if attrs == nil {
				attrs = map[string]any{}
			}
			externalID, _ := attrs["id"].(string)
			name := firstString(attrs, "name", "tags.Name", "tags_all.Name")
			if name == "" {
				name = fmt.Sprintf("%s.%s[%d]", res.Type, res.Name, idx)
			}
			now := time.Now().UTC()
			observed, _ := json.Marshal(attrs)
			uid := stableResourceUID(normalizedTenantID(in.TenantID), "tf", externalID, in.Module, in.WorkingDir, res.Type, res.Name, idx)
			records = append(records, store.ResourceRecord{
				TenantID:          normalizedTenantID(in.TenantID),
				ResourceUID:       uid,
				ResourceType:      res.Type,
				Cloud:             cloudFromProvider(res.Provider),
				Region:            fmt.Sprint(in.Vars["region"]),
				Env:               fmt.Sprint(in.Vars["env"]),
				Engine:            "terraform",
				Provider:          res.Provider,
				ExternalID:        externalID,
				Name:              name,
				LabelsJSON:        []byte("{}"),
				DesiredStateJSON:  observed,
				ObservedStateJSON: observed,
				DriftStatus:       "in_sync",
				StateObjectKey:    in.WorkingDir,
				LastChangeSetID:   in.ChangeSetID,
				LastSeenAt:        &now,
			})
		}
	}
	return records, nil
}

func syntheticTerraformRecord(in terraformToolInput, action string, workingDir string, sourceDir string, runRes map[string]any) store.ResourceRecord {
	payload, _ := json.Marshal(map[string]any{
		"action":     action,
		"module":     in.Module,
		"workingDir": workingDir,
		"sourceDir":  sourceDir,
		"result":     runRes,
	})
	now := time.Now().UTC()
	return store.ResourceRecord{
		TenantID:          normalizedTenantID(in.TenantID),
		ResourceUID:       stableResourceUID(normalizedTenantID(in.TenantID), "tf-module", in.Module, workingDir),
		ResourceType:      "terraform_module",
		Cloud:             fmt.Sprint(in.Vars["cloud"]),
		Region:            fmt.Sprint(in.Vars["region"]),
		Env:               fmt.Sprint(in.Vars["env"]),
		Engine:            "terraform",
		Provider:          "terraform",
		Name:              firstNonEmptyString(in.Module, filepath.Base(workingDir)),
		LabelsJSON:        []byte("{}"),
		DesiredStateJSON:  payload,
		ObservedStateJSON: payload,
		DriftStatus:       "unknown",
		StateObjectKey:    workingDir,
		LastChangeSetID:   in.ChangeSetID,
		LastSeenAt:        &now,
	}
}

func syntheticAnsiblePlaybookRecord(in ansibleToolInput, runRes map[string]any, check bool) (store.ResourceRecord, store.ResourceEvent) {
	mode := "apply"
	eventType := "ansible_apply"
	if check {
		mode = "check"
		eventType = "ansible_check"
	}
	payload, _ := json.Marshal(map[string]any{
		"playbook":  in.Playbook,
		"inventory": in.Inventory,
		"limit":     in.Limit,
		"mode":      mode,
		"result":    runRes,
	})
	now := time.Now().UTC()
	uid := stableResourceUID(normalizedTenantID(in.TenantID), "ansible-playbook", in.Playbook, in.Inventory, in.Limit)
	return store.ResourceRecord{
			TenantID:          normalizedTenantID(in.TenantID),
			ResourceUID:       uid,
			ResourceType:      "ansible_playbook",
			Engine:            "ansible",
			Provider:          "ansible",
			Name:              in.Playbook,
			LabelsJSON:        []byte("{}"),
			DesiredStateJSON:  payload,
			ObservedStateJSON: payload,
			DriftStatus:       "unknown",
			StateObjectKey:    in.Playbook,
			LastChangeSetID:   in.ChangeSetID,
			LastSeenAt:        &now,
		}, store.ResourceEvent{
			TenantID:    normalizedTenantID(in.TenantID),
			ResourceUID: uid,
			ChangeSetID: in.ChangeSetID,
			EventType:   eventType,
			DiffJSON:    payload,
			Message:     fmt.Sprintf("ansible %s completed for %s", mode, in.Playbook),
		}
}

func syntheticAnsibleAdhocRecord(in ansibleAdhocInput, runRes map[string]any) (store.ResourceRecord, store.ResourceEvent) {
	payload, _ := json.Marshal(map[string]any{
		"target":    in.Target,
		"module":    in.Module,
		"args":      in.Args,
		"inventory": in.Inventory,
		"result":    runRes,
	})
	now := time.Now().UTC()
	uid := stableResourceUID(normalizedTenantID(in.TenantID), "ansible-adhoc", in.Target, in.Module, in.Args)
	return store.ResourceRecord{
			TenantID:          normalizedTenantID(in.TenantID),
			ResourceUID:       uid,
			ResourceType:      "ansible_adhoc",
			Engine:            "ansible",
			Provider:          "ansible",
			Name:              in.Target,
			LabelsJSON:        []byte("{}"),
			DesiredStateJSON:  payload,
			ObservedStateJSON: payload,
			DriftStatus:       "unknown",
			StateObjectKey:    in.Target,
			LastChangeSetID:   in.ChangeSetID,
			LastSeenAt:        &now,
		}, store.ResourceEvent{
			TenantID:    normalizedTenantID(in.TenantID),
			ResourceUID: uid,
			ChangeSetID: in.ChangeSetID,
			EventType:   "ansible_adhoc",
			DiffJSON:    payload,
			Message:     fmt.Sprintf("ansible adhoc completed for %s", in.Target),
		}
}

func stableResourceUID(parts ...any) string {
	h := fnv.New64a()
	for _, part := range parts {
		_, _ = h.Write([]byte(fmt.Sprint(part)))
		_, _ = h.Write([]byte{0})
	}
	return fmt.Sprintf("res-%x", h.Sum64())
}

func cloudFromProvider(provider string) string {
	p := strings.ToLower(provider)
	switch {
	case strings.Contains(p, "aws"):
		return "aws"
	case strings.Contains(p, "azurerm"), strings.Contains(p, "azure"):
		return "azure"
	case strings.Contains(p, "google"), strings.Contains(p, "gcp"):
		return "gcp"
	case strings.Contains(p, "alicloud"), strings.Contains(p, "aliyun"):
		return "aliyun"
	default:
		return ""
	}
}

func firstString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := lookupNestedValue(m, key).(string); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func lookupNestedValue(m map[string]any, path string) any {
	current := any(m)
	for _, part := range strings.Split(path, ".") {
		next, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = next[part]
	}
	return current
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func normalizedTenantID(tenantID string) string {
	if strings.TrimSpace(tenantID) == "" {
		return defaults.TenantID()
	}
	return tenantID
}

func findMCPServerByName(servers []store.MCPServer, name string) *store.MCPServer {
	for i := range servers {
		if servers[i].Name == name {
			return &servers[i]
		}
	}
	return nil
}
