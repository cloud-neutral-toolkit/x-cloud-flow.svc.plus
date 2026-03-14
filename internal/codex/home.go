package codex

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type HomeFiles struct {
	ConfigTOML     string `json:"configToml"`
	MCPServersTOML string `json:"mcpServersToml"`
}

func RenderHomeFiles(cfg BridgeConfig) HomeFiles {
	edgeServer := ""
	if strings.TrimSpace(cfg.SSHMCPURL) != "" {
		edgeServer = fmt.Sprintf("\n[mcp_servers.edge_ssh]\nurl = %q\nbearer_token_env_var = %q\n", cfg.SSHMCPURL, "XCF_SSH_MCP_BEARER_TOKEN")
	}

	mcpServers := strings.TrimSpace(fmt.Sprintf(`[mcp_servers.xcloudflow]
url = %q%s
`, cfg.MCPURL, edgeServer))
	config := strings.TrimSpace(fmt.Sprintf(`sandbox_mode = "workspace-write"

[sandbox_workspace_write]
writable_roots = [%q, %q]
network_access = true

%s
`, cfg.Workspace, cfg.HomeDir, mcpServers))

	return HomeFiles{
		ConfigTOML:     config + "\n",
		MCPServersTOML: mcpServers + "\n",
	}
}

func InitHome(cfg BridgeConfig) error {
	if cfg.HomeDir == "" {
		return fmt.Errorf("missing codex home dir")
	}
	for _, dir := range []string{
		cfg.HomeDir,
		filepath.Join(cfg.HomeDir, "log"),
		filepath.Join(cfg.HomeDir, "prompts"),
		filepath.Join(cfg.HomeDir, "state"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	files := RenderHomeFiles(cfg)
	if err := os.WriteFile(filepath.Join(cfg.HomeDir, "config.toml"), []byte(files.ConfigTOML), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(cfg.HomeDir, "mcp-servers.toml"), []byte(files.MCPServersTOML), 0o644); err != nil {
		return err
	}
	return nil
}
