#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  setup.sh <domain> [options]

Examples:
  setup.sh x-cloud-flow.svc.plus
  setup.sh x-cloud-flow.svc.plus --mode docker
  setup.sh x-cloud-flow.svc.plus --mode process --action uninstall
  setup.sh x-cloud-flow.svc.plus --mode cloud-run --project my-gcp-project --region us-central1

Options:
  --mode <process|docker|cloud-run>   Deployment mode. Default: process
  --action <install|uninstall|status> Action to run. Default: install
  --app-dir <path>                    Managed app directory. Default: /opt/x-cloud-flow.svc.plus
  --source-dir <path>                 Local source directory to sync instead of cloning
  --repo-url <url>                    Git repo URL. Default: https://github.com/cloud-neutral-toolkit/x-cloud-flow.svc.plus.git
  --branch <name>                     Git branch to clone/update. Default: main
  --port <port>                       Backend listen port. Default: 18083
  --image <ref>                       Docker image tag. Default: x-cloud-flow-svc-plus:local
  --project <id>                      GCP project for cloud-run mode
  --region <name>                     GCP region for cloud-run mode. Default: us-central1
  --service-name <name>               Service/systemd/container name. Default: x-cloud-flow-svc-plus
  --database-url <dsn>                DATABASE_URL written into env file
  --tenant-id <tenant>                XCF_TENANT_ID written into env file. Default: default
  --env-file <path>                   Managed env file path. Default: /etc/x-cloud-flow.svc.plus.env
  --caddy-dir <path>                  Caddy conf.d dir. Default: /etc/caddy/conf.d
  --skip-caddy                        Do not write/reload Caddy config
  --help                              Show this help
EOF
}

require_root() {
  if [[ ${EUID} -ne 0 ]]; then
    echo "run as root" >&2
    exit 1
  fi
}

log() {
  printf '[x-cloud-flow] %s\n' "$*"
}

die() {
  printf '[x-cloud-flow] ERROR: %s\n' "$*" >&2
  exit 1
}

command_exists() {
  command -v "$1" >/dev/null 2>&1
}

sync_source() {
  rm -rf "${APP_DIR}.tmp"
  mkdir -p "${APP_DIR}.tmp"

  if [[ -n "${SOURCE_DIR}" ]]; then
    [[ -d "${SOURCE_DIR}" ]] || die "source dir not found: ${SOURCE_DIR}"
    tar -C "${SOURCE_DIR}" --exclude .git --exclude .xcloudflow -cf - . | tar -C "${APP_DIR}.tmp" -xf -
  else
    rm -rf "${APP_DIR}.tmp"
    if [[ -d "${APP_DIR}/.git" ]]; then
      git -C "${APP_DIR}" fetch --depth 1 origin "${BRANCH}"
      git -C "${APP_DIR}" checkout -f "${BRANCH}"
      git -C "${APP_DIR}" reset --hard "origin/${BRANCH}"
      return
    fi
    git clone --depth 1 --branch "${BRANCH}" "${REPO_URL}" "${APP_DIR}.tmp"
  fi

  rm -rf "${APP_DIR}"
  mv "${APP_DIR}.tmp" "${APP_DIR}"
}

write_env_file() {
  mkdir -p "$(dirname "${ENV_FILE}")"
  cat >"${ENV_FILE}" <<EOF
PORT=${PORT}
XCF_TENANT_ID=${TENANT_ID}
EOF
  if [[ -n "${DATABASE_URL}" ]]; then
    printf 'DATABASE_URL=%s\n' "${DATABASE_URL}" >>"${ENV_FILE}"
  fi
}

write_caddy_config() {
  [[ "${SKIP_CADDY}" == "1" ]] && return 0
  mkdir -p "${CADDY_DIR}"
  cat >"${CADDY_FILE}" <<EOF
${DOMAIN} {
    encode zstd gzip
    reverse_proxy 127.0.0.1:${PORT}
}
EOF
  if command_exists systemctl && systemctl list-unit-files caddy.service >/dev/null 2>&1; then
    systemctl reload caddy || systemctl restart caddy
  fi
}

remove_caddy_config() {
  [[ "${SKIP_CADDY}" == "1" ]] && return 0
  rm -f "${CADDY_FILE}"
  if command_exists systemctl && systemctl list-unit-files caddy.service >/dev/null 2>&1; then
    systemctl reload caddy || true
  fi
}

build_binary() {
  mkdir -p "${APP_DIR}/.build"
  if command_exists go; then
    (cd "${APP_DIR}" && go build -o .build/xcloud-server ./cmd/xcloud-server)
    return
  fi
  if command_exists docker; then
    docker run --rm \
      -v "${APP_DIR}:/src" \
      -w /src \
      golang:1.24-alpine \
      sh -lc 'apk add --no-cache git >/dev/null && go build -o /src/.build/xcloud-server ./cmd/xcloud-server'
    return
  fi
  die "neither go nor docker is available to build process-mode binary"
}

install_process_mode() {
  sync_source
  write_env_file
  build_binary
  install -m 755 "${APP_DIR}/.build/xcloud-server" "/usr/local/bin/${SERVICE_NAME}"
  cat >"/etc/systemd/system/${SERVICE_NAME}.service" <<EOF
[Unit]
Description=XCloudFlow process service
After=network.target

[Service]
Type=simple
EnvironmentFile=${ENV_FILE}
WorkingDirectory=${APP_DIR}
ExecStart=/usr/local/bin/${SERVICE_NAME} --addr 127.0.0.1:${PORT} --workspace ${APP_DIR}
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF
  systemctl daemon-reload
  systemctl enable --now "${SERVICE_NAME}"
  write_caddy_config
}

uninstall_process_mode() {
  if command_exists systemctl && systemctl list-unit-files "${SERVICE_NAME}.service" >/dev/null 2>&1; then
    systemctl disable --now "${SERVICE_NAME}" || true
  fi
  rm -f "/etc/systemd/system/${SERVICE_NAME}.service"
  systemctl daemon-reload || true
  rm -f "/usr/local/bin/${SERVICE_NAME}"
  rm -rf "${APP_DIR}"
  rm -f "${ENV_FILE}"
  remove_caddy_config
}

status_process_mode() {
  systemctl status "${SERVICE_NAME}" --no-pager || true
  [[ -f "${CADDY_FILE}" ]] && log "caddy config: ${CADDY_FILE}"
  curl -fsS "http://127.0.0.1:${PORT}/healthz" || true
}

install_docker_mode() {
  command_exists docker || die "docker not found"
  sync_source
  write_env_file
  docker build -t "${IMAGE}" "${APP_DIR}"
  docker rm -f "${SERVICE_NAME}" >/dev/null 2>&1 || true
  docker run -d \
    --name "${SERVICE_NAME}" \
    --restart unless-stopped \
    --env-file "${ENV_FILE}" \
    -p "127.0.0.1:${PORT}:8080" \
    "${IMAGE}"
  write_caddy_config
}

uninstall_docker_mode() {
  docker rm -f "${SERVICE_NAME}" >/dev/null 2>&1 || true
  docker rmi "${IMAGE}" >/dev/null 2>&1 || true
  rm -rf "${APP_DIR}"
  rm -f "${ENV_FILE}"
  remove_caddy_config
}

status_docker_mode() {
  docker ps -a --filter "name=${SERVICE_NAME}" || true
  [[ -f "${CADDY_FILE}" ]] && log "caddy config: ${CADDY_FILE}"
  curl -fsS "http://127.0.0.1:${PORT}/healthz" || true
}

install_cloud_run_mode() {
  command_exists gcloud || die "gcloud not found"
  [[ -n "${GCP_PROJECT}" ]] || die "--project is required for cloud-run mode"
  sync_source
  gcloud run deploy "${SERVICE_NAME}" \
    --project "${GCP_PROJECT}" \
    --region "${REGION}" \
    --source "${APP_DIR}" \
    --allow-unauthenticated \
    --set-env-vars "XCF_TENANT_ID=${TENANT_ID}"$( [[ -n "${DATABASE_URL}" ]] && printf ',DATABASE_URL=%s' "${DATABASE_URL}" )
}

uninstall_cloud_run_mode() {
  command_exists gcloud || die "gcloud not found"
  [[ -n "${GCP_PROJECT}" ]] || die "--project is required for cloud-run mode"
  gcloud run services delete "${SERVICE_NAME}" --project "${GCP_PROJECT}" --region "${REGION}" --quiet
}

status_cloud_run_mode() {
  command_exists gcloud || die "gcloud not found"
  [[ -n "${GCP_PROJECT}" ]] || die "--project is required for cloud-run mode"
  gcloud run services describe "${SERVICE_NAME}" --project "${GCP_PROJECT}" --region "${REGION}"
}

DOMAIN="${1:-}"
if [[ -z "${DOMAIN}" || "${DOMAIN}" == "--help" || "${DOMAIN}" == "-h" ]]; then
  usage
  exit 0
fi
shift

MODE="process"
ACTION="install"
APP_DIR="/opt/x-cloud-flow.svc.plus"
SOURCE_DIR=""
REPO_URL="https://github.com/cloud-neutral-toolkit/x-cloud-flow.svc.plus.git"
BRANCH="main"
PORT="18083"
IMAGE="x-cloud-flow-svc-plus:local"
GCP_PROJECT=""
REGION="us-central1"
SERVICE_NAME="x-cloud-flow-svc-plus"
DATABASE_URL=""
TENANT_ID="default"
ENV_FILE="/etc/x-cloud-flow.svc.plus.env"
CADDY_DIR="/etc/caddy/conf.d"
SKIP_CADDY="0"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --mode) MODE="$2"; shift 2 ;;
    --action) ACTION="$2"; shift 2 ;;
    --app-dir) APP_DIR="$2"; shift 2 ;;
    --source-dir) SOURCE_DIR="$2"; shift 2 ;;
    --repo-url) REPO_URL="$2"; shift 2 ;;
    --branch) BRANCH="$2"; shift 2 ;;
    --port) PORT="$2"; shift 2 ;;
    --image) IMAGE="$2"; shift 2 ;;
    --project) GCP_PROJECT="$2"; shift 2 ;;
    --region) REGION="$2"; shift 2 ;;
    --service-name) SERVICE_NAME="$2"; shift 2 ;;
    --database-url) DATABASE_URL="$2"; shift 2 ;;
    --tenant-id) TENANT_ID="$2"; shift 2 ;;
    --env-file) ENV_FILE="$2"; shift 2 ;;
    --caddy-dir) CADDY_DIR="$2"; shift 2 ;;
    --skip-caddy) SKIP_CADDY="1"; shift ;;
    --help|-h) usage; exit 0 ;;
    *) die "unknown argument: $1" ;;
  esac
done

CADDY_FILE="${CADDY_DIR}/${DOMAIN}.caddy"

require_root

case "${MODE}" in
  process)
    case "${ACTION}" in
      install) install_process_mode ;;
      uninstall) uninstall_process_mode ;;
      status) status_process_mode ;;
      *) die "unsupported action for process mode: ${ACTION}" ;;
    esac
    ;;
  docker)
    case "${ACTION}" in
      install) install_docker_mode ;;
      uninstall) uninstall_docker_mode ;;
      status) status_docker_mode ;;
      *) die "unsupported action for docker mode: ${ACTION}" ;;
    esac
    ;;
  cloud-run)
    case "${ACTION}" in
      install) install_cloud_run_mode ;;
      uninstall) uninstall_cloud_run_mode ;;
      status) status_cloud_run_mode ;;
      *) die "unsupported action for cloud-run mode: ${ACTION}" ;;
    esac
    ;;
  *)
    die "unsupported mode: ${MODE}"
    ;;
esac

log "mode=${MODE} action=${ACTION} complete"
