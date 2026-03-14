.PHONY: help all xcloud-init xcloud-build xcloud-run xcloud-up xcloud-down xcloud-export xcloud-import xcloud-ansible xconfig-init xconfig-build xconfig-run xconfig-playbook xconfig-agent-build xconfig-agent-run xconfig-agent-install codex-init codex-status codex-home codex-render-config codex-exec xcloudflow-build xcloudflow-mcp xcloudflow-agent-spec xcloudflow-openclaw-register

ENV ?= sit

help:
	@echo "🚀 Project Targets"
	@echo "  make xcloud-build          # 构建 Go 版 XCloud CLI"
	@echo "  make xcloud-run ENV=sit    # 运行 XCloud CLI (示例)"
	@echo "  make xconfig-build         # 构建 Go 版 Xconfig"
	@echo "  make xconfig-playbook      # 使用默认示例执行 playbook"
	@echo "  make xconfig-agent-build   # 构建 Rust 版 xconfig-agent"
	@echo "  make xconfig-agent-run     # 运行 xconfig-agent oneshot"
	@echo "  make codex-init            # 初始化 third_party/codex 子模块"
	@echo "  make codex-home            # 初始化项目级 CODEX_HOME"
	@echo "  make codex-render-config   # 重新渲染 Codex config.toml / MCP 配置"
	@echo "  make codex-exec            # 通过包装层执行 codex exec"
	@echo "  make xcloudflow-build      # 构建 xcloudflow CLI / MCP server"
	@echo "  make xcloudflow-mcp        # 启动 XCloudFlow MCP server"
	@echo "  make xcloudflow-agent-spec # 生成 Codex/OpenClaw IaC agent spec"
	@echo "  make xcloudflow-openclaw-register # 真实注册/更新 x-automation-agent 到 OpenClaw Gateway"

all: help

build: xcloud-build xconfig-build xconfig-agent-build

xcloudflow-build:
	go build ./cmd/xcloudflow ./cmd/xcloud-server

xcloudflow-mcp:
	go run ./cmd/xcloudflow mcp serve --addr :8808

xcloudflow-agent-spec:
	go run ./cmd/xcloudflow agent spec --config examples/stackflow/demo.yaml --env prod --env-file .env

xcloudflow-openclaw-register:
	go run ./cmd/xcloudflow agent register-openclaw --env-file .env

xcloud-init:
	$(MAKE) -C xcloud-cli init

xcloud-build:
	$(MAKE) -C xcloud-cli build

xcloud-run:
	$(MAKE) -C xcloud-cli run ENV=$(ENV)

xcloud-up:
	$(MAKE) -C xcloud-cli up ENV=$(ENV)

xcloud-down:
	$(MAKE) -C xcloud-cli down ENV=$(ENV)

xcloud-export:
	$(MAKE) -C xcloud-cli export ENV=$(ENV)

xcloud-import:
	$(MAKE) -C xcloud-cli import ENV=$(ENV)

xcloud-ansible:
	$(MAKE) -C xcloud-cli ansible ENV=$(ENV)

xconfig-init:
	$(MAKE) -C xconfig init

xconfig-build:
	$(MAKE) -C xconfig build

xconfig-run:
	$(MAKE) -C xconfig run

xconfig-playbook:
	$(MAKE) -C xconfig playbook

xconfig-agent-build:
	$(MAKE) -C xconfig-agent build

xconfig-agent-run:
	$(MAKE) -C xconfig-agent run

xconfig-agent-install:
	$(MAKE) -C xconfig-agent install

codex-init:
	git submodule update --init --recursive third_party/codex

codex-status:
	git submodule status --recursive third_party/codex

codex-home:
	./scripts/codex/init-home.sh

codex-render-config:
	./scripts/codex/render-config.sh

codex-exec:
	./scripts/codex/run-exec.sh
