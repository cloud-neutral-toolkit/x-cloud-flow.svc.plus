package codex

import (
	"encoding/json"
	"strings"

	"xcloudflow/internal/defaults"
)

type BridgeConfig struct {
	HarnessID     string `json:"harnessId"`
	Backend       string `json:"backend"`
	Mode          string `json:"mode"`
	Workspace     string `json:"workspace"`
	RepoDir       string `json:"repoDir"`
	HomeDir       string `json:"homeDir"`
	Command       string `json:"command"`
	MCPURL        string `json:"mcpUrl,omitempty"`
	MCPPort       string `json:"mcpPort,omitempty"`
	TerraformRepo string `json:"terraformRepo,omitempty"`
	PlaybooksRepo string `json:"playbooksRepo,omitempty"`
	SSHMCPURL     string `json:"sshMcpUrl,omitempty"`
}

type TaskRequest struct {
	Task       string          `json:"task"`
	ConfigPath string          `json:"configPath,omitempty"`
	ConfigYAML string          `json:"configYaml,omitempty"`
	Env        string          `json:"env,omitempty"`
	IACPlan    json.RawMessage `json:"iacPlan,omitempty"`
}

type Manifest struct {
	Kind          string       `json:"kind"`
	HarnessID     string       `json:"harnessId"`
	Backend       string       `json:"backend"`
	Mode          string       `json:"mode"`
	Workspace     string       `json:"workspace"`
	RepoDir       string       `json:"repoDir"`
	HomeDir       string       `json:"homeDir"`
	Command       string       `json:"command"`
	MCPURL        string       `json:"mcpUrl,omitempty"`
	MCPPort       string       `json:"mcpPort,omitempty"`
	TerraformRepo string       `json:"terraformRepo,omitempty"`
	PlaybooksRepo string       `json:"playbooksRepo,omitempty"`
	SSHMCPURL     string       `json:"sshMcpUrl,omitempty"`
	SystemPrompt  string       `json:"systemPrompt"`
	Request       *TaskRequest `json:"request"`
}

func DefaultBridgeConfig(workspace, mcpURL string) BridgeConfig {
	workspace = defaults.WorkspaceDir(workspace)
	if strings.TrimSpace(mcpURL) == "" {
		mcpURL = defaults.MCPURL()
	}

	return BridgeConfig{
		HarnessID:     defaults.DefaultCodexHarnessID,
		Backend:       defaults.DefaultCodexBackend,
		Mode:          defaults.DefaultCodexMode,
		Workspace:     workspace,
		RepoDir:       defaults.CodexRepo(workspace),
		HomeDir:       defaults.CodexHome(workspace),
		Command:       defaults.DefaultCodexCommand,
		MCPURL:        mcpURL,
		MCPPort:       defaults.MCPPort(),
		TerraformRepo: defaults.TerraformRepo(),
		PlaybooksRepo: defaults.PlaybooksRepo(),
		SSHMCPURL:     defaults.SSHMCPURL(),
	}
}

func BuildManifest(cfg BridgeConfig, req TaskRequest) Manifest {
	task := strings.TrimSpace(req.Task)
	if task == "" {
		task = "Review the StackFlow configuration, generate an IaC plan, and explain the next safe execution step."
	}
	req.Task = task

	return Manifest{
		Kind:          "xcloudflow.codex.manifest/v1",
		HarnessID:     cfg.HarnessID,
		Backend:       cfg.Backend,
		Mode:          cfg.Mode,
		Workspace:     cfg.Workspace,
		RepoDir:       cfg.RepoDir,
		HomeDir:       cfg.HomeDir,
		Command:       cfg.Command,
		MCPURL:        cfg.MCPURL,
		MCPPort:       cfg.MCPPort,
		TerraformRepo: cfg.TerraformRepo,
		PlaybooksRepo: cfg.PlaybooksRepo,
		SSHMCPURL:     cfg.SSHMCPURL,
		SystemPrompt:  buildSystemPrompt(cfg, req),
		Request:       &req,
	}
}

func buildSystemPrompt(cfg BridgeConfig, req TaskRequest) string {
	var lines []string
	lines = append(lines, "You are the embedded Codex runtime for XCloudFlow.")
	lines = append(lines, "Goal: act as an IaC automation agent without replacing existing XCloudFlow validation and planning behavior.")
	lines = append(lines, "Always inspect the current repository state before proposing changes.")
	lines = append(lines, "Prefer calling the local XCloudFlow MCP server for StackFlow validation and planning before making infrastructure edits.")
	lines = append(lines, "Use Terraform as the default IaC engine and the shared playbooks repository as the default configuration automation source.")
	lines = append(lines, "Keep mutating terraform/ansible/ssh actions behind an explicit confirm=APPLY + change_ref gate.")
	lines = append(lines, "Do not convert xconfig-agent into a Codex/OpenClaw edge runtime; it remains a lightweight node-side executor.")
	if cfg.MCPURL != "" {
		lines = append(lines, "MCP endpoint: "+cfg.MCPURL)
	}
	lines = append(lines, "Workspace: "+cfg.Workspace)
	if cfg.HomeDir != "" {
		lines = append(lines, "CODEX_HOME: "+cfg.HomeDir)
	}
	lines = append(lines, "Codex source repo: "+cfg.RepoDir)
	if cfg.TerraformRepo != "" {
		lines = append(lines, "Terraform repo: "+cfg.TerraformRepo)
	}
	if cfg.PlaybooksRepo != "" {
		lines = append(lines, "Playbooks repo: "+cfg.PlaybooksRepo)
	}
	if cfg.SSHMCPURL != "" {
		lines = append(lines, "Edge SSH MCP server: "+cfg.SSHMCPURL)
	}
	if req.ConfigPath != "" {
		lines = append(lines, "Primary StackFlow config path: "+req.ConfigPath)
	}
	if req.Env != "" {
		lines = append(lines, "Target environment: "+req.Env)
	}
	lines = append(lines, "Task: "+req.Task)
	if len(req.IACPlan) > 0 {
		lines = append(lines, "An XCloudFlow-generated IaC plan is attached in the request payload; treat it as authoritative context.")
	}
	if req.ConfigYAML != "" {
		lines = append(lines, "A raw StackFlow YAML payload is attached in the request payload when file access is unavailable.")
	}
	return strings.Join(lines, "\n")
}
