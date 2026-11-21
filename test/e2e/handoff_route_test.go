package e2e_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"guard/internal/adapters/launcher"
	"guard/internal/hub/handoff"
	"guard/internal/hub/session"
)

// TC-ROUTE-01: 활성 세션 즉시 전달 및 Envelope 마커 검증
func TestHandoffRoute01_ActiveSessionImmediateDelivery(t *testing.T) {
	handoffMgr, _, sessionDir := newIsolatedHandoffManager(t)
	sessMgr := session.NewManager(sessionDir)
	ws := t.TempDir()

	const sessID = "sess-dev-live"
	if err := sessMgr.Create(&session.Session{
		ID:        sessID,
		Mode:      "DEV",
		PID:       os.Getpid(),
		Workspace: ws,
	}); err != nil {
		t.Fatalf("Create active DEV session failed: %v", err)
	}

	const summary = "활성 세션 즉시 전달 테스트"
	item, err := handoffMgr.Enqueue(handoff.HandoffItem{
		Type:    handoff.TypeFix,
		From:    handoff.HandoffNode{Mode: handoff.ModeTest},
		To:      handoff.HandoffNode{Mode: handoff.ModeDev},
		Summary: summary,
	})
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	res, err := handoff.ApproveAndRoute(handoffMgr, sessMgr, ws, item.ID)
	if err != nil {
		t.Fatalf("ApproveAndRoute failed: %v", err)
	}
	if res.Buffered {
		t.Fatalf("expected Buffered=false for active session, got true")
	}
	if res.SessionID != sessID {
		t.Fatalf("expected SessionID=%q, got %q", sessID, res.SessionID)
	}

	wantInboxPath, err := handoffMgr.InboxPath(sessID)
	if err != nil {
		t.Fatalf("InboxPath failed: %v", err)
	}
	if res.InboxPath != wantInboxPath {
		t.Fatalf("expected InboxPath=%q, got %q", wantInboxPath, res.InboxPath)
	}

	raw, err := os.ReadFile(res.InboxPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) failed: %v", res.InboxPath, err)
	}
	content := string(raw)
	for _, marker := range []string{
		"<!-- GUARD_HANDOFF_ENVELOPE: START",
		"<!-- GUARD_HANDOFF_ENVELOPE: END -->",
		item.ID,
		summary,
	} {
		if !strings.Contains(content, marker) {
			t.Errorf("inbox envelope missing %q\n--- content ---\n%s", marker, content)
		}
	}

	got, err := handoffMgr.Get(item.ID)
	if err != nil {
		t.Fatalf("Get after ApproveAndRoute failed: %v", err)
	}
	if got.Status != handoff.StatusApproved {
		t.Fatalf("expected Status=%q, got %q", handoff.StatusApproved, got.Status)
	}
	if got.To.SessionID != sessID {
		t.Fatalf("expected To.SessionID=%q, got %q", sessID, got.To.SessionID)
	}
}

// TC-ROUTE-02: 오프라인 모드 버퍼링 (사망 PID 세션 배제 포함)
func TestHandoffRoute02_OfflineBufferingExcludesDeadPID(t *testing.T) {
	handoffMgr, _, sessionDir := newIsolatedHandoffManager(t)
	sessMgr := session.NewManager(sessionDir)
	ws := t.TempDir()

	const deadPID = 999999
	if session.IsProcessAlive(deadPID) {
		t.Skipf("PID %d unexpectedly alive in test environment", deadPID)
	}

	const staleSessID = "sess-dev-stale"
	if err := sessMgr.Create(&session.Session{
		ID:        staleSessID,
		Mode:      "DEV",
		PID:       deadPID,
		Workspace: ws,
	}); err != nil {
		t.Fatalf("Create stale session failed: %v", err)
	}

	item, err := handoffMgr.Enqueue(handoff.HandoffItem{
		Type:    handoff.TypeTask,
		From:    handoff.HandoffNode{Mode: handoff.ModePlan},
		To:      handoff.HandoffNode{Mode: handoff.ModeDev},
		Summary: "오프라인 버퍼링 테스트",
	})
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	res, err := handoff.ApproveAndRoute(handoffMgr, sessMgr, ws, item.ID)
	if err != nil {
		t.Fatalf("ApproveAndRoute offline mode failed: %v", err)
	}
	if !res.Buffered {
		t.Fatalf("expected Buffered=true when only stale PID session exists, got false (SessionID=%q)", res.SessionID)
	}
	if res.SessionID != "" {
		t.Fatalf("expected empty SessionID for buffered item, got %q", res.SessionID)
	}
	if res.InboxPath != "" {
		t.Fatalf("expected empty InboxPath for buffered item, got %q", res.InboxPath)
	}

	got, err := handoffMgr.Get(item.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Status != handoff.StatusApproved {
		t.Fatalf("expected Status=%q, got %q", handoff.StatusApproved, got.Status)
	}
	if got.To.SessionID != "" {
		t.Fatalf("expected empty To.SessionID for buffered item, got %q", got.To.SessionID)
	}

	staleInbox, err := handoffMgr.InboxPath(staleSessID)
	if err != nil {
		t.Fatalf("InboxPath failed: %v", err)
	}
	if _, err := os.Stat(staleInbox); !os.IsNotExist(err) {
		t.Fatalf("stale session inbox %q must not be created, stat err=%v", staleInbox, err)
	}
}

// TC-ROUTE-03: 미존재 특정 SessionID 대상 승인 실패 시 원자적 롤백 (Adversarial)
func TestHandoffRoute03_MissingTargetSessionRollback(t *testing.T) {
	handoffMgr, _, sessionDir := newIsolatedHandoffManager(t)
	sessMgr := session.NewManager(sessionDir)
	ws := t.TempDir()

	item, err := handoffMgr.Enqueue(handoff.HandoffItem{
		Type:    handoff.TypeFix,
		From:    handoff.HandoffNode{Mode: handoff.ModeTest},
		To:      handoff.HandoffNode{SessionID: "sess-ghost-999"},
		Summary: "유령 세션 대상 승인 롤백 테스트",
	})
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	_, err = handoff.ApproveAndRoute(handoffMgr, sessMgr, ws, item.ID)
	if err == nil {
		t.Fatalf("expected ApproveAndRoute to fail for non-existent SessionID, got nil error")
	}

	got, err := handoffMgr.Get(item.ID)
	if err != nil {
		t.Fatalf("Get after failed ApproveAndRoute failed: %v", err)
	}
	if got.Status != handoff.StatusPending {
		t.Fatalf("expected item to remain %q after failed ApproveAndRoute, got %q (partial transition defect)", handoff.StatusPending, got.Status)
	}
}

// TC-ROUTE-04: 다중 세션 동시 ConsumePendingForMode 중복 소비 방지 (Race Attack)
func TestHandoffRoute04_ConcurrentConsumeNoDuplicate(t *testing.T) {
	handoffMgr, _, sessionDir := newIsolatedHandoffManager(t)
	sessMgr := session.NewManager(sessionDir)
	ws := t.TempDir()

	item, err := handoffMgr.Enqueue(handoff.HandoffItem{
		Type:    handoff.TypeTask,
		From:    handoff.HandoffNode{Mode: handoff.ModePlan},
		To:      handoff.HandoffNode{Mode: handoff.ModeDev},
		Summary: "동시 소비 중복 방지 테스트",
	})
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	res, err := handoff.ApproveAndRoute(handoffMgr, sessMgr, ws, item.ID)
	if err != nil {
		t.Fatalf("ApproveAndRoute failed: %v", err)
	}
	if !res.Buffered {
		t.Fatalf("expected buffered approval, got Buffered=false")
	}

	const consumers = 5
	sessionIDs := make([]string, consumers)
	for i := 0; i < consumers; i++ {
		sessionIDs[i] = fmt.Sprintf("sess-%d", i+1)
	}

	type consumeOutcome struct {
		sessID string
		items  []handoff.HandoffItem
		err    error
	}

	start := make(chan struct{})
	outCh := make(chan consumeOutcome, consumers)
	var wg sync.WaitGroup
	for _, sid := range sessionIDs {
		wg.Add(1)
		go func(sessionID string) {
			defer wg.Done()
			<-start
			items, err := handoffMgr.ConsumePendingForMode(handoff.ModeDev, sessionID)
			outCh <- consumeOutcome{sessID: sessionID, items: items, err: err}
		}(sid)
	}
	close(start)
	wg.Wait()
	close(outCh)

	totalConsumed := 0
	var winnerSessID string
	for out := range outCh {
		if out.err != nil {
			t.Fatalf("ConsumePendingForMode(%s) returned error: %v", out.sessID, out.err)
		}
		if len(out.items) > 0 {
			totalConsumed += len(out.items)
			winnerSessID = out.sessID
		}
	}

	if totalConsumed != 1 {
		t.Fatalf("expected total consumed tickets == 1 across %d concurrent sessions, got %d", consumers, totalConsumed)
	}
	if winnerSessID == "" {
		t.Fatalf("expected a winner session ID")
	}

	existingInboxCount := 0
	for _, sid := range sessionIDs {
		inboxPath, err := handoffMgr.InboxPath(sid)
		if err != nil {
			t.Fatalf("InboxPath(%s) failed: %v", sid, err)
		}
		_, statErr := os.Stat(inboxPath)
		if sid == winnerSessID {
			if statErr != nil {
				t.Errorf("winner session %q inbox %q should exist, stat err=%v", sid, inboxPath, statErr)
			} else {
				existingInboxCount++
			}
		} else {
			if !os.IsNotExist(statErr) {
				t.Errorf("non-winner session %q inbox %q must not exist, stat err=%v", sid, inboxPath, statErr)
				existingInboxCount++
			}
		}
	}
	if existingInboxCount != 1 {
		t.Fatalf("expected exactly 1 inbox file created, got %d", existingInboxCount)
	}

	got, err := handoffMgr.Get(item.ID)
	if err != nil {
		t.Fatalf("Get after concurrent consume failed: %v", err)
	}
	if got.To.SessionID != winnerSessID {
		t.Fatalf("queue item To.SessionID mismatch: want %q, got %q", winnerSessID, got.To.SessionID)
	}
}

// TC-ROUTE-05: 기전달 티켓(To.SessionID != "") 재소비 차단
func TestHandoffRoute05_AlreadyDeliveredNotReconsumed(t *testing.T) {
	handoffMgr, _, sessionDir := newIsolatedHandoffManager(t)
	sessMgr := session.NewManager(sessionDir)
	ws := t.TempDir()

	const liveSessID = "sess-dev-live"
	if err := sessMgr.Create(&session.Session{
		ID:        liveSessID,
		Mode:      "DEV",
		PID:       os.Getpid(),
		Workspace: ws,
	}); err != nil {
		t.Fatalf("Create live session failed: %v", err)
	}

	item, err := handoffMgr.Enqueue(handoff.HandoffItem{
		Type:    handoff.TypeFix,
		From:    handoff.HandoffNode{Mode: handoff.ModeTest},
		To:      handoff.HandoffNode{Mode: handoff.ModeDev},
		Summary: "기전달 티켓 재소비 차단 테스트",
	})
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	res, err := handoff.ApproveAndRoute(handoffMgr, sessMgr, ws, item.ID)
	if err != nil {
		t.Fatalf("ApproveAndRoute failed: %v", err)
	}
	if res.Buffered || res.SessionID != liveSessID {
		t.Fatalf("expected immediate delivery to %q, got buffered=%v sess=%q", liveSessID, res.Buffered, res.SessionID)
	}

	const newSessID = "sess-dev-new"
	consumed, err := handoffMgr.ConsumePendingForMode(handoff.ModeDev, newSessID)
	if err != nil {
		t.Fatalf("ConsumePendingForMode(%s) failed: %v", newSessID, err)
	}
	if len(consumed) != 0 {
		t.Fatalf("expected 0 consumed tickets for already-delivered item, got %d: %+v", len(consumed), consumed)
	}

	newInboxPath, err := handoffMgr.InboxPath(newSessID)
	if err != nil {
		t.Fatalf("InboxPath(%s) failed: %v", newSessID, err)
	}
	if _, err := os.Stat(newInboxPath); !os.IsNotExist(err) {
		t.Fatalf("new session inbox %q must not be created, stat err=%v", newInboxPath, err)
	}
}

// TC-ROUTE-06: 런처(launcher.Run) 오프라인 티켓 자동 소비 · 환경 주입 · 종료 정리
func TestHandoffRoute06_LauncherAutoConsumeAndCleanup(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	handoffMgr, _, sessionDir := newIsolatedHandoffManager(t)
	sessMgr := session.NewManager(sessionDir)
	ws := t.TempDir()

	const summary = "런처 오프라인 자동 소비 검증"
	item, err := handoffMgr.Enqueue(handoff.HandoffItem{
		Type:    handoff.TypeFix,
		From:    handoff.HandoffNode{Mode: handoff.ModeTest},
		To:      handoff.HandoffNode{Mode: handoff.ModeDev},
		Summary: summary,
		Targets: []string{"src/app.go"},
	})
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	approveRes, err := handoff.ApproveAndRoute(handoffMgr, sessMgr, ws, item.ID)
	if err != nil {
		t.Fatalf("ApproveAndRoute failed: %v", err)
	}
	if !approveRes.Buffered {
		t.Fatalf("expected buffered offline ticket before launcher.Run")
	}

	captureFile := filepath.Join(t.TempDir(), "child_capture.txt")
	sidFile := filepath.Join(t.TempDir(), "child_sid.txt")
	inboxFileRecord := filepath.Join(t.TempDir(), "child_inbox_path.txt")

	shellCmd := fmt.Sprintf(
		`{ env; echo "===INBOX_START==="; test -f "$GUARD_INBOX_PATH" && cat "$GUARD_INBOX_PATH"; echo "===INBOX_END==="; } > %q && printf '%%s' "$GUARD_SESSION_ID" > %q && printf '%%s' "$GUARD_INBOX_PATH" > %q`,
		captureFile, sidFile, inboxFileRecord,
	)

	var stdout, stderr bytes.Buffer
	l := launcher.New(sessMgr, &stdout, &stderr).WithHandoffManager(handoffMgr)
	exitCode := l.Run("dev", ws, []string{"sh", "-c", shellCmd})
	if exitCode != 0 {
		t.Fatalf("launcher.Run exited with %d; stdout=%s; stderr=%s", exitCode, stdout.String(), stderr.String())
	}

	// 버퍼 stdout에 ANSI OSC 시퀀스(\033]0;) 미포함 검증
	if strings.Contains(stdout.String(), "\033]0;") {
		t.Errorf("non-TTY stdout buffer must not contain ANSI OSC title sequence: %q", stdout.String())
	}

	// 자식 프로세스 환경 및 인박스 내용 검증
	childCaptureBytes, err := os.ReadFile(captureFile)
	if err != nil {
		t.Fatalf("ReadFile(captureFile) failed: %v", err)
	}
	childCapture := string(childCaptureBytes)

	childSIDBytes, err := os.ReadFile(sidFile)
	if err != nil {
		t.Fatalf("ReadFile(sidFile) failed: %v", err)
	}
	childSID := strings.TrimSpace(string(childSIDBytes))
	if childSID == "" {
		t.Fatalf("GUARD_SESSION_ID was empty in child process")
	}

	childInboxPathBytes, err := os.ReadFile(inboxFileRecord)
	if err != nil {
		t.Fatalf("ReadFile(inboxFileRecord) failed: %v", err)
	}
	childInboxPath := strings.TrimSpace(string(childInboxPathBytes))
	wantInboxPath := filepath.Join(sessionDir, childSID+".inbox")
	if childInboxPath != wantInboxPath {
		t.Errorf("GUARD_INBOX_PATH mismatch: want %q, got %q", wantInboxPath, childInboxPath)
	}

	for _, wantLine := range []string{
		"AGENT_MODE=DEV",
		"GUARD_SESSION_ID=" + childSID,
		"GUARD_INBOX_PATH=" + wantInboxPath,
		"<!-- GUARD_HANDOFF_ENVELOPE: START",
		item.ID,
		summary,
	} {
		if !strings.Contains(childCapture, wantLine) {
			t.Errorf("child output missing %q\n--- child output ---\n%s", wantLine, childCapture)
		}
	}

	// 일반 sh 명령 실행 시에는 워크스페이스 오염 방지를 위해 guard-workflow.md가 생성되지 않아야 함
	workflowRulePath := filepath.Join(ws, ".agents", "rules", "guard-workflow.md")
	if _, err := os.Stat(workflowRulePath); !os.IsNotExist(err) {
		t.Errorf("workflow rule %q must NOT be created for non-agy command (sh), stat err=%v", workflowRulePath, err)
	}

	// launcher.Run 종료 직후 세션 JSON 및 .inbox 파일 모두 삭제 확인
	sessJSONPath := sessMgr.SessionPath(childSID)
	if _, err := os.Stat(sessJSONPath); !os.IsNotExist(err) {
		t.Errorf("session file %q must be cleaned up after launcher exit, stat err=%v", sessJSONPath, err)
	}
	if _, err := os.Stat(wantInboxPath); !os.IsNotExist(err) {
		t.Errorf("inbox file %q must be cleaned up after launcher exit, stat err=%v", wantInboxPath, err)
	}

	// filepath.Base(cmdArgs[0]) == "agy" 실행 시 <ws>/.agents/rules/guard-workflow.md 자동 생성 검증
	fakeAgy := filepath.Join(t.TempDir(), "agy")
	if err := os.WriteFile(fakeAgy, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("WriteFile(fakeAgy) failed: %v", err)
	}
	var agyOut, agyErr bytes.Buffer
	agyLauncher := launcher.New(sessMgr, &agyOut, &agyErr).WithHandoffManager(handoffMgr)
	if code := agyLauncher.Run("dev", ws, []string{fakeAgy}); code != 0 {
		t.Fatalf("launcher.Run(agy) exited with %d; stdout=%s stderr=%s", code, agyOut.String(), agyErr.String())
	}
	if info, err := os.Stat(workflowRulePath); err != nil || info.Size() == 0 {
		t.Errorf("expected non-empty workflow rule at %q after agy launch, err=%v", workflowRulePath, err)
	}
}
