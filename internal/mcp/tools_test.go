package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIACTerraformPlanStagesExternalRepo(t *testing.T) {
	workspace := t.TempDir()
	tfRepo := t.TempDir()
	moduleDir := filepath.Join(tfRepo, "example", "terraform", "demo")
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		t.Fatalf("mkdir module: %v", err)
	}
	if err := os.WriteFile(filepath.Join(moduleDir, "main.tf"), []byte("terraform {}"), 0o644); err != nil {
		t.Fatalf("write main.tf: %v", err)
	}

	binDir := t.TempDir()
	writeStubCommand(t, binDir, "terraform", "#!/usr/bin/env bash\necho \"$0 $@\"\n")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("XCF_TERRAFORM_REPO", tfRepo)

	srv := NewServer(ServerOptions{WorkspaceDir: workspace, MCPURL: "http://127.0.0.1:8808/mcp"})
	out, err := srv.callTool(context.Background(), "iac.terraform.plan", json.RawMessage(`{"module":"demo"}`))
	if err != nil {
		t.Fatalf("callTool: %v", err)
	}

	result := out.(map[string]any)
	if ok := result["ok"]; ok != true {
		t.Fatalf("expected ok=true, got %#v", result)
	}
	workingDir := result["working_dir"].(string)
	if !strings.HasPrefix(workingDir, filepath.Join(workspace, ".xcloudflow", "terraform")) {
		t.Fatalf("expected staged working dir, got %s", workingDir)
	}
	if _, err := os.Stat(filepath.Join(moduleDir, ".terraform")); !os.IsNotExist(err) {
		t.Fatalf("expected source repo to remain clean, got err=%v", err)
	}
}

func TestConfigAnsibleCheckUsesSharedRepo(t *testing.T) {
	workspace := t.TempDir()
	playbooksRepo := t.TempDir()
	if err := os.WriteFile(filepath.Join(playbooksRepo, "ansible.cfg"), []byte("[defaults]\n"), 0o644); err != nil {
		t.Fatalf("write ansible.cfg: %v", err)
	}
	if err := os.WriteFile(filepath.Join(playbooksRepo, "inventory.ini"), []byte("[all]\nlocalhost ansible_connection=local\n"), 0o644); err != nil {
		t.Fatalf("write inventory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(playbooksRepo, "site.yml"), []byte("- hosts: all\n  gather_facts: false\n  tasks: []\n"), 0o644); err != nil {
		t.Fatalf("write playbook: %v", err)
	}

	binDir := t.TempDir()
	writeStubCommand(t, binDir, "ansible-playbook", "#!/usr/bin/env bash\necho \"$0 $@\"\n")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("XCF_PLAYBOOKS_REPO", playbooksRepo)

	srv := NewServer(ServerOptions{WorkspaceDir: workspace, MCPURL: "http://127.0.0.1:8808/mcp"})
	out, err := srv.callTool(context.Background(), "config.ansible.check", json.RawMessage(`{"playbook":"site.yml"}`))
	if err != nil {
		t.Fatalf("callTool: %v", err)
	}

	result := out.(map[string]any)
	if ok := result["ok"]; ok != true {
		t.Fatalf("expected ok=true, got %#v", result)
	}
	if mode := result["mode"]; mode != "check" {
		t.Fatalf("expected check mode, got %#v", mode)
	}
}

func TestEdgeSSHExecCallsExternalMCPServer(t *testing.T) {
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
			ID     any             `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		switch req.Method {
		case "tools/list":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result": map[string]any{
					"tools": []map[string]any{
						{
							"name":        "ssh_execute",
							"description": "execute ssh command",
							"inputSchema": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"server":  map[string]any{"type": "string"},
									"command": map[string]any{"type": "string"},
									"cwd":     map[string]any{"type": "string"},
									"timeout": map[string]any{"type": "integer"},
								},
							},
						},
					},
				},
			})
		case "tools/call":
			var params struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			}
			if err := json.Unmarshal(req.Params, &params); err != nil {
				t.Fatalf("decode params: %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result": map[string]any{
					"name":      params.Name,
					"arguments": params.Arguments,
				},
			})
		default:
			t.Fatalf("unexpected method: %s", req.Method)
		}
	}))
	defer remote.Close()

	t.Setenv("XCF_SSH_MCP_URL", remote.URL)
	srv := NewServer(ServerOptions{WorkspaceDir: t.TempDir(), MCPURL: "http://127.0.0.1:8808/mcp"})
	out, err := srv.callTool(context.Background(), "edge.ssh.exec", json.RawMessage(`{"target":"edge-01","command":"uname -a","timeout_sec":5}`))
	if err != nil {
		t.Fatalf("callTool: %v", err)
	}

	result := out.(map[string]any)
	if ok := result["ok"]; ok != true {
		t.Fatalf("expected ok=true, got %#v", result)
	}
	if result["remote_tool"] != "ssh_execute" {
		t.Fatalf("unexpected remote tool: %#v", result["remote_tool"])
	}
}

func TestEdgeSSHExecReturnsStructuredErrorWhenRemoteUnavailable(t *testing.T) {
	t.Setenv("XCF_SSH_MCP_URL", "http://127.0.0.1:1/mcp")
	srv := NewServer(ServerOptions{WorkspaceDir: t.TempDir(), MCPURL: "http://127.0.0.1:8808/mcp"})
	out, err := srv.callTool(context.Background(), "edge.ssh.exec", json.RawMessage(`{"target":"edge-01","command":"uname -a"}`))
	if err != nil {
		t.Fatalf("callTool should return structured result, got err=%v", err)
	}

	result := out.(map[string]any)
	if ok := result["ok"]; ok != false {
		t.Fatalf("expected ok=false, got %#v", result)
	}
	if result["error"] == nil {
		t.Fatalf("expected remote execution error in result: %#v", result)
	}
}

func writeStubCommand(t *testing.T, dir string, name string, body string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write stub command: %v", err)
	}
}
