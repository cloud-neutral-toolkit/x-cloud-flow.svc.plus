package automation

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"xcloudflow/internal/defaults"
)

type Gate struct {
	Confirm   string
	ChangeRef string
}

type CommandResult struct {
	Command    []string          `json:"command"`
	Cwd        string            `json:"cwd,omitempty"`
	Env        map[string]string `json:"env,omitempty"`
	Stdout     string            `json:"stdout,omitempty"`
	Stderr     string            `json:"stderr,omitempty"`
	ExitCode   int               `json:"exit_code"`
	DurationMS int64             `json:"duration_ms"`
}

type TerraformOptions struct {
	WorkingDir string         `json:"working_dir,omitempty"`
	Module     string         `json:"module,omitempty"`
	VarsFile   string         `json:"vars_file,omitempty"`
	Vars       map[string]any `json:"vars,omitempty"`
	Workspace  string         `json:"workspace,omitempty"`
	Gate       Gate           `json:"gate,omitempty"`
}

type AnsibleOptions struct {
	Playbook  string         `json:"playbook,omitempty"`
	Inventory string         `json:"inventory,omitempty"`
	Limit     string         `json:"limit,omitempty"`
	ExtraVars map[string]any `json:"extra_vars,omitempty"`
	Gate      Gate           `json:"gate,omitempty"`
}

func RequireApplyGate(gate Gate) error {
	if strings.TrimSpace(gate.Confirm) != "APPLY" {
		return fmt.Errorf("mutating operations require confirm=APPLY")
	}
	if strings.TrimSpace(gate.ChangeRef) == "" {
		return fmt.Errorf("mutating operations require a non-empty change_ref")
	}
	return nil
}

func LooksLikeMutatingCommand(command string) bool {
	cmd := strings.TrimSpace(strings.ToLower(command))
	if cmd == "" {
		return false
	}
	mutatingFragments := []string{
		" >",
		"> ",
		">>",
		"| tee",
		"rm ",
		"mv ",
		"cp ",
		"chmod ",
		"chown ",
		"mkdir ",
		"rmdir ",
		"touch ",
		"sed -i",
		"perl -pi",
		"systemctl restart",
		"systemctl start",
		"systemctl stop",
		"service ",
		"apt ",
		"apt-get ",
		"yum ",
		"dnf ",
		"apk ",
		"pip install",
		"npm install",
		"brew install",
		"kubectl apply",
		"kubectl delete",
		"helm install",
		"helm upgrade",
		"docker run",
		"docker rm",
		"docker stop",
		"useradd ",
		"usermod ",
		"passwd ",
		"reboot",
		"shutdown",
		"terraform apply",
		"terraform destroy",
		"ansible-playbook",
	}
	for _, fragment := range mutatingFragments {
		if strings.Contains(cmd, fragment) {
			return true
		}
	}
	readOnlyPrefixes := []string{
		"cat ",
		"cd ",
		"df ",
		"du ",
		"env",
		"find ",
		"grep ",
		"head ",
		"id",
		"journalctl",
		"ls",
		"netstat",
		"ps",
		"pwd",
		"ss ",
		"stat ",
		"tail ",
		"uname",
		"whoami",
	}
	for _, prefix := range readOnlyPrefixes {
		if strings.HasPrefix(cmd, prefix) {
			return false
		}
	}
	return false
}

func RunCommand(ctx context.Context, name string, args []string, cwd string, extraEnv map[string]string) (CommandResult, error) {
	start := time.Now()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = cwd
	if len(extraEnv) > 0 {
		env := os.Environ()
		keys := make([]string, 0, len(extraEnv))
		for k := range extraEnv {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, key := range keys {
			env = append(env, fmt.Sprintf("%s=%s", key, extraEnv[key]))
		}
		cmd.Env = env
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := CommandResult{
		Command:    append([]string{name}, args...),
		Cwd:        cwd,
		Env:        extraEnv,
		Stdout:     strings.TrimSpace(stdout.String()),
		Stderr:     strings.TrimSpace(stderr.String()),
		ExitCode:   0,
		DurationMS: time.Since(start).Milliseconds(),
	}
	if err == nil {
		return result, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
	} else {
		result.ExitCode = -1
	}
	return result, fmt.Errorf("%s: %w", strings.Join(result.Command, " "), err)
}

func PrepareTerraformWorkingDir(workspaceRoot string, opts TerraformOptions) (map[string]string, error) {
	workspaceRoot = defaults.WorkspaceDir(workspaceRoot)

	sourceDir := strings.TrimSpace(opts.WorkingDir)
	if sourceDir == "" {
		module := strings.TrimSpace(opts.Module)
		if module == "" {
			return nil, fmt.Errorf("missing module or working_dir")
		}
		sourceDir = filepath.Join(defaults.TerraformRepo(), "example", "terraform", filepath.FromSlash(module))
	}
	if !filepath.IsAbs(sourceDir) {
		sourceDir = filepath.Join(workspaceRoot, sourceDir)
	}
	sourceDir = filepath.Clean(sourceDir)

	info, err := os.Stat(sourceDir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("terraform source is not a directory: %s", sourceDir)
	}

	stageDir := filepath.Join(workspaceRoot, ".xcloudflow", "terraform", fmt.Sprintf("%s-%x", sanitizePath(sourceDir), shortHash(sourceDir)))
	if err := os.RemoveAll(stageDir); err != nil {
		return nil, err
	}
	if err := copyDir(sourceDir, stageDir); err != nil {
		return nil, err
	}
	return map[string]string{
		"source_dir":  sourceDir,
		"working_dir": stageDir,
	}, nil
}

func RunTerraform(ctx context.Context, action string, opts TerraformOptions) (map[string]any, error) {
	action = strings.TrimSpace(strings.ToLower(action))
	if opts.WorkingDir == "" {
		return nil, fmt.Errorf("missing working_dir")
	}
	if action == "apply" || action == "destroy" {
		if err := RequireApplyGate(opts.Gate); err != nil {
			return nil, err
		}
	}

	out := map[string]any{
		"action":      action,
		"working_dir": opts.WorkingDir,
	}

	if action != "init" && strings.TrimSpace(opts.Workspace) != "" {
		workspaceRes, err := ensureTerraformWorkspace(ctx, opts.WorkingDir, opts.Workspace)
		out["workspace"] = workspaceRes
		if err != nil {
			return out, err
		}
	}

	args := []string{action, "-input=false", "-no-color"}
	switch action {
	case "apply", "destroy":
		args = append(args, "-auto-approve")
	}
	if vf := strings.TrimSpace(opts.VarsFile); vf != "" {
		args = append(args, "-var-file", vf)
	}
	for _, arg := range terraformVarArgs(opts.Vars) {
		args = append(args, arg)
	}

	res, err := RunCommand(ctx, "terraform", args, opts.WorkingDir, nil)
	out["result"] = res
	return out, err
}

func RunAnsiblePlaybook(ctx context.Context, check bool, opts AnsibleOptions) (map[string]any, error) {
	if strings.TrimSpace(opts.Playbook) == "" {
		return nil, fmt.Errorf("missing playbook")
	}
	if !check {
		if err := RequireApplyGate(opts.Gate); err != nil {
			return nil, err
		}
	}

	playbookPath := resolvePlaybookPath(opts.Playbook)
	inventoryPath := resolveInventoryPath(opts.Inventory)
	repoRoot := defaults.PlaybooksRepo()
	playbookDir := filepath.Dir(playbookPath)
	out := map[string]any{
		"playbook":  playbookPath,
		"inventory": inventoryPath,
		"mode":      "apply",
	}
	if check {
		out["mode"] = "check"
	}

	env := map[string]string{
		"ANSIBLE_CONFIG":              filepath.Join(repoRoot, "ansible.cfg"),
		"ANSIBLE_RETRY_FILES_ENABLED": "False",
	}
	syntaxArgs := []string{"--syntax-check", "-i", inventoryPath, playbookPath}
	if limit := strings.TrimSpace(opts.Limit); limit != "" {
		syntaxArgs = append(syntaxArgs, "--limit", limit)
	}
	if extra := encodeExtraVars(opts.ExtraVars); extra != "" {
		syntaxArgs = append(syntaxArgs, "--extra-vars", extra)
	}
	syntaxRes, syntaxErr := RunCommand(ctx, "ansible-playbook", syntaxArgs, playbookDir, env)
	out["syntax"] = syntaxRes
	if syntaxErr != nil {
		return out, syntaxErr
	}

	args := []string{"-i", inventoryPath, playbookPath}
	if check {
		args = append(args, "--check")
	}
	if limit := strings.TrimSpace(opts.Limit); limit != "" {
		args = append(args, "--limit", limit)
	}
	if extra := encodeExtraVars(opts.ExtraVars); extra != "" {
		args = append(args, "--extra-vars", extra)
	}
	runRes, runErr := RunCommand(ctx, "ansible-playbook", args, playbookDir, env)
	out["result"] = runRes
	return out, runErr
}

func RunAnsibleAdhoc(ctx context.Context, check bool, inventory string, target string, module string, moduleArgs string, gate Gate) (map[string]any, error) {
	if strings.TrimSpace(target) == "" {
		return nil, fmt.Errorf("missing target")
	}
	if strings.TrimSpace(module) == "" {
		return nil, fmt.Errorf("missing module")
	}
	if !check {
		if err := RequireApplyGate(gate); err != nil {
			return nil, err
		}
	}

	inventoryPath := resolveInventoryPath(inventory)
	args := []string{"-i", inventoryPath, target, "-m", module}
	if strings.TrimSpace(moduleArgs) != "" {
		args = append(args, "-a", moduleArgs)
	}
	if check {
		args = append(args, "--check")
	}

	env := map[string]string{
		"ANSIBLE_CONFIG":              filepath.Join(defaults.PlaybooksRepo(), "ansible.cfg"),
		"ANSIBLE_RETRY_FILES_ENABLED": "False",
	}
	res, err := RunCommand(ctx, "ansible", args, defaults.PlaybooksRepo(), env)
	return map[string]any{
		"target":    target,
		"inventory": inventoryPath,
		"result":    res,
	}, err
}

func ensureTerraformWorkspace(ctx context.Context, workingDir string, workspace string) (map[string]any, error) {
	selectRes, selectErr := RunCommand(ctx, "terraform", []string{"workspace", "select", workspace}, workingDir, nil)
	out := map[string]any{
		"name":   workspace,
		"select": selectRes,
	}
	if selectErr == nil {
		return out, nil
	}
	newRes, newErr := RunCommand(ctx, "terraform", []string{"workspace", "new", workspace}, workingDir, nil)
	out["create"] = newRes
	if newErr != nil {
		return out, newErr
	}
	return out, nil
}

func terraformVarArgs(vars map[string]any) []string {
	if len(vars) == 0 {
		return nil
	}
	keys := make([]string, 0, len(vars))
	for key := range vars {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	args := make([]string, 0, len(keys))
	for _, key := range keys {
		args = append(args, fmt.Sprintf("-var=%s=%v", key, vars[key]))
	}
	return args
}

func encodeExtraVars(vars map[string]any) string {
	if len(vars) == 0 {
		return ""
	}
	keys := make([]string, 0, len(vars))
	for key := range vars {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", key, vars[key]))
	}
	return strings.Join(parts, " ")
}

func resolvePlaybookPath(playbook string) string {
	if filepath.IsAbs(playbook) {
		return playbook
	}
	return filepath.Join(defaults.PlaybooksRepo(), filepath.FromSlash(playbook))
}

func resolveInventoryPath(inventory string) string {
	if strings.TrimSpace(inventory) == "" {
		return filepath.Join(defaults.PlaybooksRepo(), "inventory.ini")
	}
	if filepath.IsAbs(inventory) {
		return inventory
	}
	return filepath.Join(defaults.PlaybooksRepo(), filepath.FromSlash(inventory))
}

func sanitizePath(path string) string {
	replacer := strings.NewReplacer(string(filepath.Separator), "-", ".", "-", ":", "-", " ", "-")
	path = replacer.Replace(strings.TrimSpace(path))
	path = strings.Trim(path, "-")
	if path == "" {
		return "terraform"
	}
	return path
}

func shortHash(text string) uint32 {
	h := fnv.New32a()
	_, _ = io.WriteString(h, text)
	return h.Sum32()
}

func copyDir(srcDir string, dstDir string) error {
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dstDir, 0o755)
		}
		if info.IsDir() {
			if info.Name() == ".git" || info.Name() == ".terraform" {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(dstDir, rel), info.Mode())
		}
		if info.Name() == ".terraform.lock.hcl" {
			// Keep the lock file if present, but copy it like a normal file.
		}
		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		dstPath := filepath.Join(dstDir, rel)
		if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
			return err
		}
		dstFile, err := os.OpenFile(dstPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
		if err != nil {
			return err
		}
		defer dstFile.Close()
		_, err = io.Copy(dstFile, srcFile)
		return err
	})
}
