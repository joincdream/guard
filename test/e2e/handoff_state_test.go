package e2e_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"testing"

	"guard/internal/hub/handoff"
	"guard/internal/hub/session"
)

func newIsolatedHandoffManager(t *testing.T) (*handoff.Manager, string, string) {
	t.Helper()
	tmp := t.TempDir()
	handoffDir := filepath.Join(tmp, "handoff")
	sessionDir := filepath.Join(tmp, "sessions")
	mgr, err := handoff.NewManager(handoffDir, sessionDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	return mgr, handoffDir, sessionDir
}

// TC-STATE-01: 정상 생명주기 전이 및 ID/타임스탬프 계약
func TestHandoffState01_LifecycleAndIDTimestamp(t *testing.T) {
	mgr, _, _ := newIsolatedHandoffManager(t)

	enqueued, err := mgr.Enqueue(handoff.HandoffItem{
		Type:    handoff.TypeFix,
		From:    handoff.HandoffNode{Mode: handoff.ModeTest},
		To:      handoff.HandoffNode{Mode: handoff.ModeDev},
		Summary: "결함 수정",
	})
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	idPattern := regexp.MustCompile(`^hnd-\d{8}-[0-9a-f]{8}$`)
	if !idPattern.MatchString(enqueued.ID) {
		t.Fatalf("generated ID %q does not match contract regex %q", enqueued.ID, idPattern.String())
	}
	if enqueued.Status != handoff.StatusPending {
		t.Fatalf("expected initial status %q, got %q", handoff.StatusPending, enqueued.Status)
	}
	if enqueued.CreatedAt.IsZero() {
		t.Fatalf("expected non-zero CreatedAt")
	}

	persistedPending, err := mgr.Get(enqueued.ID)
	if err != nil {
		t.Fatalf("Get after Enqueue failed: %v", err)
	}
	if persistedPending.Status != handoff.StatusPending {
		t.Fatalf("persisted status expected %q, got %q", handoff.StatusPending, persistedPending.Status)
	}

	approved, err := mgr.UpdateStatus(enqueued.ID, handoff.StatusApproved, "")
	if err != nil {
		t.Fatalf("UpdateStatus(approved) failed: %v", err)
	}
	if approved.Status != handoff.StatusApproved {
		t.Fatalf("expected returned status %q, got %q", handoff.StatusApproved, approved.Status)
	}

	persistedApproved, err := mgr.Get(enqueued.ID)
	if err != nil {
		t.Fatalf("Get after approved failed: %v", err)
	}
	if persistedApproved.Status != handoff.StatusApproved {
		t.Fatalf("persisted status expected %q, got %q", handoff.StatusApproved, persistedApproved.Status)
	}
	if persistedApproved.UpdatedAt.Before(persistedApproved.CreatedAt) {
		t.Fatalf("UpdatedAt (%v) must be >= CreatedAt (%v)", persistedApproved.UpdatedAt, persistedApproved.CreatedAt)
	}

	acked, err := mgr.UpdateStatus(enqueued.ID, handoff.StatusAcked, "")
	if err != nil {
		t.Fatalf("UpdateStatus(acked) failed: %v", err)
	}
	if acked.Status != handoff.StatusAcked {
		t.Fatalf("expected returned status %q, got %q", handoff.StatusAcked, acked.Status)
	}

	persistedAcked, err := mgr.Get(enqueued.ID)
	if err != nil {
		t.Fatalf("Get after acked failed: %v", err)
	}
	if persistedAcked.Status != handoff.StatusAcked {
		t.Fatalf("persisted status expected %q, got %q", handoff.StatusAcked, persistedAcked.Status)
	}
	if persistedAcked.UpdatedAt.Before(persistedAcked.CreatedAt) {
		t.Fatalf("UpdatedAt (%v) must be >= CreatedAt (%v)", persistedAcked.UpdatedAt, persistedAcked.CreatedAt)
	}
	if persistedAcked.UpdatedAt.Before(persistedApproved.UpdatedAt) {
		t.Fatalf("acked UpdatedAt (%v) must be >= approved UpdatedAt (%v)", persistedAcked.UpdatedAt, persistedApproved.UpdatedAt)
	}
}

// TC-STATE-02: 정상 반려 전이 및 인박스 미전달
func TestHandoffState02_RejectAndNoInboxDelivery(t *testing.T) {
	mgr, _, sessionDir := newIsolatedHandoffManager(t)
	sessMgr := session.NewManager(sessionDir)
	ws := t.TempDir()

	activeSessID := "sess-dev-active-1"
	if err := sessMgr.Create(&session.Session{
		ID:        activeSessID,
		Mode:      "DEV",
		PID:       os.Getpid(),
		Workspace: ws,
	}); err != nil {
		t.Fatalf("Create active DEV session failed: %v", err)
	}

	item, err := mgr.Enqueue(handoff.HandoffItem{
		Type:    handoff.TypeFix,
		From:    handoff.HandoffNode{Mode: handoff.ModeTest},
		To:      handoff.HandoffNode{Mode: handoff.ModeDev},
		Summary: "반려 대상 티켓",
	})
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	const rejectReason = "요구사항 불명확"
	if _, err := mgr.UpdateStatus(item.ID, handoff.StatusRejected, rejectReason); err != nil {
		t.Fatalf("UpdateStatus(rejected) failed: %v", err)
	}

	got, err := mgr.Get(item.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Status != handoff.StatusRejected {
		t.Fatalf("expected status %q, got %q", handoff.StatusRejected, got.Status)
	}
	if got.RejectReason != rejectReason {
		t.Fatalf("expected reject reason %q, got %q", rejectReason, got.RejectReason)
	}

	inboxPath, err := mgr.InboxPath(activeSessID)
	if err != nil {
		t.Fatalf("InboxPath failed: %v", err)
	}
	if _, err := os.Stat(inboxPath); !os.IsNotExist(err) {
		t.Fatalf("expected inbox file %q not to exist after reject, stat err=%v", inboxPath, err)
	}
}

// TC-STATE-03: 금지된 상태 전이 전수 차단 (Adversarial)
func TestHandoffState03_InvalidTransitions(t *testing.T) {
	mgr, _, _ := newIsolatedHandoffManager(t)

	createTicket := func(summary string) handoff.HandoffItem {
		t.Helper()
		it, err := mgr.Enqueue(handoff.HandoffItem{
			Type:    handoff.TypeTask,
			From:    handoff.HandoffNode{Mode: handoff.ModePlan},
			To:      handoff.HandoffNode{Mode: handoff.ModeDev},
			Summary: summary,
		})
		if err != nil {
			t.Fatalf("Enqueue(%s) failed: %v", summary, err)
		}
		return it
	}

	pendingItem := createTicket("pending-item")

	approvedItem := createTicket("approved-item")
	if _, err := mgr.UpdateStatus(approvedItem.ID, handoff.StatusApproved, ""); err != nil {
		t.Fatalf("setup approved failed: %v", err)
	}

	rejectedItem := createTicket("rejected-item")
	const origRejectReason = "원본 반려 사유"
	if _, err := mgr.UpdateStatus(rejectedItem.ID, handoff.StatusRejected, origRejectReason); err != nil {
		t.Fatalf("setup rejected failed: %v", err)
	}

	ackedItem := createTicket("acked-item")
	if _, err := mgr.UpdateStatus(ackedItem.ID, handoff.StatusApproved, ""); err != nil {
		t.Fatalf("setup acked (approve) failed: %v", err)
	}
	if _, err := mgr.UpdateStatus(ackedItem.ID, handoff.StatusAcked, ""); err != nil {
		t.Fatalf("setup acked (ack) failed: %v", err)
	}

	cases := []struct {
		name           string
		id             string
		fromStatus     handoff.HandoffStatus
		origReason     string
		targetStatus   handoff.HandoffStatus
		attemptReason  string
	}{
		{"pending->acked", pendingItem.ID, handoff.StatusPending, "", handoff.StatusAcked, "bad"},
		{"pending->pending", pendingItem.ID, handoff.StatusPending, "", handoff.StatusPending, "bad"},
		{"approved->approved", approvedItem.ID, handoff.StatusApproved, "", handoff.StatusApproved, "bad"},
		{"approved->pending", approvedItem.ID, handoff.StatusApproved, "", handoff.StatusPending, "bad"},
		{"approved->rejected", approvedItem.ID, handoff.StatusApproved, "", handoff.StatusRejected, "override"},
		{"rejected->approved", rejectedItem.ID, handoff.StatusRejected, origRejectReason, handoff.StatusApproved, ""},
		{"rejected->acked", rejectedItem.ID, handoff.StatusRejected, origRejectReason, handoff.StatusAcked, ""},
		{"rejected->pending", rejectedItem.ID, handoff.StatusRejected, origRejectReason, handoff.StatusPending, ""},
		{"rejected->rejected", rejectedItem.ID, handoff.StatusRejected, origRejectReason, handoff.StatusRejected, "mutated"},
		{"acked->approved", ackedItem.ID, handoff.StatusAcked, "", handoff.StatusApproved, ""},
		{"acked->rejected", ackedItem.ID, handoff.StatusAcked, "", handoff.StatusRejected, "late reject"},
		{"acked->pending", ackedItem.ID, handoff.StatusAcked, "", handoff.StatusPending, ""},
		{"acked->acked", ackedItem.ID, handoff.StatusAcked, "", handoff.StatusAcked, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := handoff.ValidateStatusTransition(tc.fromStatus, tc.targetStatus); !errors.Is(err, handoff.ErrInvalidTransition) {
				t.Errorf("ValidateStatusTransition(%s -> %s): expected ErrInvalidTransition, got %v", tc.fromStatus, tc.targetStatus, err)
			}

			_, err := mgr.UpdateStatus(tc.id, tc.targetStatus, tc.attemptReason)
			if !errors.Is(err, handoff.ErrInvalidTransition) {
				t.Fatalf("UpdateStatus(%s -> %s): expected ErrInvalidTransition, got %v", tc.fromStatus, tc.targetStatus, err)
			}

			got, err := mgr.Get(tc.id)
			if err != nil {
				t.Fatalf("Get after failed transition failed: %v", err)
			}
			if got.Status != tc.fromStatus {
				t.Errorf("status mutated on failed transition: want %q, got %q", tc.fromStatus, got.Status)
			}
			if got.RejectReason != tc.origReason {
				t.Errorf("reject reason mutated on failed transition: want %q, got %q", tc.origReason, got.RejectReason)
			}
		})
	}
}

// TC-STATE-04: 입력 유효성 위반 및 경로 탈출 공격 (Adversarial)
func TestHandoffState04_ValidationAndPathTraversal(t *testing.T) {
	mgr, _, _ := newIsolatedHandoffManager(t)

	// (1) ParseTarget 비정상 입력
	badTargets := []string{
		"",
		"   ",
		"../etc/passwd",
		"..",
		".",
		"sess/1",
		`sess\1`,
	}
	for _, raw := range badTargets {
		node, err := handoff.ParseTarget(raw)
		if !errors.Is(err, handoff.ErrInvalidItem) {
			t.Errorf("ParseTarget(%q): expected ErrInvalidItem, got node=%+v err=%v", raw, node, err)
		}
	}

	// (2) InboxPath 비정상 입력
	badSessionIDs := []string{
		"../evil",
		"a/b",
		"",
	}
	for _, sid := range badSessionIDs {
		p, err := mgr.InboxPath(sid)
		if !errors.Is(err, handoff.ErrInvalidSessionID) {
			t.Errorf("InboxPath(%q): expected ErrInvalidSessionID, got path=%q err=%v", sid, p, err)
		}
	}

	// (3) Enqueue 비정상 아이템
	baseValid := func() handoff.HandoffItem {
		return handoff.HandoffItem{
			Type:    handoff.TypeFix,
			From:    handoff.HandoffNode{Mode: handoff.ModeTest},
			To:      handoff.HandoffNode{Mode: handoff.ModeDev},
			Summary: "유효 요약",
		}
	}

	badItems := []struct {
		name string
		item handoff.HandoffItem
	}{
		{
			name: "empty summary",
			item: func() handoff.HandoffItem { it := baseValid(); it.Summary = ""; return it }(),
		},
		{
			name: "unknown type deploy",
			item: func() handoff.HandoffItem { it := baseValid(); it.Type = handoff.HandoffType("deploy"); return it }(),
		},
		{
			name: "empty element in references",
			item: func() handoff.HandoffItem { it := baseValid(); it.References = []string{"doc.md", ""}; return it }(),
		},
		{
			name: "empty element in targets",
			item: func() handoff.HandoffItem { it := baseValid(); it.Targets = []string{""}; return it }(),
		},
		{
			name: "path traversal parent_id",
			item: func() handoff.HandoffItem { it := baseValid(); it.ParentID = "../bad"; return it }(),
		},
	}

	for _, tc := range badItems {
		t.Run("Enqueue/"+tc.name, func(t *testing.T) {
			_, err := mgr.Enqueue(tc.item)
			if !errors.Is(err, handoff.ErrInvalidItem) {
				t.Errorf("Enqueue(%s): expected ErrInvalidItem, got %v", tc.name, err)
			}
		})
	}

	// (4) 동일 ID 중복 Enqueue 차단 및 기존 티켓 무손상 검증
	dupID := "hnd-20261121-deadbeef"
	first := baseValid()
	first.ID = dupID
	first.Summary = "첫 번째 원본 티켓"
	if _, err := mgr.Enqueue(first); err != nil {
		t.Fatalf("first Enqueue with explicit ID failed: %v", err)
	}

	second := baseValid()
	second.ID = dupID
	second.Summary = "두 번째 덮어쓰기 시도"
	if _, err := mgr.Enqueue(second); err == nil {
		t.Fatalf("duplicate ID Enqueue succeeded unexpectedly; expected error")
	}

	got, err := mgr.Get(dupID)
	if err != nil {
		t.Fatalf("Get after duplicate Enqueue failed: %v", err)
	}
	if got.Summary != first.Summary {
		t.Fatalf("existing ticket corrupted after duplicate Enqueue: want %q, got %q", first.Summary, got.Summary)
	}
}

// TC-STATE-05: 동시 쓰기 및 동일 티켓 승인 경합 (flock + Atomic Write)
func TestHandoffState05_ConcurrencyFlock(t *testing.T) {
	mgr, _, _ := newIsolatedHandoffManager(t)

	// (1) 10개 고루틴 x 5건 = 50건 동시 Enqueue
	const workers = 10
	const perWorker = 5
	startEnqueue := make(chan struct{})
	var wg sync.WaitGroup
	errCh := make(chan error, workers*perWorker)
	idCh := make(chan string, workers*perWorker)

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(workerIdx int) {
			defer wg.Done()
			<-startEnqueue
			for i := 0; i < perWorker; i++ {
				it, err := mgr.Enqueue(handoff.HandoffItem{
					Type:    handoff.TypeTask,
					From:    handoff.HandoffNode{Mode: handoff.ModePlan},
					To:      handoff.HandoffNode{Mode: handoff.ModeDev},
					Summary: fmt.Sprintf("worker-%d-item-%d", workerIdx, i),
				})
				if err != nil {
					errCh <- err
					return
				}
				idCh <- it.ID
			}
		}(w)
	}
	close(startEnqueue)
	wg.Wait()
	close(errCh)
	close(idCh)

	for err := range errCh {
		t.Fatalf("concurrent Enqueue error: %v", err)
	}

	createdIDs := make(map[string]struct{}, workers*perWorker)
	for id := range idCh {
		if _, dup := createdIDs[id]; dup {
			t.Fatalf("duplicate ID generated during concurrent Enqueue: %q", id)
		}
		createdIDs[id] = struct{}{}
	}
	if len(createdIDs) != workers*perWorker {
		t.Fatalf("expected %d created IDs, got %d", workers*perWorker, len(createdIDs))
	}

	// queue.json 유효 JSON 확인
	rawQueue, err := os.ReadFile(mgr.QueuePath())
	if err != nil {
		t.Fatalf("ReadFile(QueuePath) failed: %v", err)
	}
	if !json.Valid(rawQueue) {
		t.Fatalf("queue.json is not valid JSON after concurrent writes")
	}

	listed, err := mgr.List(handoff.ListFilter{})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(listed) != workers*perWorker {
		t.Fatalf("expected %d items in queue, got %d", workers*perWorker, len(listed))
	}
	seenListed := make(map[string]struct{}, len(listed))
	for _, it := range listed {
		if _, dup := seenListed[it.ID]; dup {
			t.Fatalf("duplicate ID %q found in queue list", it.ID)
		}
		seenListed[it.ID] = struct{}{}
	}
	for id := range createdIDs {
		if _, ok := seenListed[id]; !ok {
			t.Fatalf("created ID %q missing from queue list", id)
		}
	}

	// (2) 단일 pending 티켓에 대해 10개 고루틴 동시 UpdateStatus(approved)
	target, err := mgr.Enqueue(handoff.HandoffItem{
		Type:    handoff.TypeFix,
		From:    handoff.HandoffNode{Mode: handoff.ModeTest},
		To:      handoff.HandoffNode{Mode: handoff.ModeDev},
		Summary: "승인 경합 대상",
	})
	if err != nil {
		t.Fatalf("Enqueue race target failed: %v", err)
	}

	const approvers = 10
	startApprove := make(chan struct{})
	results := make(chan error, approvers)
	var approveWG sync.WaitGroup
	for i := 0; i < approvers; i++ {
		approveWG.Add(1)
		go func() {
			defer approveWG.Done()
			<-startApprove
			_, err := mgr.UpdateStatus(target.ID, handoff.StatusApproved, "")
			results <- err
		}()
	}
	close(startApprove)
	approveWG.Wait()
	close(results)

	successCount := 0
	invalidTransitionCount := 0
	for err := range results {
		if err == nil {
			successCount++
		} else if errors.Is(err, handoff.ErrInvalidTransition) {
			invalidTransitionCount++
		} else {
			t.Fatalf("unexpected error during concurrent approve: %v", err)
		}
	}
	if successCount != 1 {
		t.Fatalf("expected exactly 1 successful approve, got %d", successCount)
	}
	if invalidTransitionCount != approvers-1 {
		t.Fatalf("expected exactly %d ErrInvalidTransition, got %d", approvers-1, invalidTransitionCount)
	}
}

// TC-STATE-06: 큐 Compaction (DefaultMaxQueueItems = 200) 활성 티켓 보존
func TestHandoffState06_QueueCompaction(t *testing.T) {
	mgr, _, _ := newIsolatedHandoffManager(t)

	completedOrder := make([]string, 0, 180)

	// 1. 완료 티켓 180건 (acked 100건 + rejected 80건)
	for i := 0; i < 100; i++ {
		it, err := mgr.Enqueue(handoff.HandoffItem{
			Type:    handoff.TypeTask,
			From:    handoff.HandoffNode{Mode: handoff.ModePlan},
			To:      handoff.HandoffNode{Mode: handoff.ModeDev},
			Summary: fmt.Sprintf("acked-%03d", i),
		})
		if err != nil {
			t.Fatalf("Enqueue acked-%d failed: %v", i, err)
		}
		if _, err := mgr.UpdateStatus(it.ID, handoff.StatusApproved, ""); err != nil {
			t.Fatalf("approve acked-%d failed: %v", i, err)
		}
		if _, err := mgr.UpdateStatus(it.ID, handoff.StatusAcked, ""); err != nil {
			t.Fatalf("ack acked-%d failed: %v", i, err)
		}
		completedOrder = append(completedOrder, it.ID)
	}

	for i := 0; i < 80; i++ {
		it, err := mgr.Enqueue(handoff.HandoffItem{
			Type:    handoff.TypeReview,
			From:    handoff.HandoffNode{Mode: handoff.ModeDev},
			To:      handoff.HandoffNode{Mode: handoff.ModePlan},
			Summary: fmt.Sprintf("rejected-%03d", i),
		})
		if err != nil {
			t.Fatalf("Enqueue rejected-%d failed: %v", i, err)
		}
		if _, err := mgr.UpdateStatus(it.ID, handoff.StatusRejected, "compaction test"); err != nil {
			t.Fatalf("reject rejected-%d failed: %v", i, err)
		}
		completedOrder = append(completedOrder, it.ID)
	}

	// 2. 활성 티켓 50건 (pending 25건 + approved 25건) -> 총 230건 > 200
	activeIDs := make(map[string]handoff.HandoffStatus, 50)
	for i := 0; i < 25; i++ {
		it, err := mgr.Enqueue(handoff.HandoffItem{
			Type:    handoff.TypeFix,
			From:    handoff.HandoffNode{Mode: handoff.ModeTest},
			To:      handoff.HandoffNode{Mode: handoff.ModeDev},
			Summary: fmt.Sprintf("active-pending-%02d", i),
		})
		if err != nil {
			t.Fatalf("Enqueue active-pending-%d failed: %v", i, err)
		}
		activeIDs[it.ID] = handoff.StatusPending
	}
	for i := 0; i < 25; i++ {
		it, err := mgr.Enqueue(handoff.HandoffItem{
			Type:    handoff.TypeVerify,
			From:    handoff.HandoffNode{Mode: handoff.ModeDev},
			To:      handoff.HandoffNode{Mode: handoff.ModeTest},
			Summary: fmt.Sprintf("active-approved-%02d", i),
		})
		if err != nil {
			t.Fatalf("Enqueue active-approved-%d failed: %v", i, err)
		}
		if _, err := mgr.UpdateStatus(it.ID, handoff.StatusApproved, ""); err != nil {
			t.Fatalf("approve active-approved-%d failed: %v", i, err)
		}
		activeIDs[it.ID] = handoff.StatusApproved
	}

	// 활성 티켓 50건 무손실 확인
	activeList, err := mgr.List(handoff.ListFilter{OnlyActive: true})
	if err != nil {
		t.Fatalf("List(OnlyActive) failed: %v", err)
	}
	if len(activeList) != 50 {
		t.Fatalf("expected exactly 50 active items preserved, got %d", len(activeList))
	}
	for _, it := range activeList {
		wantStatus, ok := activeIDs[it.ID]
		if !ok {
			t.Errorf("unexpected active item ID %q", it.ID)
			continue
		}
		if it.Status != wantStatus {
			t.Errorf("active item %q status mismatch: want %q, got %q", it.ID, wantStatus, it.Status)
		}
	}

	// 전체 티켓 수 <= DefaultMaxQueueItems (200) 확인
	allList, err := mgr.List(handoff.ListFilter{})
	if err != nil {
		t.Fatalf("List(all) failed: %v", err)
	}
	if len(allList) > handoff.DefaultMaxQueueItems {
		t.Fatalf("expected total items <= %d, got %d", handoff.DefaultMaxQueueItems, len(allList))
	}

	// 제거된 티켓이 가장 오래된 완료 티켓 30건인지 검증
	presentIDs := make(map[string]struct{}, len(allList))
	for _, it := range allList {
		presentIDs[it.ID] = struct{}{}
	}
	evictedCount := 0
	for idx, id := range completedOrder {
		_, present := presentIDs[id]
		if !present {
			evictedCount++
			// 앞에서부터 연속적으로 제거되었는지 확인 (이미 남아있는 완료 티켓보다 뒤에 있는 티켓이 제거되면 안 됨)
			if idx >= 30 {
				t.Errorf("completed item at index %d (%s) was evicted, expected only oldest 30 completed items to be evicted", idx, id)
			}
		}
	}
	if evictedCount < 30 {
		t.Fatalf("expected at least 30 oldest completed items evicted, got %d", evictedCount)
	}
}
