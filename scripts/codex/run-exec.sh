#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

export XCF_WORKSPACE="${XCF_WORKSPACE:-${REPO_ROOT}}"
export XCF_ENV_FILE="${XCF_ENV_FILE:-${REPO_ROOT}/.env}"
export XCF_CODEX_HOME="${XCF_CODEX_HOME:-${REPO_ROOT}/.xcloudflow/codex-home/default}"
export XCF_MCP_PORT="${XCF_MCP_PORT:-8808}"
export XCF_MCP_URL="${XCF_MCP_URL:-http://127.0.0.1:${XCF_MCP_PORT}/mcp}"
export XCF_TERRAFORM_REPO="${XCF_TERRAFORM_REPO:-/Users/shenlan/workspaces/cloud-neutral-toolkit/iac_modules}"
export XCF_PLAYBOOKS_REPO="${XCF_PLAYBOOKS_REPO:-/Users/shenlan/workspaces/cloud-neutral-toolkit/playbooks}"

if command -v xcloudflow >/dev/null 2>&1; then
  XCFLOW_CMD=(xcloudflow)
else
  XCFLOW_CMD=(go run ./cmd/xcloudflow)
fi

CODEX_BIN="${XCF_CODEX_COMMAND:-codex}"
if ! command -v "${CODEX_BIN}" >/dev/null 2>&1; then
  echo "missing codex executable; install Codex CLI or set XCF_CODEX_COMMAND" >&2
  exit 1
fi

cd "${REPO_ROOT}"
if [[ -f "${XCF_ENV_FILE}" ]]; then
  eval "$("${XCFLOW_CMD[@]}" agent shell-env --env-file "${XCF_ENV_FILE}" --workspace "${XCF_WORKSPACE}" --mcp-url "${XCF_MCP_URL}")"
fi

"${SCRIPT_DIR}/init-home.sh"
"${SCRIPT_DIR}/ensure-mcp.sh"

export CODEX_HOME="${XCF_CODEX_HOME}"
cd "${XCF_WORKSPACE}"
exec "${CODEX_BIN}" exec "$@"
