package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"xcloudflow/internal/a2a"
	"xcloudflow/internal/defaults"
	"xcloudflow/internal/mcp"
	"xcloudflow/internal/openclaw"
	"xcloudflow/internal/store"
)

func mcpCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "MCP server/client utilities",
	}
	cmd.AddCommand(mcpServeCmd())
	cmd.AddCommand(mcpServersCmd())
	return cmd
}

func mcpServeCmd() *cobra.Command {
	var addr string
	var workspace string
	var envFile string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run MCP HTTP server (Cloud Run friendly)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if addr == "" {
				if p := os.Getenv("PORT"); p != "" {
					addr = ":" + p
				} else {
					addr = ":" + defaults.DefaultMCPPort
				}
			}

			var st *store.Store
			if rf.DSN != "" {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				s, err := store.Open(ctx, rf.DSN)
				if err != nil {
					return err
				}
				st = s
				defer st.Close()
			}

			srv := mcp.NewServer(mcp.ServerOptions{
				Store:        st,
				WorkspaceDir: workspace,
				EnvFile:      envFile,
				MCPURL:       mcpURLForAddr(addr),
			})
			a2aServer := a2a.NewService(resolveAgentIDForServe(envFile), "automation")

			mux := http.NewServeMux()
			mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
			mux.Handle("/a2a/v1/", a2aServer.Handler())
			mux.Handle("/mcp", srv)

			fmt.Println("listening on", addr)
			return http.ListenAndServe(addr, mux)
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "", "listen addr (default :8808, or :$PORT)")
	cmd.Flags().StringVar(&workspace, "workspace", "", "workspace root for Codex/OpenClaw bridge defaults (default: cwd)")
	cmd.Flags().StringVar(&envFile, "env-file", "", "path to local mixed .env/.json-style gateway secrets file (default: <workspace>/.env)")
	return cmd
}

func resolveAgentIDForServe(envFile string) string {
	if value := strings.TrimSpace(os.Getenv("OPENCLAW_AGENT_ID")); value != "" {
		return value
	}
	if strings.TrimSpace(envFile) != "" {
		env, err := openclaw.LoadGatewayEnv(envFile)
		if err == nil && strings.TrimSpace(env.AgentID) != "" {
			return env.AgentID
		}
	}
	return defaults.OpenClawAgentID()
}

func mcpServersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "servers",
		Short: "Manage external MCP servers (registry in PostgreSQL)",
	}
	cmd.AddCommand(mcpServersAddCmd())
	cmd.AddCommand(mcpServersListCmd())
	cmd.AddCommand(mcpServersRefreshCmd())
	return cmd
}

func mcpServersAddCmd() *cobra.Command {
	var name, baseURL, kind, authType, audience string
	var enabled bool
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Upsert an MCP server into xcf.mcp_servers",
		RunE: func(cmd *cobra.Command, args []string) error {
			dsn, err := dsnOrErr()
			if err != nil {
				return err
			}
			if name == "" || baseURL == "" {
				return fmt.Errorf("missing required: --name --url")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			st, err := store.Open(ctx, dsn)
			if err != nil {
				return err
			}
			defer st.Close()
			_, err = st.UpsertMCPServer(ctx, store.MCPServer{
				Name:     name,
				BaseURL:  baseURL,
				Kind:     kind,
				AuthType: authType,
				Audience: audience,
				Enabled:  enabled,
			})
			return err
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "server name (unique)")
	cmd.Flags().StringVar(&baseURL, "url", "", "server MCP endpoint URL (e.g. https://service/mcp)")
	cmd.Flags().StringVar(&kind, "kind", "generic", "server kind (optional)")
	cmd.Flags().StringVar(&authType, "auth", "none", "auth type: none|bearer|oidc (stored only; enforcement in client)")
	cmd.Flags().StringVar(&audience, "audience", "", "OIDC audience (optional)")
	cmd.Flags().BoolVar(&enabled, "enabled", true, "enable this server")
	return cmd
}

func mcpServersListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List MCP servers",
		RunE: func(cmd *cobra.Command, args []string) error {
			dsn, err := dsnOrErr()
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			st, err := store.Open(ctx, dsn)
			if err != nil {
				return err
			}
			defer st.Close()
			srvs, err := st.ListMCPServers(ctx)
			if err != nil {
				return err
			}
			b, _ := json.MarshalIndent(srvs, "", "  ")
			fmt.Println(string(b))
			return nil
		},
	}
	return cmd
}

func mcpServersRefreshCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "refresh-tools",
		Short: "Fetch tools/list from each enabled MCP server and cache into xcf.mcp_tools_cache",
		RunE: func(cmd *cobra.Command, args []string) error {
			dsn, err := dsnOrErr()
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			st, err := store.Open(ctx, dsn)
			if err != nil {
				return err
			}
			defer st.Close()

			srvs, err := st.ListMCPServers(ctx)
			if err != nil {
				return err
			}
			for _, srv := range srvs {
				if !srv.Enabled {
					continue
				}
				c := mcp.NewClientWithOptions(srv.BaseURL, clientOptionsForServer(srv))
				tools, err := c.ToolsList(ctx)
				if err != nil {
					return fmt.Errorf("server %s: %w", srv.Name, err)
				}
				tb, _ := json.Marshal(tools)
				if err := st.UpdateMCPToolsCache(ctx, srv.ServerID, tb, ""); err != nil {
					return err
				}
			}
			fmt.Println("ok: tools cache refreshed")
			return nil
		},
	}
	return cmd
}

func mcpURLForAddr(addr string) string {
	if explicit := strings.TrimSpace(os.Getenv("XCF_MCP_URL")); explicit != "" {
		return explicit
	}
	if strings.HasPrefix(addr, ":") {
		return "http://127.0.0.1" + addr + "/mcp"
	}
	return defaults.MCPURL()
}

func clientOptionsForServer(srv store.MCPServer) mcp.ClientOptions {
	if strings.EqualFold(srv.AuthType, "bearer") && srv.Name == defaults.DefaultSSHMCPServerName {
		return mcp.ClientOptions{BearerToken: defaults.SSHMCPBearerToken()}
	}
	return mcp.ClientOptions{}
}
