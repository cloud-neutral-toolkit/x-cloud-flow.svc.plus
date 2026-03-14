package openclaw

import (
	"os"
	"strings"

	"xcloudflow/internal/codex"
	"xcloudflow/internal/defaults"
)

type RegistrationOptions struct {
	AgentID        string `json:"agentId"`
	Workspace      string `json:"workspace"`
	MCPURL         string `json:"mcpUrl,omitempty"`
	IncludeSecrets bool   `json:"includeSecrets"`
}

type SecretValue struct {
	Configured bool   `json:"configured"`
	EnvName    string `json:"envName"`
	Value      string `json:"value,omitempty"`
}

type RegistrationSpec struct {
	Kind                string         `json:"kind"`
	GatewayRemote       map[string]any `json:"gatewayRemote,omitempty"`
	ShellEnv            map[string]any `json:"shellEnv,omitempty"`
	OpenClawConfigPatch map[string]any `json:"openclawConfigPatch"`
	Notes               []string       `json:"notes,omitempty"`
}

func BuildRegistration(env GatewayEnv, codexCfg codex.BridgeConfig, opts RegistrationOptions) RegistrationSpec {
	agentID := strings.TrimSpace(opts.AgentID)
	if agentID == "" {
		agentID = defaults.OpenClawAgentID()
	}

	workspace := strings.TrimSpace(opts.Workspace)
	if workspace == "" {
		workspace = codexCfg.Workspace
	}
	workspace = defaults.WorkspaceDir(workspace)

	remote := map[string]any{}
	if env.RemoteURL != "" {
		remote["url"] = env.RemoteURL
	}
	if token := secretValue("OPENCLAW_GATEWAY_TOKEN", env.RemoteToken, opts.IncludeSecrets); token != nil {
		remote["token"] = token
	}

	shellEnv := map[string]any{}
	if env.AIGatewayURL != "" {
		shellEnv["OPENAI_BASE_URL"] = env.AIGatewayURL
	}
	if apiKey := secretValue("OPENAI_API_KEY", env.AIGatewayAPIKey, opts.IncludeSecrets); apiKey != nil {
		shellEnv["OPENAI_API_KEY"] = apiKey
	}
	if home := firstNonEmpty(env.CodexHome, codexCfg.HomeDir); home != "" {
		shellEnv["CODEX_HOME"] = home
	}
	if codexCfg.MCPPort != "" {
		shellEnv["XCF_MCP_PORT"] = codexCfg.MCPPort
	}
	if codexCfg.MCPURL != "" {
		shellEnv["XCF_MCP_URL"] = codexCfg.MCPURL
	}
	if repo := firstNonEmpty(env.TerraformRepo, codexCfg.TerraformRepo); repo != "" {
		shellEnv["XCF_TERRAFORM_REPO"] = repo
	}
	if repo := firstNonEmpty(env.PlaybooksRepo, codexCfg.PlaybooksRepo); repo != "" {
		shellEnv["XCF_PLAYBOOKS_REPO"] = repo
	}
	if sshURL := firstNonEmpty(env.SSHMCPURL, codexCfg.SSHMCPURL); sshURL != "" {
		shellEnv["XCF_SSH_MCP_URL"] = sshURL
	}
	if sshToken := secretValue("XCF_SSH_MCP_BEARER_TOKEN", firstNonEmpty(env.SSHMCPToken, os.Getenv("XCF_SSH_MCP_BEARER_TOKEN")), opts.IncludeSecrets); sshToken != nil {
		shellEnv["XCF_SSH_MCP_BEARER_TOKEN"] = sshToken
	}

	patch := map[string]any{
		"agents": map[string]any{
			"list": []map[string]any{
				{
					"id":        agentID,
					"workspace": workspace,
					"identity": map[string]any{
						"name":  "XCloudFlow Automation",
						"emoji": "☁️",
					},
					"runtime": map[string]any{
						"type": "acp",
						"acp": map[string]any{
							"agent":   codexCfg.HarnessID,
							"backend": codexCfg.Backend,
							"mode":    codexCfg.Mode,
							"cwd":     workspace,
						},
					},
					"tools": map[string]any{
						"profile": "coding",
					},
				},
			},
		},
	}
	if len(remote) > 0 {
		patch["gateway"] = map[string]any{
			"remote": remote,
		}
	}

	notes := []string{
		"Start OpenClaw with the codex ACP backend enabled before using this agent patch.",
		"Export OPENAI_BASE_URL and OPENAI_API_KEY into the OpenClaw/acpx runtime so Codex uses the configured AI gateway.",
		"Provision CODEX_HOME, XCF_MCP_PORT, XCF_TERRAFORM_REPO, XCF_PLAYBOOKS_REPO, and optional XCF_SSH_MCP_URL before starting the ACP runtime.",
		"Keep xconfig-agent as the lightweight node-side executor; do not run Codex/OpenClaw on edge nodes.",
	}
	if opts.MCPURL != "" {
		notes = append(notes, "Expose the XCloudFlow MCP server at "+opts.MCPURL+" and reference it from the agent workspace or runtime bootstrap.")
	}

	return RegistrationSpec{
		Kind:                "xcloudflow.openclaw.registration/v1",
		GatewayRemote:       remote,
		ShellEnv:            shellEnv,
		OpenClawConfigPatch: patch,
		Notes:               notes,
	}
}

func secretValue(envName, value string, includeSecrets bool) map[string]any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	out := map[string]any{
		"env":        envName,
		"configured": true,
	}
	if includeSecrets {
		out["value"] = value
	}
	return out
}
