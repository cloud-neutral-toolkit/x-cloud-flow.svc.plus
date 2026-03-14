package openclaw

import (
	"bufio"
	"os"
	"strings"
)

type GatewayEnv struct {
	RemoteURL       string            `json:"remoteUrl,omitempty"`
	RemoteToken     string            `json:"remoteToken,omitempty"`
	RemotePassword  string            `json:"remotePassword,omitempty"`
	AIGatewayURL    string            `json:"aiGatewayUrl,omitempty"`
	AIGatewayAPIKey string            `json:"aiGatewayApiKey,omitempty"`
	AgentID         string            `json:"agentId,omitempty"`
	AgentName       string            `json:"agentName,omitempty"`
	AgentWorkspace  string            `json:"agentWorkspace,omitempty"`
	AgentModel      string            `json:"agentModel,omitempty"`
	SSHMCPURL       string            `json:"sshMcpUrl,omitempty"`
	SSHMCPToken     string            `json:"sshMcpBearerToken,omitempty"`
	TerraformRepo   string            `json:"terraformRepo,omitempty"`
	PlaybooksRepo   string            `json:"playbooksRepo,omitempty"`
	CodexHome       string            `json:"codexHome,omitempty"`
	Raw             map[string]string `json:"raw,omitempty"`
}

func LoadGatewayEnv(path string) (GatewayEnv, error) {
	f, err := os.Open(path)
	if err != nil {
		return GatewayEnv{}, err
	}
	defer f.Close()

	env := GatewayEnv{Raw: map[string]string{}}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := splitKVLine(line)
		if !ok {
			continue
		}
		env.Raw[key] = value

		switch normalizeKey(key) {
		case "remote", "remoteurl", "gatewayremoteurl", "openclawgatewayurl":
			env.RemoteURL = firstNonEmpty(env.RemoteURL, value)
		case "remotetoken", "gatewayauthtoken", "openclawgatewaytoken":
			env.RemoteToken = firstNonEmpty(env.RemoteToken, value)
		case "remotepassword", "openclawgatewaypassword":
			env.RemotePassword = firstNonEmpty(env.RemotePassword, value)
		case "aigatewayurl", "openaibaseurl":
			env.AIGatewayURL = firstNonEmpty(env.AIGatewayURL, value)
		case "aigatewayapikey", "openaiapikey":
			env.AIGatewayAPIKey = firstNonEmpty(env.AIGatewayAPIKey, value)
		case "openclawagentid":
			env.AgentID = firstNonEmpty(env.AgentID, value)
		case "openclawagentname":
			env.AgentName = firstNonEmpty(env.AgentName, value)
		case "openclawagentworkspace":
			env.AgentWorkspace = firstNonEmpty(env.AgentWorkspace, value)
		case "openclawagentmodel":
			env.AgentModel = firstNonEmpty(env.AgentModel, value)
		case "sshmcpurl", "xcfsshmcpurl", "edgesshmcpurl":
			env.SSHMCPURL = firstNonEmpty(env.SSHMCPURL, value)
		case "sshmcpbearertoken", "xcfsshmcpbearertoken", "edgesshmcpbearertoken":
			env.SSHMCPToken = firstNonEmpty(env.SSHMCPToken, value)
		case "terraformrepo", "xcfterraformrepo":
			env.TerraformRepo = firstNonEmpty(env.TerraformRepo, value)
		case "playbooksrepo", "xcfplaybooksrepo":
			env.PlaybooksRepo = firstNonEmpty(env.PlaybooksRepo, value)
		case "codexhome", "xcfcodexhome":
			env.CodexHome = firstNonEmpty(env.CodexHome, value)
		}
	}
	if err := scanner.Err(); err != nil {
		return GatewayEnv{}, err
	}
	return env, nil
}

func splitKVLine(line string) (string, string, bool) {
	sep := strings.IndexAny(line, "=:")
	if sep <= 0 {
		return "", "", false
	}
	key := cleanToken(line[:sep])
	value := cleanToken(line[sep+1:])
	if key == "" || value == "" {
		return "", "", false
	}
	return key, value, true
}

func cleanToken(s string) string {
	out := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), ","))
	out = strings.Trim(out, `"`)
	return strings.TrimSpace(out)
}

func normalizeKey(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	replacer := strings.NewReplacer("-", "", "_", "", ".", "", "\"", "")
	return replacer.Replace(s)
}

func firstNonEmpty(existing, next string) string {
	if strings.TrimSpace(existing) != "" {
		return existing
	}
	return strings.TrimSpace(next)
}
