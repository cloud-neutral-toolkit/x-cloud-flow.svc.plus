package defaults

import (
	"path/filepath"
	"testing"
)

func TestMCPURLUsesEnvOverride(t *testing.T) {
	t.Setenv("XCF_MCP_URL", "http://127.0.0.1:9911/mcp")
	if got := MCPURL(); got != "http://127.0.0.1:9911/mcp" {
		t.Fatalf("unexpected mcp url: %s", got)
	}
}

func TestMCPURLUsesDefaultPort(t *testing.T) {
	t.Setenv("XCF_MCP_URL", "")
	t.Setenv("XCF_MCP_PORT", "")
	if got := MCPURL(); got != "http://127.0.0.1:8808/mcp" {
		t.Fatalf("unexpected default mcp url: %s", got)
	}
}

func TestRepoDefaultsSupportEnvOverride(t *testing.T) {
	t.Setenv("XCF_TERRAFORM_REPO", "/tmp/tf")
	t.Setenv("XCF_PLAYBOOKS_REPO", "/tmp/pb")
	if got := TerraformRepo(); got != "/tmp/tf" {
		t.Fatalf("unexpected terraform repo: %s", got)
	}
	if got := PlaybooksRepo(); got != "/tmp/pb" {
		t.Fatalf("unexpected playbooks repo: %s", got)
	}
}

func TestCodexHomeResolvesRelativeOverride(t *testing.T) {
	t.Setenv("XCF_CODEX_HOME", ".custom/codex")
	workspace := filepath.Join(string(filepath.Separator), "tmp", "workspace")
	got := CodexHome(workspace)
	want := filepath.Join(workspace, ".custom", "codex")
	if got != want {
		t.Fatalf("unexpected codex home: got %s want %s", got, want)
	}
}
