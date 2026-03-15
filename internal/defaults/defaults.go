package defaults

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	DefaultMCPPort           = "8808"
	DefaultTerraformRepo     = "/Users/shenlan/workspaces/cloud-neutral-toolkit/iac_modules"
	DefaultPlaybooksRepo     = "/Users/shenlan/workspaces/cloud-neutral-toolkit/playbooks"
	DefaultTenantID          = "default"
	DefaultOpenClawAgentID   = "xcloudflow-iac"
	DefaultSSHMCPServerName  = "edge_ssh"
	DefaultSSHMCPTool        = "ssh_execute"
	DefaultCodexHomeRelative = ".xcloudflow/codex-home/default"
	DefaultCodexCommand      = "codex"
	DefaultCodexHarnessID    = "codex"
	DefaultCodexBackend      = "acpx"
	DefaultCodexMode         = "persistent"
)

func WorkspaceDir(workspace string) string {
	workspace = strings.TrimSpace(workspace)
	if workspace == "" {
		workspace, _ = os.Getwd()
	}
	if workspace == "" {
		return "."
	}
	if abs, err := filepath.Abs(workspace); err == nil {
		return abs
	}
	return workspace
}

func MCPPort() string {
	if port := strings.TrimSpace(os.Getenv("XCF_MCP_PORT")); port != "" {
		return port
	}
	return DefaultMCPPort
}

func MCPURL() string {
	if url := strings.TrimSpace(os.Getenv("XCF_MCP_URL")); url != "" {
		return url
	}
	return fmt.Sprintf("http://127.0.0.1:%s/mcp", MCPPort())
}

func TerraformRepo() string {
	if repo := strings.TrimSpace(os.Getenv("XCF_TERRAFORM_REPO")); repo != "" {
		return repo
	}
	return DefaultTerraformRepo
}

func PlaybooksRepo() string {
	if repo := strings.TrimSpace(os.Getenv("XCF_PLAYBOOKS_REPO")); repo != "" {
		return repo
	}
	return DefaultPlaybooksRepo
}

func TenantID() string {
	if tenantID := strings.TrimSpace(os.Getenv("XCF_TENANT_ID")); tenantID != "" {
		return tenantID
	}
	return DefaultTenantID
}

func SSHMCPURL() string {
	return strings.TrimSpace(os.Getenv("XCF_SSH_MCP_URL"))
}

func SSHMCPToolName() string {
	if name := strings.TrimSpace(os.Getenv("XCF_SSH_MCP_TOOL")); name != "" {
		return name
	}
	return DefaultSSHMCPTool
}

func SSHMCPBearerToken() string {
	return strings.TrimSpace(os.Getenv("XCF_SSH_MCP_BEARER_TOKEN"))
}

func OpenClawAgentID() string {
	if agentID := strings.TrimSpace(os.Getenv("XCF_OPENCLAW_AGENT_ID")); agentID != "" {
		return agentID
	}
	return DefaultOpenClawAgentID
}

func CodexRepo(workspace string) string {
	if repo := strings.TrimSpace(os.Getenv("XCF_CODEX_REPO")); repo != "" {
		return repo
	}
	return filepath.Join(WorkspaceDir(workspace), "third_party", "codex")
}

func CodexHome(workspace string) string {
	if home := strings.TrimSpace(os.Getenv("XCF_CODEX_HOME")); home != "" {
		if filepath.IsAbs(home) {
			return home
		}
		return filepath.Join(WorkspaceDir(workspace), home)
	}
	return filepath.Join(WorkspaceDir(workspace), DefaultCodexHomeRelative)
}
