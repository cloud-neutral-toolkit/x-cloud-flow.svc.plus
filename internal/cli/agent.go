package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"xcloudflow/internal/codex"
	"xcloudflow/internal/defaults"
	"xcloudflow/internal/openclaw"
	"xcloudflow/internal/stackflow"
	"xcloudflow/internal/store"
)

func agentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Agent mode (stateless worker, state/memory in PostgreSQL)",
	}
	cmd.AddCommand(agentRunCmd())
	cmd.AddCommand(agentSpecCmd())
	cmd.AddCommand(agentShellEnvCmd())
	cmd.AddCommand(agentInitCodexHomeCmd())
	cmd.AddCommand(agentOpenClawRegisterCmd())
	return cmd
}

func agentRunCmd() *cobra.Command {
	var configPath string
	var env string
	var interval time.Duration
	var once bool
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run validate + dns-plan in a loop and persist runs to PostgreSQL",
		RunE: func(cmd *cobra.Command, args []string) error {
			dsn, err := dsnOrErr()
			if err != nil {
				return err
			}
			if configPath == "" {
				return fmt.Errorf("missing --config")
			}
			if once {
				interval = 0
			}

			ctx := context.Background()
			st, err := store.Open(ctx, dsn)
			if err != nil {
				return err
			}
			defer st.Close()

			doOnce := func() error {
				b, err := os.ReadFile(configPath)
				if err != nil {
					return err
				}
				cfg, err := stackflow.LoadYAML(b)
				if err != nil {
					return err
				}
				stackName, err := stackflow.StackName(cfg)
				if err != nil {
					return err
				}
				if env != "" {
					cfg = stackflow.ApplyEnvOverrides(cfg, env)
				}

				runID, err := st.CreateRun(ctx, store.Run{
					Stack:     stackName,
					Env:       env,
					Phase:     "validate+dns-plan",
					Status:    "running",
					ConfigRef: configPath,
				})
				if err != nil {
					return err
				}

				val, err := stackflow.Validate(cfg)
				if err != nil {
					_ = st.FinishRun(ctx, runID, "failed", []byte(fmt.Sprintf(`{"error":%q}`, err.Error())))
					return err
				}

				plan, err := stackflow.DNSPlan(cfg, env)
				if err != nil {
					_ = st.FinishRun(ctx, runID, "failed", []byte(fmt.Sprintf(`{"error":%q}`, err.Error())))
					return err
				}

				out := map[string]any{
					"validate": val,
					"dnsPlan":  plan,
				}
				rb, _ := json.Marshal(out)
				if err := st.FinishRun(ctx, runID, "ok", rb); err != nil {
					return err
				}
				return nil
			}

			if interval == 0 {
				return doOnce()
			}
			t := time.NewTicker(interval)
			defer t.Stop()
			for {
				if err := doOnce(); err != nil {
					fmt.Fprintln(os.Stderr, "run failed:", err)
				}
				<-t.C
			}
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "", "Path to StackFlow YAML file")
	cmd.Flags().StringVar(&env, "env", "", "Optional env name (global.environments.<env>)")
	cmd.Flags().DurationVar(&interval, "interval", 10*time.Minute, "Run interval (0 to run once)")
	cmd.Flags().BoolVar(&once, "once", false, "Run once and exit")
	return cmd
}

func agentSpecCmd() *cobra.Command {
	var configPath string
	var env string
	var task string
	var envFile string
	var agentID string
	var mcpURL string
	var includeSecrets bool

	cmd := &cobra.Command{
		Use:   "spec",
		Short: "Build a Codex/OpenClaw IaC agent spec from local StackFlow config",
		RunE: func(cmd *cobra.Command, args []string) error {
			workspace, err := os.Getwd()
			if err != nil {
				return err
			}
			workspace = defaults.WorkspaceDir(workspace)

			var (
				plan       map[string]any
				configYAML string
			)
			if configPath != "" {
				b, err := os.ReadFile(configPath)
				if err != nil {
					return err
				}
				configYAML = string(b)
				cfg, err := stackflow.LoadYAML(b)
				if err != nil {
					return err
				}
				plan, err = stackflow.IACPlan(cfg, env)
				if err != nil {
					return err
				}
			}

			codexCfg := codex.DefaultBridgeConfig(workspace, mcpURL)
			var planJSON []byte
			if len(plan) > 0 {
				planJSON, _ = json.Marshal(plan)
			}
			manifest := codex.BuildManifest(codexCfg, codex.TaskRequest{
				Task:       task,
				ConfigPath: configPath,
				ConfigYAML: configYAML,
				Env:        env,
				IACPlan:    planJSON,
			})

			out := map[string]any{
				"kind":  "xcloudflow.agent.spec/v1",
				"iac":   plan,
				"codex": manifest,
			}

			if envFile != "" {
				gwEnv, err := openclaw.LoadGatewayEnv(envFile)
				if err != nil {
					return err
				}
				out["openclaw"] = openclaw.BuildRegistration(gwEnv, codexCfg, openclaw.RegistrationOptions{
					AgentID:        agentID,
					Workspace:      workspace,
					MCPURL:         manifest.MCPURL,
					IncludeSecrets: includeSecrets,
				})
			}

			b, _ := json.MarshalIndent(out, "", "  ")
			fmt.Println(string(b))
			return nil
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "", "Path to StackFlow YAML file")
	cmd.Flags().StringVar(&env, "env", "", "Optional environment override")
	cmd.Flags().StringVar(&task, "task", "", "High-level IaC task for the Codex runtime")
	cmd.Flags().StringVar(&envFile, "env-file", ".env", "Path to local mixed .env/.json-style gateway secrets file")
	cmd.Flags().StringVar(&agentID, "agent-id", defaults.OpenClawAgentID(), "OpenClaw agent id to emit when --env-file is used")
	cmd.Flags().StringVar(&mcpURL, "mcp-url", defaults.MCPURL(), "XCloudFlow MCP endpoint exposed to Codex/OpenClaw")
	cmd.Flags().BoolVar(&includeSecrets, "with-secrets", false, "Include resolved secrets in the registration payload")
	return cmd
}

func agentShellEnvCmd() *cobra.Command {
	var envFile string
	var workspace string
	var mcpURL string

	cmd := &cobra.Command{
		Use:   "shell-env",
		Short: "Print shell exports for the Codex/OpenClaw runtime wrapper",
		RunE: func(cmd *cobra.Command, args []string) error {
			workspace = defaults.WorkspaceDir(workspace)
			cfg := codex.DefaultBridgeConfig(workspace, mcpURL)

			var env openclaw.GatewayEnv
			if envFile != "" {
				loaded, err := openclaw.LoadGatewayEnv(envFile)
				if err != nil {
					return err
				}
				env = loaded
			}

			exports := map[string]string{
				"CODEX_HOME":               firstNonEmpty(env.CodexHome, cfg.HomeDir),
				"XCF_MCP_PORT":             cfg.MCPPort,
				"XCF_MCP_URL":              cfg.MCPURL,
				"XCF_TERRAFORM_REPO":       firstNonEmpty(env.TerraformRepo, cfg.TerraformRepo),
				"XCF_PLAYBOOKS_REPO":       firstNonEmpty(env.PlaybooksRepo, cfg.PlaybooksRepo),
				"OPENAI_BASE_URL":          env.AIGatewayURL,
				"OPENAI_API_KEY":           env.AIGatewayAPIKey,
				"XCF_SSH_MCP_URL":          firstNonEmpty(env.SSHMCPURL, cfg.SSHMCPURL),
				"XCF_SSH_MCP_BEARER_TOKEN": firstNonEmpty(env.SSHMCPToken, os.Getenv("XCF_SSH_MCP_BEARER_TOKEN")),
			}
			order := []string{
				"CODEX_HOME",
				"XCF_MCP_PORT",
				"XCF_MCP_URL",
				"XCF_TERRAFORM_REPO",
				"XCF_PLAYBOOKS_REPO",
				"OPENAI_BASE_URL",
				"OPENAI_API_KEY",
				"XCF_SSH_MCP_URL",
				"XCF_SSH_MCP_BEARER_TOKEN",
			}
			for _, key := range order {
				if value := exports[key]; value != "" {
					fmt.Printf("export %s=%s\n", key, strconv.Quote(value))
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&envFile, "env-file", ".env", "Path to local mixed .env/.json-style gateway secrets file")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Workspace path used to resolve defaults")
	cmd.Flags().StringVar(&mcpURL, "mcp-url", defaults.MCPURL(), "XCloudFlow MCP endpoint exposed to Codex/OpenClaw")
	return cmd
}

func agentInitCodexHomeCmd() *cobra.Command {
	var envFile string
	var workspace string
	var mcpURL string

	cmd := &cobra.Command{
		Use:   "init-codex-home",
		Short: "Initialize the project-level CODEX_HOME layout and config.toml",
		RunE: func(cmd *cobra.Command, args []string) error {
			workspace = defaults.WorkspaceDir(workspace)
			cfg := codex.DefaultBridgeConfig(workspace, mcpURL)
			if envFile != "" {
				env, err := openclaw.LoadGatewayEnv(envFile)
				if err == nil {
					cfg.HomeDir = firstNonEmpty(env.CodexHome, cfg.HomeDir)
					cfg.TerraformRepo = firstNonEmpty(env.TerraformRepo, cfg.TerraformRepo)
					cfg.PlaybooksRepo = firstNonEmpty(env.PlaybooksRepo, cfg.PlaybooksRepo)
					cfg.SSHMCPURL = firstNonEmpty(env.SSHMCPURL, cfg.SSHMCPURL)
				}
			}
			if err := codex.InitHome(cfg); err != nil {
				return err
			}
			fmt.Println(cfg.HomeDir)
			return nil
		},
	}
	cmd.Flags().StringVar(&envFile, "env-file", ".env", "Path to local mixed .env/.json-style gateway secrets file")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Workspace path used to resolve defaults")
	cmd.Flags().StringVar(&mcpURL, "mcp-url", defaults.MCPURL(), "XCloudFlow MCP endpoint exposed to Codex/OpenClaw")
	return cmd
}

func agentOpenClawRegisterCmd() *cobra.Command {
	var envFile string
	var agentID string
	var workspace string
	var mcpURL string
	var includeSecrets bool

	cmd := &cobra.Command{
		Use:   "register-openclaw",
		Short: "Generate an OpenClaw Gateway patch for the embedded Codex IaC agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			workspace = defaults.WorkspaceDir(workspace)

			env, err := openclaw.LoadGatewayEnv(envFile)
			if err != nil {
				return err
			}
			codexCfg := codex.DefaultBridgeConfig(workspace, mcpURL)
			spec := openclaw.BuildRegistration(env, codexCfg, openclaw.RegistrationOptions{
				AgentID:        agentID,
				Workspace:      workspace,
				MCPURL:         codexCfg.MCPURL,
				IncludeSecrets: includeSecrets,
			})
			b, _ := json.MarshalIndent(spec, "", "  ")
			fmt.Println(string(b))
			return nil
		},
	}
	cmd.Flags().StringVar(&envFile, "env-file", ".env", "Path to local mixed .env/.json-style gateway secrets file")
	cmd.Flags().StringVar(&agentID, "agent-id", defaults.OpenClawAgentID(), "OpenClaw agent id to register")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Workspace path exposed to the OpenClaw ACP/Codex runtime")
	cmd.Flags().StringVar(&mcpURL, "mcp-url", defaults.MCPURL(), "XCloudFlow MCP endpoint exposed to OpenClaw")
	cmd.Flags().BoolVar(&includeSecrets, "with-secrets", false, "Include resolved secrets in the registration payload")
	return cmd
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
