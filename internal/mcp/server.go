package mcp

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"xcloudflow/internal/codex"
	"xcloudflow/internal/store"
)

// Minimal JSON-RPC 2.0 handler that supports:
// - initialize
// - tools/list
// - tools/call
//
// This is intentionally small: enough to act as an MCP-like server on Cloud Run.

type ServerOptions struct {
	Store        *store.Store
	WorkspaceDir string
	EnvFile      string
	MCPURL       string
}

type Server struct {
	store        *store.Store
	tools        []Tool
	workspaceDir string
	envFile      string
	codex        codex.BridgeConfig
}

type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

func NewServer(opts ServerOptions) *Server {
	workspace := opts.WorkspaceDir
	if workspace == "" {
		if wd, err := os.Getwd(); err == nil {
			workspace = wd
		}
	}
	envFile := opts.EnvFile
	if envFile == "" {
		envFile = filepath.Join(workspace, ".env")
	}
	codexCfg := codex.DefaultBridgeConfig(workspace, opts.MCPURL)

	tools := []Tool{
		{
			Name:        "stackflow.validate",
			Description: "Validate StackFlow config (schema + constraints).",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"config_yaml":{"type":"string"},"env":{"type":"string"}},"required":["config_yaml"]}`),
		},
		{
			Name:        "stackflow.plan.dns",
			Description: "Generate DNS plan from StackFlow config.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"config_yaml":{"type":"string"},"env":{"type":"string"}},"required":["config_yaml"]}`),
		},
		{
			Name:        "stackflow.plan.iac",
			Description: "Generate a high-level IaC plan from StackFlow config.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"config_yaml":{"type":"string"},"env":{"type":"string"}},"required":["config_yaml"]}`),
		},
		{
			Name:        "stackflow.codex.manifest",
			Description: "Build an embedded Codex manifest for an IaC automation task.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"task":{"type":"string"},"config_yaml":{"type":"string"},"config_path":{"type":"string"},"env":{"type":"string"},"mcp_url":{"type":"string"},"workspace":{"type":"string"}}}`),
		},
		{
			Name:        "stackflow.openclaw.registration",
			Description: "Generate an OpenClaw agent registration patch for the embedded Codex IaC agent.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"agent_id":{"type":"string"},"workspace":{"type":"string"},"env_file":{"type":"string"},"mcp_url":{"type":"string"},"include_secrets":{"type":"boolean"}}}`),
		},
		{
			Name:        "agent.codex.manifest",
			Description: "Build the stable Codex ACP manifest used by XCloudFlow.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"task":{"type":"string"},"config_yaml":{"type":"string"},"config_path":{"type":"string"},"env":{"type":"string"},"mcp_url":{"type":"string"},"workspace":{"type":"string"}}}`),
		},
		{
			Name:        "agent.openclaw.patch",
			Description: "Generate the OpenClaw ACP agent patch for the Codex-backed XCloudFlow agent.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"agent_id":{"type":"string"},"workspace":{"type":"string"},"env_file":{"type":"string"},"mcp_url":{"type":"string"},"include_secrets":{"type":"boolean"}}}`),
		},
		{
			Name:        "iac.terraform.plan",
			Description: "Stage a Terraform module from the shared IaC repository, run init, then generate a plan.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"module":{"type":"string"},"working_dir":{"type":"string"},"vars_file":{"type":"string"},"vars":{"type":"object"},"workspace":{"type":"string"}}}`),
		},
		{
			Name:        "iac.terraform.apply",
			Description: "Stage a Terraform module from the shared IaC repository, run init, then apply it behind an explicit approval gate.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"module":{"type":"string"},"working_dir":{"type":"string"},"vars_file":{"type":"string"},"vars":{"type":"object"},"workspace":{"type":"string"},"confirm":{"type":"string"},"change_ref":{"type":"string"},"change_set_id":{"type":"string"},"tenant_id":{"type":"string"}}}`),
		},
		{
			Name:        "config.ansible.check",
			Description: "Run ansible-playbook syntax-check and --check against the shared playbooks repository.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"playbook":{"type":"string"},"inventory":{"type":"string"},"limit":{"type":"string"},"extra_vars":{"type":"object"}},"required":["playbook"]}`),
		},
		{
			Name:        "config.ansible.apply",
			Description: "Run ansible-playbook from the shared playbooks repository behind an explicit approval gate.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"playbook":{"type":"string"},"inventory":{"type":"string"},"limit":{"type":"string"},"extra_vars":{"type":"object"},"confirm":{"type":"string"},"change_ref":{"type":"string"},"change_set_id":{"type":"string"},"tenant_id":{"type":"string"}},"required":["playbook"]}`),
		},
		{
			Name:        "edge.ssh.exec",
			Description: "Execute a read-only or explicitly approved command through the external edge_ssh MCP server.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"target":{"type":"string"},"command":{"type":"string"},"cwd":{"type":"string"},"env":{"type":"object"},"timeout_sec":{"type":"integer"},"confirm":{"type":"string"},"change_ref":{"type":"string"},"change_set_id":{"type":"string"},"tenant_id":{"type":"string"}},"required":["target","command"]}`),
		},
		{
			Name:        "terraform.init",
			Description: "Run terraform init in a provided working directory.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"working_dir":{"type":"string"},"workspace":{"type":"string"}},"required":["working_dir"]}`),
		},
		{
			Name:        "terraform.plan",
			Description: "Run terraform plan in a provided working directory.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"working_dir":{"type":"string"},"vars_file":{"type":"string"},"vars":{"type":"object"},"workspace":{"type":"string"}},"required":["working_dir"]}`),
		},
		{
			Name:        "terraform.apply",
			Description: "Run terraform apply in a provided working directory behind an explicit approval gate.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"working_dir":{"type":"string"},"vars_file":{"type":"string"},"vars":{"type":"object"},"workspace":{"type":"string"},"confirm":{"type":"string"},"change_ref":{"type":"string"},"change_set_id":{"type":"string"},"tenant_id":{"type":"string"}},"required":["working_dir"]}`),
		},
		{
			Name:        "terraform.destroy",
			Description: "Run terraform destroy in a provided working directory behind an explicit approval gate.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"working_dir":{"type":"string"},"vars_file":{"type":"string"},"vars":{"type":"object"},"workspace":{"type":"string"},"confirm":{"type":"string"},"change_ref":{"type":"string"},"change_set_id":{"type":"string"},"tenant_id":{"type":"string"}},"required":["working_dir"]}`),
		},
		{
			Name:        "ansible.playbook",
			Description: "Run ansible-playbook in check mode or apply mode using the shared playbooks repository.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"playbook":{"type":"string"},"inventory":{"type":"string"},"limit":{"type":"string"},"extra_vars":{"type":"object"},"check":{"type":"boolean"},"confirm":{"type":"string"},"change_ref":{"type":"string"},"change_set_id":{"type":"string"},"tenant_id":{"type":"string"}},"required":["playbook"]}`),
		},
		{
			Name:        "ansible.adhoc",
			Description: "Run an ansible ad-hoc command in check mode or behind an explicit approval gate.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"inventory":{"type":"string"},"target":{"type":"string"},"module":{"type":"string"},"args":{"type":"string"},"check":{"type":"boolean"},"confirm":{"type":"string"},"change_ref":{"type":"string"},"change_set_id":{"type":"string"},"tenant_id":{"type":"string"}},"required":["target","module"]}`),
		},
	}
	return &Server{
		store:        opts.Store,
		tools:        tools,
		workspaceDir: workspace,
		envFile:      envFile,
		codex:        codexCfg,
	}
}

type rpcReq struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResp struct {
	JSONRPC string  `json:"jsonrpc"`
	ID      any     `json:"id"`
	Result  any     `json:"result,omitempty"`
	Error   *rpcErr `json:"error,omitempty"`
}

type rpcErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req rpcReq
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeJSON(w, rpcResp{JSONRPC: "2.0", ID: nil, Error: &rpcErr{Code: -32700, Message: "invalid JSON"}})
		return
	}
	if req.JSONRPC == "" {
		req.JSONRPC = "2.0"
	}

	switch req.Method {
	case "initialize":
		writeJSON(w, rpcResp{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{
			"server": map[string]any{
				"name":    "xcloudflow",
				"version": "0.1",
			},
			"capabilities": map[string]any{
				"tools": true,
			},
			"time": time.Now().UTC().Format(time.RFC3339),
		}})
		return

	case "tools/list":
		writeJSON(w, rpcResp{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"tools": s.tools}})
		return

	case "tools/call":
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil || p.Name == "" {
			writeJSON(w, rpcResp{JSONRPC: "2.0", ID: req.ID, Error: &rpcErr{Code: -32602, Message: "invalid params"}})
			return
		}

		res, err := s.callTool(r.Context(), p.Name, p.Arguments)
		if err != nil {
			writeJSON(w, rpcResp{JSONRPC: "2.0", ID: req.ID, Error: &rpcErr{Code: -32000, Message: err.Error()}})
			return
		}
		writeJSON(w, rpcResp{JSONRPC: "2.0", ID: req.ID, Result: res})
		return
	default:
		writeJSON(w, rpcResp{JSONRPC: "2.0", ID: req.ID, Error: &rpcErr{Code: -32601, Message: "method not found"}})
		return
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
