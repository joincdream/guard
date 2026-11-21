package e2e_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"

	"guard/internal/hub/handoff"
)

var (
	cliBuildOnce sync.Once
	cliBinPath   string
	cliBuildErr  error
)

func buildGuardBinary(t *testing.T) string {
	t.Helper()
	cliBuildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "guard-cli-e2e-bin-*")
		if err != nil {
			cliBuildErr = err
			return
		}
		bin := filepath.Join(dir, "guard")
		wd, err := os.Getwd()
		if err != nil {
			cliBuildErr = err
			return
		}
		moduleRoot := filepath.Clean(filepath.Join(wd, "..", ".."))
		cmd := exec.Command("go", "build", "-o", bin, "./cmd/guard")
		cmd.Dir = moduleRoot
		out, err := cmd.CombinedOutput()
		if err != nil {
			cliBuildErr = errors.New("go build ./cmd/guard failed: " + err.Error() + "\n" + string(out))
			return
		}
		cliBinPath = bin
	})
	if cliBuildErr != nil {
		t.Fatalf("%v", cliBuildErr)
	}
	return cliBinPath
}

type cliEnv struct {
	bin  string
	home string
	ws   string
}

func newIsolatedCLIEnv(t *testing.T) *cliEnv {
	t.Helper()
	return &cliEnv{
		bin:  buildGuardBinary(t),
		home: t.TempDir(),
		ws:   t.TempDir(),
	}
}

func (e *cliEnv) run(stdin string, extraEnv []string, args ...string) (int, string, string) {
	cmd := exec.Command(e.bin, args...)
	cmd.Dir = e.ws

	// 격리된 최소 환경 구성 (실제 HOME/AGENT_MODE/GUARD_SESSION_ID 오염 방지)
	baseEnv := []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + e.home,
		"WORKSPACE_ROOT=" + e.ws,
	}
	cmd.Env = append(baseEnv, extraEnv...)

	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}
	return exitCode, stdout.String(), stderr.String()
}

var cliIDRegex = regexp.MustCompile(`hnd-\d{8}-[0-9a-f]{8}`)

func extractHandoffID(t *testing.T, output string) string {
	t.Helper()
	id := cliIDRegex.FindString(output)
	if id == "" {
		t.Fatalf("failed to find handoff ID in output: %q", output)
	}
	return id
}

// TC-CLI-01: guard handoff send 플래그 파싱 및 필수값 누락 거부
func TestHandoffCLI01_SendFlagsAndValidation(t *testing.T) {
	env := newIsolatedCLIEnv(t)

	// (1) 정상 send
	code, stdout, stderr := env.run("", []string{"AGENT_MODE=TEST"},
		"handoff", "send",
		"--to", "dev",
		"--type", "fix",
		"--ref", "a.md",
		"--ref", "b.md",
		"--targets", "x.go,y.go",
		"-m", "CLI 테스트",
	)
	if code != 0 {
		t.Fatalf("expected exit 0 for valid send, got %d; stdout=%q stderr=%q", code, stdout, stderr)
	}
	id := extractHandoffID(t, stdout)

	// inbox --json으로 파싱 결과 단언
	code, inboxOut, inboxErr := env.run("", nil, "handoff", "inbox", "--json")
	if code != 0 {
		t.Fatalf("expected exit 0 for inbox --json, got %d; stderr=%q", code, inboxErr)
	}
	var items []handoff.HandoffItem
	if err := json.Unmarshal([]byte(inboxOut), &items); err != nil {
		t.Fatalf("failed to unmarshal inbox JSON %q: %v", inboxOut, err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item in inbox, got %d", len(items))
	}
	got := items[0]
	if got.ID != id {
		t.Errorf("ID mismatch: want %q, got %q", id, got.ID)
	}
	if got.Status != handoff.StatusPending {
		t.Errorf("Status mismatch: want %q, got %q", handoff.StatusPending, got.Status)
	}
	if got.Type != handoff.TypeFix {
		t.Errorf("Type mismatch: want %q, got %q", handoff.TypeFix, got.Type)
	}
	if got.Summary != "CLI 테스트" {
		t.Errorf("Summary mismatch: want %q, got %q", "CLI 테스트", got.Summary)
	}
	if !reflect.DeepEqual(got.References, []string{"a.md", "b.md"}) {
		t.Errorf("References mismatch: want [a.md b.md], got %v", got.References)
	}
	if !reflect.DeepEqual(got.Targets, []string{"x.go", "y.go"}) {
		t.Errorf("Targets mismatch: want [x.go y.go], got %v", got.Targets)
	}

	// (2) 비정상 send 케이스
	invalidCases := []struct {
		name string
		args []string
	}{
		{"missing_summary", []string{"handoff", "send", "--to", "dev"}},
		{"missing_to", []string{"handoff", "send", "-m", "요약"}},
		{"path_traversal_to", []string{"handoff", "send", "--to", "../bad", "-m", "요약"}},
	}
	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			c, out, errOut := env.run("", nil, tc.args...)
			if c != 1 {
				t.Errorf("expected exit code 1, got %d (stdout=%q stderr=%q)", c, out, errOut)
			}
			if strings.TrimSpace(errOut) == "" {
				t.Errorf("expected non-empty stderr error message for %s", tc.name)
			}
		})
	}
}

// TC-CLI-02: guard handoff inbox 필터링 및 --all / --json 계약
func TestHandoffCLI02_InboxFilteringAndJSON(t *testing.T) {
	env := newIsolatedCLIEnv(t)

	sendTicket := func(to, summary string) string {
		t.Helper()
		c, out, errOut := env.run("", []string{"AGENT_MODE=PLAN"}, "handoff", "send", "--to", to, "-m", summary)
		if c != 0 {
			t.Fatalf("send(%s, %s) failed (exit %d): stdout=%q stderr=%q", to, summary, c, out, errOut)
		}
		return extractHandoffID(t, out)
	}

	pendingDevID := sendTicket("dev", "pending-dev")

	approvedTestID := sendTicket("test", "approved-test")
	if c, out, errOut := env.run("", nil, "handoff", "approve", approvedTestID); c != 0 {
		t.Fatalf("approve(%s) failed (exit %d): stdout=%q stderr=%q", approvedTestID, c, out, errOut)
	}

	ackedDevID := sendTicket("dev", "acked-dev")
	if c, out, errOut := env.run("", nil, "handoff", "approve", ackedDevID); c != 0 {
		t.Fatalf("approve(%s) failed (exit %d): stdout=%q stderr=%q", ackedDevID, c, out, errOut)
	}
	if c, out, errOut := env.run("", nil, "handoff", "ack", ackedDevID); c != 0 {
		t.Fatalf("ack(%s) failed (exit %d): stdout=%q stderr=%q", ackedDevID, c, out, errOut)
	}

	rejectedPlanID := sendTicket("plan", "rejected-plan")
	if c, out, errOut := env.run("", nil, "handoff", "reject", rejectedPlanID, "-m", "반려"); c != 0 {
		t.Fatalf("reject(%s) failed (exit %d): stdout=%q stderr=%q", rejectedPlanID, c, out, errOut)
	}

	queryJSON := func(args ...string) []handoff.HandoffItem {
		t.Helper()
		c, out, errOut := env.run("", nil, args...)
		if c != 0 {
			t.Fatalf("%v failed (exit %d): stderr=%q", args, c, errOut)
		}
		var list []handoff.HandoffItem
		if err := json.Unmarshal([]byte(out), &list); err != nil {
			t.Fatalf("%v invalid JSON %q: %v", args, out, err)
		}
		return list
	}

	// (1) guard handoff inbox --json -> 활성 2건만 (pendingDevID, approvedTestID)
	active := queryJSON("handoff", "inbox", "--json")
	if len(active) != 2 {
		t.Fatalf("inbox --json: expected 2 active items, got %d: %+v", len(active), active)
	}
	activeSet := map[string]handoff.HandoffStatus{}
	for _, it := range active {
		activeSet[it.ID] = it.Status
	}
	if activeSet[pendingDevID] != handoff.StatusPending || activeSet[approvedTestID] != handoff.StatusApproved {
		t.Fatalf("inbox --json unexpected items: %+v", activeSet)
	}

	// (2) guard handoff inbox --all --json -> 전체 4건
	all := queryJSON("handoff", "inbox", "--all", "--json")
	if len(all) != 4 {
		t.Fatalf("inbox --all --json: expected 4 items, got %d", len(all))
	}

	// (3) guard handoff inbox --mode dev --json -> dev 대상 활성 1건(pendingDevID)만
	devOnly := queryJSON("handoff", "inbox", "--mode", "dev", "--json")
	if len(devOnly) != 1 || devOnly[0].ID != pendingDevID {
		t.Fatalf("inbox --mode dev --json: expected only [%s], got %+v", pendingDevID, devOnly)
	}
}

// TC-CLI-03: approve / reject / ack CLI 상태 전이 및 오류 종료 코드
func TestHandoffCLI03_StatusTransitionsAndExitCodes(t *testing.T) {
	env := newIsolatedCLIEnv(t)

	// 티켓 A: pending -> approve (0) -> ack (0) -> approve 중복 (1)
	c, outA, errA := env.run("", nil, "handoff", "send", "--to", "dev", "-m", "티켓 A")
	if c != 0 {
		t.Fatalf("send A failed: %d %q", c, errA)
	}
	idA := extractHandoffID(t, outA)

	if c, out, errOut := env.run("", nil, "handoff", "approve", idA); c != 0 {
		t.Fatalf("approve A expected exit 0, got %d (out=%q err=%q)", c, out, errOut)
	}
	if c, out, errOut := env.run("", nil, "handoff", "ack", idA); c != 0 {
		t.Fatalf("ack A expected exit 0, got %d (out=%q err=%q)", c, out, errOut)
	}
	if c, out, errOut := env.run("", nil, "handoff", "approve", idA); c != 1 {
		t.Errorf("duplicate approve A expected exit 1, got %d (out=%q err=%q)", c, out, errOut)
	}

	// 티켓 B: pending -> reject (0) -> ack 금지 전이 (1)
	c, outB, errB := env.run("", nil, "handoff", "send", "--to", "dev", "-m", "티켓 B")
	if c != 0 {
		t.Fatalf("send B failed: %d %q", c, errB)
	}
	idB := extractHandoffID(t, outB)

	if c, out, errOut := env.run("", nil, "handoff", "reject", idB, "-m", "스펙 변경"); c != 0 {
		t.Fatalf("reject B expected exit 0, got %d (out=%q err=%q)", c, out, errOut)
	}
	if c, out, errOut := env.run("", nil, "handoff", "ack", idB); c != 1 {
		t.Errorf("ack on rejected B expected exit 1, got %d (out=%q err=%q)", c, out, errOut)
	}

	// 미존재 ID: approve -> exit 1
	if c, out, errOut := env.run("", nil, "handoff", "approve", "hnd-00000000-00000000"); c != 1 {
		t.Errorf("approve non-existent ID expected exit 1, got %d (out=%q err=%q)", c, out, errOut)
	}

	// 최종 상태 영속화 확인
	c, allOut, allErr := env.run("", nil, "handoff", "inbox", "--all", "--json")
	if c != 0 {
		t.Fatalf("inbox --all --json failed: %d %q", c, allErr)
	}
	var all []handoff.HandoffItem
	if err := json.Unmarshal([]byte(allOut), &all); err != nil {
		t.Fatalf("unmarshal all failed: %v", err)
	}
	byID := map[string]handoff.HandoffItem{}
	for _, it := range all {
		byID[it.ID] = it
	}
	if byID[idA].Status != handoff.StatusAcked {
		t.Errorf("ticket A final status: want %q, got %q", handoff.StatusAcked, byID[idA].Status)
	}
	if byID[idB].Status != handoff.StatusRejected || byID[idB].RejectReason != "스펙 변경" {
		t.Errorf("ticket B final state mismatch: status=%q reason=%q", byID[idB].Status, byID[idB].RejectReason)
	}
}

// TC-CLI-04: guard hook 바이너리 파이프라인 자체 승인 차단
func TestHandoffCLI04_HookBinarySelfApproveBlock(t *testing.T) {
	env := newIsolatedCLIEnv(t)

	const inputJSON = `{"tool_name":"run_command","tool_input":{"command":"guard handoff approve hnd-1"}}`
	code, stdout, stderr := env.run(inputJSON, []string{"AGENT_MODE=ALL"}, "hook")
	if code != 0 {
		t.Fatalf("guard hook expected exit 0, got %d (stdout=%q stderr=%q)", code, stdout, stderr)
	}

	var parsed struct {
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf("failed to parse hook stdout JSON %q: %v", stdout, err)
	}
	if parsed.Decision != "deny" {
		t.Fatalf("expected decision=\"deny\", got %q (stdout=%q stderr=%q)", parsed.Decision, stdout, stderr)
	}
	if strings.TrimSpace(parsed.Reason) == "" {
		t.Errorf("expected non-empty reason in stdout JSON, got %q", stdout)
	}
	if strings.TrimSpace(stderr) == "" {
		t.Errorf("expected non-empty block reason in stderr")
	}
}
