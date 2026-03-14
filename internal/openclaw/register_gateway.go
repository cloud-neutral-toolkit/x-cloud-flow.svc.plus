package openclaw

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type RegisterAgentOptions struct {
	Command   string
	AgentID   string
	AgentName string
	Workspace string
	Model     string
}

type RegisterAgentResult struct {
	Operation string `json:"operation"`
	AgentID   string `json:"agent_id"`
	Workspace string `json:"workspace"`
	Model     string `json:"model,omitempty"`
}

func RegisterOrUpdateAgent(ctx context.Context, env GatewayEnv, opts RegisterAgentOptions) (RegisterAgentResult, error) {
	command := strings.TrimSpace(opts.Command)
	if command == "" {
		command = "openclaw"
	}
	agentID := normalizeAgentID(firstNonEmpty(opts.AgentID, env.AgentID))
	if agentID == "" {
		agentID = "x-automation-agent"
	}
	desiredName := strings.TrimSpace(firstNonEmpty(opts.AgentName, env.AgentName))
	if desiredName == "" {
		desiredName = "XCloudFlow Automation"
	}
	workspace := strings.TrimSpace(firstNonEmpty(opts.Workspace, env.AgentWorkspace))
	if workspace == "" {
		workspace = "."
	}
	workspace, _ = filepath.Abs(workspace)
	model := strings.TrimSpace(firstNonEmpty(opts.Model, env.AgentModel))

	listRaw, err := runGatewayCall(ctx, command, env, "agents.list", map[string]any{})
	if err != nil {
		return RegisterAgentResult{}, err
	}
	var listPayload struct {
		Agents []struct {
			ID string `json:"id"`
		} `json:"agents"`
	}
	if err := json.Unmarshal(listRaw, &listPayload); err != nil {
		return RegisterAgentResult{}, fmt.Errorf("parse agents.list response: %w", err)
	}

	exists := false
	for _, item := range listPayload.Agents {
		if normalizeAgentID(item.ID) == agentID {
			exists = true
			break
		}
	}

	if exists {
		if _, err := runGatewayCall(ctx, command, env, "agents.update", buildUpdateParams(agentID, desiredName, workspace, model)); err != nil {
			return RegisterAgentResult{}, err
		}
		return RegisterAgentResult{Operation: "updated", AgentID: agentID, Workspace: workspace, Model: model}, nil
	}

	createName := registrationCreateName(agentID, desiredName)
	createRaw, err := runGatewayCall(ctx, command, env, "agents.create", map[string]any{
		"name":      createName,
		"workspace": workspace,
	})
	if err != nil {
		return RegisterAgentResult{}, err
	}
	var createPayload struct {
		AgentID string `json:"agentId"`
	}
	if err := json.Unmarshal(createRaw, &createPayload); err != nil {
		return RegisterAgentResult{}, fmt.Errorf("parse agents.create response: %w", err)
	}
	createdID := normalizeAgentID(createPayload.AgentID)
	if createdID == "" {
		createdID = normalizeAgentID(createName)
	}
	if createdID != agentID {
		return RegisterAgentResult{}, fmt.Errorf("unexpected created agent id %q (expected %q)", createdID, agentID)
	}
	if desiredName != createName || model != "" {
		if _, err := runGatewayCall(ctx, command, env, "agents.update", buildUpdateParams(agentID, desiredName, workspace, model)); err != nil {
			return RegisterAgentResult{}, err
		}
	}
	return RegisterAgentResult{Operation: "created", AgentID: agentID, Workspace: workspace, Model: model}, nil
}

func buildUpdateParams(agentID, desiredName, workspace, model string) map[string]any {
	params := map[string]any{
		"agentId":   agentID,
		"name":      desiredName,
		"workspace": workspace,
	}
	if strings.TrimSpace(model) != "" {
		params["model"] = strings.TrimSpace(model)
	}
	return params
}

func runGatewayCall(ctx context.Context, command string, env GatewayEnv, method string, params map[string]any) ([]byte, error) {
	body, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	args := []string{"gateway", "call", method, "--json", "--params", string(body)}
	if strings.TrimSpace(env.RemoteURL) != "" {
		args = append(args, "--url", strings.TrimSpace(env.RemoteURL))
		if strings.TrimSpace(env.RemoteToken) == "" && strings.TrimSpace(env.RemotePassword) == "" {
			return nil, fmt.Errorf("gateway url override requires token or password")
		}
	}
	if strings.TrimSpace(env.RemoteToken) != "" {
		args = append(args, "--token", strings.TrimSpace(env.RemoteToken))
	}
	if strings.TrimSpace(env.RemotePassword) != "" {
		args = append(args, "--password", strings.TrimSpace(env.RemotePassword))
	}

	cmd := exec.CommandContext(ctx, command, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("openclaw gateway call %s failed: %w (%s)", method, err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

func registrationCreateName(agentID, desiredName string) string {
	if normalizeAgentID(desiredName) == agentID {
		return desiredName
	}
	return agentID
}

func normalizeAgentID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.NewReplacer(" ", "-", "/", "-", ":", "-", ".", "-").Replace(value)
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_')
	})
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "-")
}
