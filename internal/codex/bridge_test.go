package codex

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildManifest(t *testing.T) {
	workspace := filepath.Join(string(filepath.Separator), "tmp", "workspace")
	cfg := DefaultBridgeConfig(workspace, "http://127.0.0.1:8808/mcp")
	plan := json.RawMessage(`{"summary":{"targets":2}}`)

	manifest := BuildManifest(cfg, TaskRequest{
		Task:       "plan prod rollout",
		ConfigPath: "stackflow.yaml",
		Env:        "prod",
		IACPlan:    plan,
	})

	if manifest.Kind != "xcloudflow.codex.manifest/v1" {
		t.Fatalf("unexpected kind: %s", manifest.Kind)
	}
	if manifest.Workspace != workspace {
		t.Fatalf("unexpected workspace: %s", manifest.Workspace)
	}
	if manifest.Request == nil || manifest.Request.Task != "plan prod rollout" {
		t.Fatalf("request payload missing task: %+v", manifest.Request)
	}
	if manifest.MCPURL != "http://127.0.0.1:8808/mcp" {
		t.Fatalf("unexpected mcp url: %s", manifest.MCPURL)
	}
	if manifest.HomeDir == "" {
		t.Fatal("expected codex home to be populated")
	}
	if manifest.TerraformRepo == "" || manifest.PlaybooksRepo == "" {
		t.Fatalf("expected automation repos in manifest: %+v", manifest)
	}
	if len(manifest.SystemPrompt) == 0 {
		t.Fatal("expected system prompt to be populated")
	}
}

func TestRenderHomeFiles(t *testing.T) {
	cfg := DefaultBridgeConfig(t.TempDir(), "http://127.0.0.1:8808/mcp")
	cfg.SSHMCPURL = "https://edge-ssh.example/mcp"

	files := RenderHomeFiles(cfg)
	if files.ConfigTOML == "" || files.MCPServersTOML == "" {
		t.Fatal("expected rendered codex home files")
	}
	if want := `bearer_token_env_var = "XCF_SSH_MCP_BEARER_TOKEN"`; !contains(files.ConfigTOML, want) {
		t.Fatalf("expected ssh bearer token env var in config: %s", files.ConfigTOML)
	}
}

func contains(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}
