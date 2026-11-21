package e2e_test

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"guard/internal/adapters/hook"
	"guard/internal/core"
	"guard/internal/hub/handoff"
	"guard/internal/hub/session"
)

// TC-GATE-01: 모드 및 정책 무관 자체 승인 원천 차단
func TestHandoffGate01_SelfApproveBlockAllModesAndPolicies(t *testing.T) {
	modes := []string{
		core.ModePlan,
		core.ModeDev,
		core.ModeTest,
		core.ModeAll,
	}
	policies := []struct {
		name   string
		policy *core.Policy
	}{
		{name: "nil_policy", policy: nil},
		{name: "allow_all_policy", policy: &core.Policy{Version: "1", Rules: []core.Rule{}}},
	}

	const cmd = "guard handoff approve hnd-20261121-12345678"

	for _, mode := range modes {
		for _, pol := range policies {
			t.Run(mode+"/"+pol.name, func(t *testing.T) {
				dec := core.EvaluateRequest(pol.policy, core.EvalRequest{
					Mode:        mode,
					ToolName:    "run_command",
					CommandLine: cmd,
				})
				if dec.Allowed {
					t.Fatalf("expected self-approve to be denied for mode=%s policy=%s, got Allowed=true", mode, pol.name)
				}
				if dec.RuleName != "handoff-self-approve-block" {
					t.Fatalf("expected RuleName=%q, got %q", "handoff-self-approve-block", dec.RuleName)
				}
				if strings.TrimSpace(dec.Reason) == "" {
					t.Fatalf("expected non-empty Reason for self-approve block")
				}
			})
		}
	}
}

// TC-GATE-02: 셸 체이닝 · 서브셸 · 인라인 스크립트 · 경로 변형 우회 공격 (Adversarial)
func TestHandoffGate02_BypassAttacks(t *testing.T) {
	t.Setenv("AGENT_MODE", core.ModeAll)
	t.Setenv("AGENT_ROLE", "")
	t.Setenv("GUARD_SESSION_ID", "")
	t.Setenv("SESSION_ID", "")
	t.Setenv("CONVERSATION_ID", "")
	t.Setenv("GUARD_DEBUG", "")

	attacks := []struct {
		name string
		cmd  string
	}{
		{"and_chain", `echo ok && guard handoff approve hnd-1`},
		{"semicolon_chain", `true; guard handoff approve hnd-1`},
		{"or_chain", `false || guard handoff approve hnd-1`},
		{"bash_c_double_quote", `bash -c "guard handoff approve hnd-1"`},
		{"sh_c_single_quote", `sh -c 'guard handoff approve hnd-1'`},
		{"python3_subprocess", `python3 -c 'import subprocess; subprocess.run(["guard", "handoff", "approve", "hnd-1"])'`},
		{"abs_path_binary", `/usr/local/bin/guard handoff approve hnd-1`},
		{"rel_path_binary", `./guard handoff approve hnd-1`},
		{"env_prefix", `AGENT_MODE=ALL guard handoff approve hnd-1`},
	}

	ws := t.TempDir()
	sessMgr := session.NewManager(t.TempDir())

	for _, tc := range attacks {
		t.Run(tc.name, func(t *testing.T) {
			// 1. Core EvaluateRequest 직접 검증
			dec := core.EvaluateRequest(nil, core.EvalRequest{
				Mode:        core.ModeAll,
				ToolName:    "run_command",
				CommandLine: tc.cmd,
			})
			if dec.Allowed {
				t.Errorf("[core] bypass succeeded for %q: Allowed=true", tc.cmd)
			}
			if dec.RuleName != "handoff-self-approve-block" {
				t.Errorf("[core] expected RuleName=%q, got %q (cmd=%q)", "handoff-self-approve-block", dec.RuleName, tc.cmd)
			}
			if strings.TrimSpace(dec.Reason) == "" {
				t.Errorf("[core] expected non-empty Reason (cmd=%q)", tc.cmd)
			}

			// 2. Hook Handler.Handle 검증
			payload, err := json.Marshal(map[string]any{
				"tool_name":    "run_command",
				"command_line": tc.cmd,
				"cwd":          ws,
			})
			if err != nil {
				t.Fatalf("marshal hook payload failed: %v", err)
			}

			var stdout, stderr bytes.Buffer
			handler := hook.NewHandler(sessMgr, ws, &stdout, &stderr)
			if err := handler.Handle(bytes.NewReader(payload)); err != nil {
				t.Fatalf("[hook] Handle returned error: %v", err)
			}

			var out hook.Output
			if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
				t.Fatalf("[hook] failed to parse stdout JSON %q: %v", stdout.String(), err)
			}
			if out.Decision != "deny" {
				t.Errorf("[hook] expected decision=\"deny\" for %q, got %q (stdout=%s)", tc.cmd, out.Decision, stdout.String())
			}
			if strings.TrimSpace(out.Reason) == "" {
				t.Errorf("[hook] expected non-empty output reason for %q", tc.cmd)
			}
			stderrLines := strings.Split(strings.TrimSpace(stderr.String()), "\n")
			if strings.TrimSpace(stderr.String()) == "" || len(stderrLines) != 1 {
				t.Errorf("[hook] expected exactly 1 non-empty stderr line, got %q", stderr.String())
			}
		})
	}
}

// TC-GATE-03: 정상 핸드오프 명령 오탐(False Positive) 방지
func TestHandoffGate03_NoFalsePositive(t *testing.T) {
	allowedCmds := []string{
		`guard handoff send --to test --type verify -m "검증 요청"`,
		`guard handoff inbox --json`,
		`guard handoff ack hnd-1`,
		`guard handoff reject hnd-1 -m "반려"`,
	}

	for _, cmd := range allowedCmds {
		t.Run(cmd, func(t *testing.T) {
			dec := core.EvaluateRequest(nil, core.EvalRequest{
				Mode:        core.ModeDev,
				ToolName:    "run_command",
				CommandLine: cmd,
			})
			if dec.RuleName == "handoff-self-approve-block" {
				t.Fatalf("false positive self-approve block on %q: reason=%q", cmd, dec.Reason)
			}
			if !dec.Allowed {
				t.Fatalf("expected Allowed=true for %q, got Allowed=false (rule=%q, reason=%q)", cmd, dec.RuleName, dec.Reason)
			}
		})
	}
}

// TC-GATE-04: 승인 전 인박스 물리 격리
func TestHandoffGate04_InboxIsolationBeforeApprove(t *testing.T) {
	mgr, _, sessionDir := newIsolatedHandoffManager(t)
	sessMgr := session.NewManager(sessionDir)
	ws := t.TempDir()

	const sessID = "sess-dev-1"
	if err := sessMgr.Create(&session.Session{
		ID:        sessID,
		Mode:      core.ModeDev,
		PID:       os.Getpid(),
		Workspace: ws,
	}); err != nil {
		t.Fatalf("Create session failed: %v", err)
	}

	if _, err := mgr.Enqueue(handoff.HandoffItem{
		Type:    handoff.TypeFix,
		From:    handoff.HandoffNode{Mode: handoff.ModeTest},
		To:      handoff.HandoffNode{Mode: handoff.ModeDev},
		Summary: "미승인 티켓 격리 검증",
		Targets: []string{"internal/hub/handoff/queue.go"},
	}); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	inboxPath, err := mgr.InboxPath(sessID)
	if err != nil {
		t.Fatalf("InboxPath failed: %v", err)
	}
	if _, err := os.Stat(inboxPath); !os.IsNotExist(err) {
		t.Fatalf("expected inbox file %q not to exist before approval, stat err=%v", inboxPath, err)
	}

	sess, err := sessMgr.Get(sessID)
	if err != nil {
		t.Fatalf("sessMgr.Get failed: %v", err)
	}
	if len(sess.AllowedWritePaths) != 0 {
		t.Fatalf("expected AllowedWritePaths to remain empty before approval, got %v", sess.AllowedWritePaths)
	}
}
