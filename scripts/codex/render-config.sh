#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
TEMPLATE_DIR="${REPO_ROOT}/configs/codex"

WORKSPACE="${XCF_WORKSPACE:-${REPO_ROOT}}"
CODEX_HOME="${XCF_CODEX_HOME:-${REPO_ROOT}/.xcloudflow/codex-home/default}"
ENV_FILE="${XCF_ENV_FILE:-${REPO_ROOT}/.env}"
SANDBOX_MODE="${XCF_CODEX_SANDBOX_MODE:-workspace-write}"
MCP_PORT="${XCF_MCP_PORT:-8808}"
MCP_URL="${XCF_MCP_URL:-http://127.0.0.1:${MCP_PORT}/mcp}"

if command -v xcloudflow >/dev/null 2>&1; then
  XCFLOW_CMD=(xcloudflow)
else
  XCFLOW_CMD=(go run ./cmd/xcloudflow)
fi

cd "${REPO_ROOT}"
if [[ -f "${ENV_FILE}" ]]; then
  eval "$("${XCFLOW_CMD[@]}" agent shell-env --env-file "${ENV_FILE}" --workspace "${WORKSPACE}" --mcp-url "${MCP_URL}")"
fi

mkdir -p "${CODEX_HOME}"
MCP_SERVERS_FILE="${CODEX_HOME}/mcp-servers.toml"

awk \
  -v xcf_mcp_url="${XCF_MCP_URL:-${MCP_URL}}" \
  -v edge_ssh_url="${XCF_SSH_MCP_URL:-}" \
  -v edge_ssh_token_env="${XCF_SSH_MCP_BEARER_TOKEN:+XCF_SSH_MCP_BEARER_TOKEN}" '
  {
    gsub(/__XCF_MCP_URL__/, xcf_mcp_url)
    if ($0 == "__EDGE_SSH_BLOCK__") {
      if (edge_ssh_url != "") {
        print "[mcp_servers.edge_ssh]"
        print "url = \"" edge_ssh_url "\""
        if (edge_ssh_token_env != "") {
          print "bearer_token_env_var = \"" edge_ssh_token_env "\""
        }
        print ""
      }
      next
    }
    print
  }
' "${TEMPLATE_DIR}/mcp-servers.toml.tpl" > "${MCP_SERVERS_FILE}"

awk \
  -v sandbox_mode="${SANDBOX_MODE}" \
  -v workspace="${WORKSPACE}" \
  -v codex_home="${CODEX_HOME}" \
  -v mcp_servers_file="${MCP_SERVERS_FILE}" '
  {
    gsub(/__SANDBOX_MODE__/, sandbox_mode)
    gsub(/__WORKSPACE__/, workspace)
    gsub(/__CODEX_HOME__/, codex_home)
    if ($0 == "__MCP_SERVERS__") {
      while ((getline line < mcp_servers_file) > 0) {
        print line
      }
      close(mcp_servers_file)
      next
    }
    print
  }
' "${TEMPLATE_DIR}/config.toml.tpl" > "${CODEX_HOME}/config.toml"

printf '%s\n' "${CODEX_HOME}/config.toml"
