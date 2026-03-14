#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

export XCF_WORKSPACE="${XCF_WORKSPACE:-${REPO_ROOT}}"
export XCF_CODEX_HOME="${XCF_CODEX_HOME:-${REPO_ROOT}/.xcloudflow/codex-home/default}"

mkdir -p "${XCF_CODEX_HOME}/log" "${XCF_CODEX_HOME}/prompts" "${XCF_CODEX_HOME}/state"
"${SCRIPT_DIR}/render-config.sh" "$@"
printf '%s\n' "${XCF_CODEX_HOME}"
