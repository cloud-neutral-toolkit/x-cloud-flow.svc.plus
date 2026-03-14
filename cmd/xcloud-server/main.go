package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"xcloudflow/internal/a2a"
	"xcloudflow/internal/mcp"
	"xcloudflow/internal/openclaw"
	"xcloudflow/internal/store"
)

// xcloud-server is the stateless control plane entrypoint intended for Cloud Run.
//
// Endpoints:
// - GET  /healthz
// - POST /mcp   (minimal JSON-RPC handler)
//
// State/memory is persisted in PostgreSQL (postgresql.svc.plus) when DATABASE_URL is provided.
func main() {
	var addr string
	var workspace string
	var envFile string
	flag.StringVar(&addr, "addr", "", "listen address (default :$PORT or :8080)")
	flag.StringVar(&workspace, "workspace", "", "workspace root for Codex/OpenClaw bridge defaults")
	flag.StringVar(&envFile, "env-file", "", "path to local mixed .env/.json-style gateway secrets file")
	flag.Parse()

	if addr == "" {
		if p := os.Getenv("PORT"); p != "" {
			addr = ":" + p
		} else {
			addr = ":8080"
		}
	}

	dsn := os.Getenv("DATABASE_URL")
	var st *store.Store
	if dsn != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		s, err := store.Open(ctx, dsn)
		if err != nil {
			fmt.Fprintln(os.Stderr, "db connect:", err)
			os.Exit(1)
		}
		st = s
		defer st.Close()
	}

	srv := mcp.NewServer(mcp.ServerOptions{
		Store:        st,
		WorkspaceDir: workspace,
		EnvFile:      envFile,
	})
	a2aServer := a2a.NewService(resolveAgentID(envFile), "automation")

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.Handle("/a2a/v1/", a2aServer.Handler())
	mux.Handle("/mcp", srv)

	fmt.Println("listening on", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func resolveAgentID(envFile string) string {
	if value := strings.TrimSpace(os.Getenv("OPENCLAW_AGENT_ID")); value != "" {
		return value
	}
	if strings.TrimSpace(envFile) != "" {
		env, err := openclaw.LoadGatewayEnv(envFile)
		if err == nil && strings.TrimSpace(env.AgentID) != "" {
			return env.AgentID
		}
	}
	return "x-automation-agent"
}
