package main_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"guard/internal/hub/handoff"
)

var (
	guardBinOnce sync.Once
	guardBinPath string
	guardBinErr  error
)

func buildGuardCLI(t *testing.T) string {
	t.Helper()
	guardBinOnce.Do(func() {
		dir, err := os.MkdirTemp("", "guard-cli-custom-mode-*")
		if err != nil {
			guardBinErr = err
			return
		}
		bin := filepath.Join(dir, "guard")
		cmd := exec.Command("go", "build", "-o", bin, ".")
		out, err := cmd.CombinedOutput()
		if err != nil {
			guardBinErr = errors.New("go build . failed: " + err.Error() + "\n" + string(out))
			return
		}
		guardBinPath = bin
	})
	if guardBinErr != nil {
		t.Fatalf("%v", guardBinErr)
	}
	return guardBinPath
}

type isolatedCLI struct {
	bin  string
	home string
	ws   string
}

func newIsolatedCLI(t *testing.T) *isolatedCLI {
	t.Helper()
	ws := t.TempDir()
	policyYAML := `version: "1.0"
rules:
  - name: "review-deny-write"
    mode: "REVIEW"
    type: "write"
    patterns:
      - "internal/**"
    message: "REVIEW: write denied"
`
	if err := os.WriteFile(filepath.Join(ws, "guard-policy.yaml"), []byte(policyYAML), 0o644); err != nil {
		t.Fatalf("write guard-policy.yaml: %v", err)
	}
	return &isolatedCLI{
		bin:  buildGuardCLI(t),
		home: t.TempDir(),
		ws:   ws,
	}
}

func (c *isolatedCLI) run(args ...string) (int, string, string) {
	cmd := exec.Command(c.bin, args...)
	cmd.Dir = c.ws
	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + c.home,
		"WORKSPACE_ROOT=" + c.ws,
		"GUARD_LANG=en",
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
		} else {
			code = -1
		}
	}
	return code, stdout.String(), stderr.String()
}

func TestContract01_CLI_CustomModeSwitch(t *testing.T) {
	cli := newIsolatedCLI(t)

	code, stdout, stderr := cli.run("review")
	if code != 0 {
		t.Fatalf("guard review exit=%d, want 0\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}

	modeBytes, err := os.ReadFile(filepath.Join(cli.ws, ".agents", "current_mode"))
	if err != nil {
		t.Fatalf("read .agents/current_mode: %v", err)
	}
	if got := strings.TrimSpace(string(modeBytes)); got != "REVIEW" {
		t.Fatalf("current_mode = %q, want %q", got, "REVIEW")
	}
}

func TestContract01_CLI_CustomModeLauncherRun(t *testing.T) {
	cli := newIsolatedCLI(t)

	code, stdout, stderr := cli.run("review", "sh", "-c", "printf '%s' \"$AGENT_MODE\"")
	if code != 0 {
		t.Fatalf("guard review sh ... exit=%d, want 0\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "REVIEW") {
		t.Fatalf("expected AGENT_MODE=REVIEW in stdout, got %q (stderr: %q)", stdout, stderr)
	}
}

func TestContract01_CLI_CustomModeHandoffSend(t *testing.T) {
	cli := newIsolatedCLI(t)

	code, stdout, stderr := cli.run("handoff", "send", "--to", "review", "-m", "Please review changes")
	if code != 0 {
		t.Fatalf("guard handoff send --to review exit=%d, want 0\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}

	code, inboxOut, inboxErr := cli.run("handoff", "inbox", "--all", "--json")
	if code != 0 {
		t.Fatalf("guard handoff inbox --all --json exit=%d, want 0\nstderr:\n%s", code, inboxErr)
	}

	var items []handoff.HandoffItem
	if err := json.Unmarshal([]byte(inboxOut), &items); err != nil {
		t.Fatalf("unmarshal inbox json: %v\nraw:\n%s", err, inboxOut)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 handoff item, got %d", len(items))
	}
	if items[0].To.Mode != "review" {
		t.Errorf("items[0].To.Mode = %q, want %q", items[0].To.Mode, "review")
	}
	if items[0].Summary != "Please review changes" {
		t.Errorf("items[0].Summary = %q, want %q", items[0].Summary, "Please review changes")
	}
}

func TestContract01_CLI_UnknownCommandReturnsExit1(t *testing.T) {
	cli := newIsolatedCLI(t)

	code, _, _ := cli.run("unknowncmd_not_in_policy_or_path")
	if code != 1 {
		t.Fatalf("guard unknowncmd exit=%d, want 1", code)
	}

	code2, _, _ := cli.run("unknowncmd_not_in_policy_or_path", "arg1", "arg2")
	if code2 != 1 {
		t.Fatalf("guard unknowncmd with args exit=%d, want 1", code2)
	}
}

func TestContract01_CLI_EdgeCases_NoPolicyAndExitCodePropagation(t *testing.T) {
	t.Run("custom mode without policy fails with exit 1", func(t *testing.T) {
		cli := &isolatedCLI{
			bin:  buildGuardCLI(t),
			home: t.TempDir(),
			ws:   t.TempDir(), // no guard-policy.yaml
		}
		code, _, _ := cli.run("review")
		if code != 1 {
			t.Fatalf("guard review without policy exit=%d, want 1", code)
		}
	})

	t.Run("handoff send --to review without policy parses as SessionID", func(t *testing.T) {
		cli := &isolatedCLI{
			bin:  buildGuardCLI(t),
			home: t.TempDir(),
			ws:   t.TempDir(), // no guard-policy.yaml
		}
		code, _, stderr := cli.run("handoff", "send", "--to", "review", "-m", "To session review")
		if code != 0 {
			t.Fatalf("handoff send --to review exit=%d, want 0\nstderr:\n%s", code, stderr)
		}
		code, out, _ := cli.run("handoff", "inbox", "--all", "--json")
		if code != 0 {
			t.Fatalf("handoff inbox exit=%d, want 0", code)
		}
		var items []handoff.HandoffItem
		if err := json.Unmarshal([]byte(out), &items); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(items) != 1 || items[0].To.SessionID != "review" || items[0].To.Mode != "" {
			t.Fatalf("expected To.SessionID='review' Mode='', got %+v", items)
		}
	})

	t.Run("custom mode launcher propagates non-zero exit code", func(t *testing.T) {
		cli := newIsolatedCLI(t)
		code, _, _ := cli.run("review", "sh", "-c", "exit 42")
		if code != 42 {
			t.Fatalf("guard review sh -c 'exit 42' exit=%d, want 42", code)
		}
	})
}
