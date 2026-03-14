package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"xcloudflow/internal/automation"
	"xcloudflow/internal/defaults"
	"xcloudflow/internal/store"
)

type terraformToolInput struct {
	Module     string         `json:"module"`
	WorkingDir string         `json:"working_dir"`
	VarsFile   string         `json:"vars_file"`
	Vars       map[string]any `json:"vars"`
	Workspace  string         `json:"workspace"`
	Confirm    string         `json:"confirm"`
	ChangeRef  string         `json:"change_ref"`
}

type ansibleToolInput struct {
	Playbook  string         `json:"playbook"`
	Inventory string         `json:"inventory"`
	Limit     string         `json:"limit"`
	ExtraVars map[string]any `json:"extra_vars"`
	Check     bool           `json:"check"`
	Confirm   string         `json:"confirm"`
	ChangeRef string         `json:"change_ref"`
}

type ansibleAdhocInput struct {
	Inventory string `json:"inventory"`
	Target    string `json:"target"`
	Module    string `json:"module"`
	Args      string `json:"args"`
	Check     bool   `json:"check"`
	Confirm   string `json:"confirm"`
	ChangeRef string `json:"change_ref"`
}

type sshExecInput struct {
	Target     string            `json:"target"`
	Command    string            `json:"command"`
	Cwd        string            `json:"cwd"`
	Env        map[string]string `json:"env"`
	TimeoutSec int               `json:"timeout_sec"`
	Confirm    string            `json:"confirm"`
	ChangeRef  string            `json:"change_ref"`
}

func (s *Server) callTerraformDomain(ctx context.Context, action string, args json.RawMessage) (any, error) {
	var in terraformToolInput
	if err := json.Unmarshal(args, &in); err != nil {
		return nil, fmt.Errorf("invalid args: %w", err)
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
			Confirm:   in.Confirm,
			ChangeRef: in.ChangeRef,
		},
	})
	out[action] = runRes
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
			Confirm:   in.Confirm,
			ChangeRef: in.ChangeRef,
		},
	})
	out["result"] = runRes
	return finalizeExecutionResult(out, runErr)
}

func (s *Server) callAnsibleDomain(ctx context.Context, check bool, args json.RawMessage) (any, error) {
	var in ansibleToolInput
	if err := json.Unmarshal(args, &in); err != nil {
		return nil, fmt.Errorf("invalid args: %w", err)
	}
	runRes, runErr := automation.RunAnsiblePlaybook(ctx, check, automation.AnsibleOptions{
		Playbook:  in.Playbook,
		Inventory: in.Inventory,
		Limit:     in.Limit,
		ExtraVars: in.ExtraVars,
		Gate: automation.Gate{
			Confirm:   in.Confirm,
			ChangeRef: in.ChangeRef,
		},
	})
	return finalizeExecutionResult(runRes, runErr)
}

func (s *Server) callAnsiblePlaybook(ctx context.Context, args json.RawMessage) (any, error) {
	var in ansibleToolInput
	if err := json.Unmarshal(args, &in); err != nil {
		return nil, fmt.Errorf("invalid args: %w", err)
	}
	runRes, runErr := automation.RunAnsiblePlaybook(ctx, in.Check, automation.AnsibleOptions{
		Playbook:  in.Playbook,
		Inventory: in.Inventory,
		Limit:     in.Limit,
		ExtraVars: in.ExtraVars,
		Gate: automation.Gate{
			Confirm:   in.Confirm,
			ChangeRef: in.ChangeRef,
		},
	})
	return finalizeExecutionResult(runRes, runErr)
}

func (s *Server) callAnsibleAdhoc(ctx context.Context, args json.RawMessage) (any, error) {
	var in ansibleAdhocInput
	if err := json.Unmarshal(args, &in); err != nil {
		return nil, fmt.Errorf("invalid args: %w", err)
	}
	runRes, runErr := automation.RunAnsibleAdhoc(ctx, in.Check, in.Inventory, in.Target, in.Module, in.Args, automation.Gate{
		Confirm:   in.Confirm,
		ChangeRef: in.ChangeRef,
	})
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
			Confirm:   in.Confirm,
			ChangeRef: in.ChangeRef,
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

func findMCPServerByName(servers []store.MCPServer, name string) *store.MCPServer {
	for i := range servers {
		if servers[i].Name == name {
			return &servers[i]
		}
	}
	return nil
}
