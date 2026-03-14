package openclaw

import (
	"os"
	"path/filepath"
	"testing"

	"xcloudflow/internal/codex"
)

func TestLoadGatewayEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := "remote: wss://openclaw.example/ws\nremote-token: secret-token\n\"AI-Gateway-Url\": \"https://api.example/v1\",\n\"AI-Gateway-apiKey\": \"secret-key\",\nXCF_SSH_MCP_URL=https://ssh.example/mcp\nXCF_SSH_MCP_BEARER_TOKEN=ssh-secret\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write env: %v", err)
	}

	env, err := LoadGatewayEnv(path)
	if err != nil {
		t.Fatalf("load env: %v", err)
	}
	if env.RemoteURL != "wss://openclaw.example/ws" {
		t.Fatalf("unexpected remote url: %s", env.RemoteURL)
	}
	if env.RemoteToken != "secret-token" {
		t.Fatalf("unexpected remote token: %s", env.RemoteToken)
	}
	if env.AIGatewayURL != "https://api.example/v1" {
		t.Fatalf("unexpected ai gateway url: %s", env.AIGatewayURL)
	}
	if env.AIGatewayAPIKey != "secret-key" {
		t.Fatalf("unexpected ai gateway key: %s", env.AIGatewayAPIKey)
	}
	if env.SSHMCPURL != "https://ssh.example/mcp" {
		t.Fatalf("unexpected ssh mcp url: %s", env.SSHMCPURL)
	}
	if env.SSHMCPToken != "ssh-secret" {
		t.Fatalf("unexpected ssh mcp token: %s", env.SSHMCPToken)
	}
}

func TestBuildRegistrationMasksSecrets(t *testing.T) {
	env := GatewayEnv{
		RemoteURL:       "wss://openclaw.example/ws",
		RemoteToken:     "secret-token",
		AIGatewayURL:    "https://api.example/v1",
		AIGatewayAPIKey: "secret-key",
	}
	cfg := codex.DefaultBridgeConfig(t.TempDir(), "http://127.0.0.1:8808/mcp")
	spec := BuildRegistration(env, cfg, RegistrationOptions{AgentID: "iac"})

	token, ok := spec.GatewayRemote["token"].(map[string]any)
	if !ok {
		t.Fatalf("expected masked token map, got %#v", spec.GatewayRemote["token"])
	}
	if _, exists := token["value"]; exists {
		t.Fatalf("expected token value to be omitted when IncludeSecrets=false")
	}
	if spec.ShellEnv["CODEX_HOME"] == nil {
		t.Fatalf("expected CODEX_HOME in shell env: %#v", spec.ShellEnv)
	}
}
