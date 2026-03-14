package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"xcloudflow/internal/codex"
	"xcloudflow/internal/defaults"
	"xcloudflow/internal/openclaw"
	"xcloudflow/internal/stackflow"
)

func (s *Server) callTool(ctx context.Context, name string, args json.RawMessage) (any, error) {
	switch name {
	case "stackflow.validate":
		var in struct {
			ConfigYAML string `json:"config_yaml"`
			Env        string `json:"env"`
		}
		if err := json.Unmarshal(args, &in); err != nil || in.ConfigYAML == "" {
			return nil, fmt.Errorf("missing config_yaml")
		}
		cfg, err := stackflow.LoadYAML([]byte(in.ConfigYAML))
		if err != nil {
			return nil, err
		}
		if in.Env != "" {
			cfg = stackflow.ApplyEnvOverrides(cfg, in.Env)
		}
		return stackflow.Validate(cfg)

	case "stackflow.plan.dns":
		var in struct {
			ConfigYAML string `json:"config_yaml"`
			Env        string `json:"env"`
		}
		if err := json.Unmarshal(args, &in); err != nil || in.ConfigYAML == "" {
			return nil, fmt.Errorf("missing config_yaml")
		}
		cfg, err := stackflow.LoadYAML([]byte(in.ConfigYAML))
		if err != nil {
			return nil, err
		}
		return stackflow.DNSPlan(cfg, in.Env)

	case "stackflow.plan.iac":
		var in struct {
			ConfigYAML string `json:"config_yaml"`
			Env        string `json:"env"`
		}
		if err := json.Unmarshal(args, &in); err != nil || in.ConfigYAML == "" {
			return nil, fmt.Errorf("missing config_yaml")
		}
		cfg, err := stackflow.LoadYAML([]byte(in.ConfigYAML))
		if err != nil {
			return nil, err
		}
		return stackflow.IACPlan(cfg, in.Env)

	case "stackflow.codex.manifest", "agent.codex.manifest":
		return s.callCodexManifest(args)

	case "stackflow.openclaw.registration", "agent.openclaw.patch":
		return s.callOpenClawPatch(args)

	case "iac.terraform.plan":
		return s.callTerraformDomain(ctx, "plan", args)
	case "iac.terraform.apply":
		return s.callTerraformDomain(ctx, "apply", args)

	case "config.ansible.check":
		return s.callAnsibleDomain(ctx, true, args)
	case "config.ansible.apply":
		return s.callAnsibleDomain(ctx, false, args)

	case "edge.ssh.exec":
		return s.callEdgeSSH(ctx, args)

	case "terraform.init":
		return s.callTerraformRaw(ctx, "init", args)
	case "terraform.plan":
		return s.callTerraformRaw(ctx, "plan", args)
	case "terraform.apply":
		return s.callTerraformRaw(ctx, "apply", args)
	case "terraform.destroy":
		return s.callTerraformRaw(ctx, "destroy", args)

	case "ansible.playbook":
		return s.callAnsiblePlaybook(ctx, args)
	case "ansible.adhoc":
		return s.callAnsibleAdhoc(ctx, args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

func (s *Server) callCodexManifest(args json.RawMessage) (any, error) {
	var in struct {
		Task       string `json:"task"`
		ConfigYAML string `json:"config_yaml"`
		ConfigPath string `json:"config_path"`
		Env        string `json:"env"`
		MCPURL     string `json:"mcp_url"`
		Workspace  string `json:"workspace"`
	}
	if err := json.Unmarshal(args, &in); err != nil && len(args) > 0 {
		return nil, fmt.Errorf("invalid args: %w", err)
	}

	workspace := in.Workspace
	if workspace == "" {
		workspace = s.workspaceDir
	}
	workspace = defaults.WorkspaceDir(workspace)

	mcpURL := in.MCPURL
	if mcpURL == "" {
		mcpURL = s.codex.MCPURL
	}
	cfg := codex.DefaultBridgeConfig(workspace, mcpURL)

	var planJSON []byte
	if in.ConfigYAML != "" {
		stackCfg, err := stackflow.LoadYAML([]byte(in.ConfigYAML))
		if err != nil {
			return nil, err
		}
		plan, err := stackflow.IACPlan(stackCfg, in.Env)
		if err != nil {
			return nil, err
		}
		planJSON, _ = json.Marshal(plan)
	}

	return codex.BuildManifest(cfg, codex.TaskRequest{
		Task:       in.Task,
		ConfigPath: in.ConfigPath,
		ConfigYAML: in.ConfigYAML,
		Env:        in.Env,
		IACPlan:    planJSON,
	}), nil
}

func (s *Server) callOpenClawPatch(args json.RawMessage) (any, error) {
	var in struct {
		AgentID        string `json:"agent_id"`
		Workspace      string `json:"workspace"`
		EnvFile        string `json:"env_file"`
		MCPURL         string `json:"mcp_url"`
		IncludeSecrets bool   `json:"include_secrets"`
	}
	if err := json.Unmarshal(args, &in); err != nil && len(args) > 0 {
		return nil, fmt.Errorf("invalid args: %w", err)
	}

	workspace := in.Workspace
	if workspace == "" {
		workspace = s.workspaceDir
	}
	workspace = defaults.WorkspaceDir(workspace)

	envFile := in.EnvFile
	if envFile == "" {
		envFile = s.envFile
	}
	if envFile == "" {
		envFile = filepath.Join(workspace, ".env")
	}
	if _, err := os.Stat(envFile); err != nil {
		return nil, err
	}

	env, err := openclaw.LoadGatewayEnv(envFile)
	if err != nil {
		return nil, err
	}
	mcpURL := in.MCPURL
	if mcpURL == "" {
		mcpURL = s.codex.MCPURL
	}
	return openclaw.BuildRegistration(env, codex.DefaultBridgeConfig(workspace, mcpURL), openclaw.RegistrationOptions{
		AgentID:        in.AgentID,
		Workspace:      workspace,
		MCPURL:         mcpURL,
		IncludeSecrets: in.IncludeSecrets,
	}), nil
}
