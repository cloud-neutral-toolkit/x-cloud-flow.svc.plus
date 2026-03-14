#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

WORKSPACE="${XCF_WORKSPACE:-${REPO_ROOT}}"
CODEX_HOME="${XCF_CODEX_HOME:-${REPO_ROOT}/.xcloudflow/codex-home/default}"
ENV_FILE="${XCF_ENV_FILE:-${REPO_ROOT}/.env}"
MCP_PORT="${XCF_MCP_PORT:-8808}"
ADDR=":${MCP_PORT}"
HEALTH_URL="${XCF_MCP_HEALTH_URL:-http://127.0.0.1:${MCP_PORT}/healthz}"
PID_FILE="${CODEX_HOME}/state/xcloudflow-mcp.pid"
LOG_FILE="${CODEX_HOME}/log/xcloudflow-mcp.log"

mkdir -p "$(dirname "${PID_FILE}")" "$(dirname "${LOG_FILE}")"

if curl -fsS "${HEALTH_URL}" >/dev/null 2>&1; then
  printf '%s\n' "${HEALTH_URL}"
  exit 0
fi

if [[ -f "${PID_FILE}" ]]; then
  if kill -0 "$(cat "${PID_FILE}")" >/dev/null 2>&1; then
    :
  else
    rm -f "${PID_FILE}"
  fi
fi

if [[ ! -f "${PID_FILE}" ]]; then
  if command -v xcloudflow >/dev/null 2>&1; then
    XCFLOW_CMD=(xcloudflow)
  else
    mkdir -p "${REPO_ROOT}/.xcloudflow/bin"
    go build -o "${REPO_ROOT}/.xcloudflow/bin/xcloudflow" ./cmd/xcloudflow
    XCFLOW_CMD=("${REPO_ROOT}/.xcloudflow/bin/xcloudflow")
  fi
  (
    cd "${REPO_ROOT}"
    nohup "${XCFLOW_CMD[@]}" mcp serve --addr "${ADDR}" --workspace "${WORKSPACE}" --env-file "${ENV_FILE}" >>"${LOG_FILE}" 2>&1 &
    printf '%s\n' "$!" > "${PID_FILE}"
  )
fi

for _ in $(seq 1 20); do
  if curl -fsS "${HEALTH_URL}" >/dev/null 2>&1; then
    printf '%s\n' "${HEALTH_URL}"
    exit 0
  fi
  sleep 1
done

echo "failed to start xcloudflow mcp server; see ${LOG_FILE}" >&2
exit 1
