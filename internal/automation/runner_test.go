package automation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRequireApplyGate(t *testing.T) {
	if err := RequireApplyGate(Gate{Confirm: "APPLY", ChangeRef: "chg-1"}); err != nil {
		t.Fatalf("expected gate to pass: %v", err)
	}
	if err := RequireApplyGate(Gate{Confirm: "PLAN", ChangeRef: "chg-1"}); err == nil {
		t.Fatal("expected confirm gate failure")
	}
	if err := RequireApplyGate(Gate{Confirm: "APPLY"}); err == nil {
		t.Fatal("expected change_ref gate failure")
	}
}

func TestLooksLikeMutatingCommand(t *testing.T) {
	if LooksLikeMutatingCommand("ls -la") {
		t.Fatal("ls should be treated as read-only")
	}
	if !LooksLikeMutatingCommand("systemctl restart nginx") {
		t.Fatal("systemctl restart should require approval")
	}
	if !LooksLikeMutatingCommand("echo test > /tmp/file") {
		t.Fatal("redirection should require approval")
	}
}

func TestPrepareTerraformWorkingDirStagesExternalRepo(t *testing.T) {
	workspace := t.TempDir()
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "main.tf"), []byte("terraform {}"), 0o644); err != nil {
		t.Fatalf("write tf file: %v", err)
	}
	if err := os.Mkdir(filepath.Join(source, ".terraform"), 0o755); err != nil {
		t.Fatalf("mkdir .terraform: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, ".terraform", "ignored.txt"), []byte("ignore"), 0o644); err != nil {
		t.Fatalf("write ignored file: %v", err)
	}

	got, err := PrepareTerraformWorkingDir(workspace, TerraformOptions{WorkingDir: source})
	if err != nil {
		t.Fatalf("prepare terraform dir: %v", err)
	}
	if got["source_dir"] != source {
		t.Fatalf("unexpected source dir: %s", got["source_dir"])
	}
	if !strings.HasPrefix(got["working_dir"], filepath.Join(workspace, ".xcloudflow", "terraform")) {
		t.Fatalf("unexpected staged dir: %s", got["working_dir"])
	}
	if _, err := os.Stat(filepath.Join(got["working_dir"], "main.tf")); err != nil {
		t.Fatalf("missing staged main.tf: %v", err)
	}
	if _, err := os.Stat(filepath.Join(got["working_dir"], ".terraform")); !os.IsNotExist(err) {
		t.Fatalf("expected .terraform to be skipped, got err=%v", err)
	}
}
