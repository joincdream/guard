package session_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"guard/internal/core"
	"guard/internal/hub/session"
)

// TC-P2-008: Create 잘못된 입력 방어 (nil, 빈 ID, 공백 ID)
func TestPhase2_Session_CreateInvalidInputs(t *testing.T) {
	dir := t.TempDir()
	mgr := session.NewManager(dir)

	if err := mgr.Create(nil); err == nil {
		t.Errorf("Create(nil) expected error, got nil")
	}
	if err := mgr.Create(&session.Session{ID: ""}); err == nil {
		t.Errorf("Create(empty ID) expected error, got nil")
	}
	if err := mgr.Create(&session.Session{ID: "   "}); err == nil {
		t.Errorf("Create(whitespace ID) expected error, got nil")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 files created on invalid Create calls, got %d", len(entries))
	}
}

// TC-P2-009: Create 기본값 초기화(빈 모드 -> DEV, 소문자 정규화, Events non-nil) 및 원자적 쓰기
func TestPhase2_Session_CreateDefaultsAndAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	mgr := session.NewManager(dir)

	if err := mgr.Create(&session.Session{
		ID:        "sess-empty-mode",
		Mode:      "",
		PID:       os.Getpid(),
		Workspace: "/ws/default",
		Events:    nil,
	}); err != nil {
		t.Fatalf("Create empty mode: %v", err)
	}

	if err := mgr.Create(&session.Session{
		ID:        "sess-lower-mode",
		Mode:      " plan ",
		PID:       os.Getpid(),
		Workspace: "/ws/plan",
		Events:    nil,
	}); err != nil {
		t.Fatalf("Create lower mode: %v", err)
	}

	gotEmpty, err := mgr.Get("sess-empty-mode")
	if err != nil {
		t.Fatalf("Get(sess-empty-mode): %v", err)
	}
	if gotEmpty.Mode != core.ModeDev {
		t.Errorf("empty Mode normalized to %q, want %q", gotEmpty.Mode, core.ModeDev)
	}
	if gotEmpty.CreatedAt.IsZero() || gotEmpty.UpdatedAt.IsZero() {
		t.Errorf("expected non-zero CreatedAt/UpdatedAt, got %v / %v", gotEmpty.CreatedAt, gotEmpty.UpdatedAt)
	}
	if gotEmpty.Events == nil {
		t.Errorf("expected non-nil Events slice from Get()")
	}

	gotPlan, err := mgr.Get("sess-lower-mode")
	if err != nil {
		t.Fatalf("Get(sess-lower-mode): %v", err)
	}
	if gotPlan.Mode != core.ModePlan {
		t.Errorf("lower Mode normalized to %q, want %q", gotPlan.Mode, core.ModePlan)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("leftover temp file found after atomic write: %s", e.Name())
		}
	}
}

// TC-P2-010: Path Traversal & 숨김 파일 ID 방어
func TestPhase2_Session_IDSanitizationAndHiddenFileDefense(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "sessions")
	mgr := session.NewManager(dir)

	evilIDs := []string{"../evil", "a/b", `a\b`, ".hidden", "..hidden"}
	for _, id := range evilIDs {
		p := mgr.SessionPath(id)
		rel, err := filepath.Rel(dir, p)
		if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
			t.Fatalf("SessionPath(%q) = %q escaped session dir %q", id, p, dir)
		}
		if strings.HasPrefix(filepath.Base(p), ".") {
			t.Errorf("SessionPath(%q) base %q must not start with '.'", id, filepath.Base(p))
		}

		if err := mgr.Create(&session.Session{
			ID:        id,
			Mode:      core.ModeDev,
			PID:       os.Getpid(),
			Workspace: "/ws",
		}); err != nil {
			t.Fatalf("Create(%q): %v", id, err)
		}
	}

	// Verify no files escaped into parent directory
	parentEntries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatalf("ReadDir(parent): %v", err)
	}
	for _, e := range parentEntries {
		if e.Name() != "sessions" {
			t.Errorf("path traversal detected: unexpected entry %q in parent dir", e.Name())
		}
	}

	// Verify all sanitized IDs appear in List() (none dropped by hidden-file filter)
	list, err := mgr.List()
	if err != nil {
		t.Fatalf("List(): %v", err)
	}
	found := make(map[string]bool, len(list))
	for _, s := range list {
		found[s.ID] = true
	}
	for _, id := range evilIDs {
		if !found[id] {
			t.Errorf("session %q missing from List() (found: %v)", id, found)
		}
	}
}

// TC-P2-011: List() 정렬 불변성 (UpdatedAt 내림차순, 동률 시 ID 오름차순) 및 손상/.lock 파일 내성
func TestPhase2_Session_ListSortingAndResilience(t *testing.T) {
	dir := t.TempDir()
	mgr := session.NewManager(dir)

	t1 := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 11, 20, 11, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)

	sessions := []*session.Session{
		{ID: "oldest", Mode: core.ModeDev, PID: os.Getpid(), Workspace: "/ws", CreatedAt: t1, UpdatedAt: t1, Events: []session.Event{}},
		{ID: "tie-b", Mode: core.ModeDev, PID: os.Getpid(), Workspace: "/ws", CreatedAt: t2, UpdatedAt: t2, Events: []session.Event{}},
		{ID: "tie-a", Mode: core.ModeDev, PID: os.Getpid(), Workspace: "/ws", CreatedAt: t2, UpdatedAt: t2, Events: []session.Event{}},
		{ID: "newest", Mode: core.ModeDev, PID: os.Getpid(), Workspace: "/ws", CreatedAt: t3, UpdatedAt: t3, Events: []session.Event{}},
	}
	for _, s := range sessions {
		if err := mgr.Save(s); err != nil {
			t.Fatalf("Save(%s): %v", s.ID, err)
		}
	}

	// Inject corrupted JSON, hidden file, and .lock file
	if err := os.WriteFile(filepath.Join(dir, "broken.json"), []byte("{corrupted"), 0o644); err != nil {
		t.Fatalf("write broken.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".hidden.json"), []byte(`{"id":"ignored"}`), 0o644); err != nil {
		t.Fatalf("write .hidden.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "newest.json.lock"), []byte(""), 0o644); err != nil {
		t.Fatalf("write lock file: %v", err)
	}

	got, err := mgr.List()
	if err != nil {
		t.Fatalf("List(): %v", err)
	}

	wantOrder := []string{"newest", "tie-a", "tie-b", "oldest"}
	if len(got) != len(wantOrder) {
		t.Fatalf("List() len = %d, want %d", len(got), len(wantOrder))
	}
	for i, wantID := range wantOrder {
		if got[i].ID != wantID {
			t.Errorf("List()[%d].ID = %q, want %q (full order: %v)", i, got[i].ID, wantID, got)
		}
	}
}

// TC-P2-012: Delete 멱등성 & DeleteAll 반환 건수 정확성 (.lock 미카운트)
func TestPhase2_Session_DeleteIdempotencyAndDeleteAllCount(t *testing.T) {
	dir := t.TempDir()
	mgr := session.NewManager(dir)

	// Deleting nonexistent session must not error
	if err := mgr.Delete("never-existed"); err != nil {
		t.Fatalf("Delete(nonexistent) = %v, want nil", err)
	}

	for _, id := range []string{"s1", "s2", "s3"} {
		if err := mgr.Create(&session.Session{ID: id, Mode: core.ModeDev, PID: os.Getpid(), Workspace: "/ws"}); err != nil {
			t.Fatalf("Create(%s): %v", id, err)
		}
		// Trigger flock to create .lock files
		if _, err := mgr.UpdateMode(id, core.ModeTest); err != nil {
			t.Fatalf("UpdateMode(%s): %v", id, err)
		}
	}

	// Delete s1 -> both json and lock should be gone
	if err := mgr.Delete("s1"); err != nil {
		t.Fatalf("Delete(s1): %v", err)
	}
	if _, err := os.Stat(mgr.SessionPath("s1")); !os.IsNotExist(err) {
		t.Errorf("s1 json still exists after Delete")
	}
	if _, err := os.Stat(mgr.SessionPath("s1") + ".lock"); !os.IsNotExist(err) {
		t.Errorf("s1 .lock still exists after Delete")
	}

	// DeleteAll should count only remaining .json files (s2, s3 = 2), not .lock files
	count, err := mgr.DeleteAll()
	if err != nil {
		t.Fatalf("DeleteAll(): %v", err)
	}
	if count != 2 {
		t.Errorf("DeleteAll() count = %d, want 2", count)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".json") || strings.HasSuffix(e.Name(), ".lock") {
			t.Errorf("unexpected leftover file after DeleteAll: %s", e.Name())
		}
	}
}

// TC-P2-013: AppendEvent 로테이션 경계값 (50 / 51 / 100건) 및 ViolationCount 누적 정확성
func TestPhase2_Session_AppendEventRotationBoundaries(t *testing.T) {
	dir := t.TempDir()
	mgr := session.NewManager(dir)

	if err := mgr.Create(&session.Session{ID: "rot", Mode: core.ModeDev, PID: os.Getpid(), Workspace: "/ws"}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	appendBatch := func(start, end int) {
		t.Helper()
		for i := start; i <= end; i++ {
			dec := "allow"
			if i%2 == 1 {
				dec = "deny"
			}
			if _, err := mgr.AppendEvent("rot", session.Event{
				Decision: dec,
				Tool:     "read_file",
				Target:   fmt.Sprintf("f_%03d.go", i),
				Reason:   "reason",
			}); err != nil {
				t.Fatalf("AppendEvent(%d): %v", i, err)
			}
		}
	}

	// Exactly MaxEvents (50)
	appendBatch(1, 50)
	s50, err := mgr.Get("rot")
	if err != nil {
		t.Fatalf("Get after 50: %v", err)
	}
	if len(s50.Events) != session.MaxEvents || s50.Events[0].Target != "f_001.go" || s50.Events[49].Target != "f_050.go" {
		t.Fatalf("at 50: len=%d first=%q last=%q", len(s50.Events), s50.Events[0].Target, s50.Events[len(s50.Events)-1].Target)
	}
	if s50.ViolationCount != 25 {
		t.Errorf("at 50: ViolationCount = %d, want 25", s50.ViolationCount)
	}

	// 51st event triggers rotation
	appendBatch(51, 51)
	s51, err := mgr.Get("rot")
	if err != nil {
		t.Fatalf("Get after 51: %v", err)
	}
	if len(s51.Events) != session.MaxEvents || s51.Events[0].Target != "f_002.go" || s51.Events[49].Target != "f_051.go" {
		t.Fatalf("at 51: len=%d first=%q last=%q", len(s51.Events), s51.Events[0].Target, s51.Events[len(s51.Events)-1].Target)
	}
	if s51.ViolationCount != 26 {
		t.Errorf("at 51: ViolationCount = %d, want 26", s51.ViolationCount)
	}

	// Up to 100 events: ViolationCount must remain cumulative (50 denies)
	appendBatch(52, 100)
	s100, err := mgr.Get("rot")
	if err != nil {
		t.Fatalf("Get after 100: %v", err)
	}
	if len(s100.Events) != session.MaxEvents || s100.Events[0].Target != "f_051.go" || s100.Events[49].Target != "f_100.go" {
		t.Fatalf("at 100: len=%d first=%q last=%q", len(s100.Events), s100.Events[0].Target, s100.Events[len(s100.Events)-1].Target)
	}
	if s100.ViolationCount != 50 {
		t.Errorf("at 100: ViolationCount = %d, want 50", s100.ViolationCount)
	}

	// ResetViolations resets counter without dropping events
	reset, err := mgr.ResetViolations("rot")
	if err != nil {
		t.Fatalf("ResetViolations: %v", err)
	}
	if reset.ViolationCount != 0 || len(reset.Events) != session.MaxEvents {
		t.Errorf("after reset: ViolationCount=%d len(Events)=%d, want 0 and %d", reset.ViolationCount, len(reset.Events), session.MaxEvents)
	}
}

// TC-P2-014: CleanAndList GC 정책 (PID <= 0 및 사망 PID 즉시 삭제, 활성 PID만 생존)
func TestPhase2_Session_CleanAndListGCPolicy(t *testing.T) {
	dir := t.TempDir()
	mgr := session.NewManager(dir)

	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Fatalf("spawn helper: %v", err)
	}
	deadPID := cmd.Process.Pid

	if session.IsProcessAlive(0) || session.IsProcessAlive(-5) {
		t.Fatalf("expected IsProcessAlive(<=0) to be false")
	}
	if !session.IsProcessAlive(os.Getpid()) || session.IsProcessAlive(deadPID) {
		t.Fatalf("unexpected IsProcessAlive results for self=%d dead=%d", os.Getpid(), deadPID)
	}

	fixtures := []*session.Session{
		{ID: "alive", Mode: core.ModeDev, PID: os.Getpid(), Workspace: "/ws1"},
		{ID: "dead", Mode: core.ModePlan, PID: deadPID, Workspace: "/ws2"},
		{ID: "unbound-zero", Mode: core.ModeTest, PID: 0, Workspace: "/ws3"},
		{ID: "unbound-neg", Mode: core.ModeTest, PID: -5, Workspace: "/ws4"},
	}
	for _, f := range fixtures {
		if err := mgr.Create(f); err != nil {
			t.Fatalf("Create(%s): %v", f.ID, err)
		}
	}

	aliveList, removed, err := mgr.CleanAndList()
	if err != nil {
		t.Fatalf("CleanAndList(): %v", err)
	}

	if len(aliveList) != 1 || aliveList[0].ID != "alive" {
		t.Fatalf("aliveList = %+v, want only [alive]", aliveList)
	}

	removedSet := make(map[string]bool, len(removed))
	for _, id := range removed {
		removedSet[id] = true
	}
	for _, wantRemoved := range []string{"dead", "unbound-zero", "unbound-neg"} {
		if !removedSet[wantRemoved] {
			t.Errorf("expected %q in removed list, got %v", wantRemoved, removed)
		}
	}
	if len(removed) != 3 {
		t.Errorf("len(removed) = %d, want 3 (%v)", len(removed), removed)
	}
}

// TC-P2-015: Flock 동시성 무결성 (Lost Update 0건, JSON 손상 0건, -race 통과)
func TestPhase2_Session_ConcurrentFlockIntegrity(t *testing.T) {
	dir := t.TempDir()
	mgr := session.NewManager(dir)

	if err := mgr.Create(&session.Session{
		ID:        "concurrent-sess",
		Mode:      core.ModeDev,
		PID:       os.Getpid(),
		Workspace: "/ws",
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	const (
		appendWorkers  = 20
		eventsPerWork  = 5
		modeWorkers    = 5
		totalExpected  = appendWorkers * eventsPerWork
	)

	var wg sync.WaitGroup
	errCh := make(chan error, totalExpected+modeWorkers)

	for w := 0; w < appendWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < eventsPerWork; i++ {
				if _, err := mgr.AppendEvent("concurrent-sess", session.Event{
					Decision: "deny",
					Tool:     "read_file",
					Target:   fmt.Sprintf("w%02d_e%02d_test.go", workerID, i),
					Reason:   "concurrent deny",
				}); err != nil {
					errCh <- fmt.Errorf("worker %d iter %d AppendEvent: %w", workerID, i, err)
				}
			}
		}(w)
	}

	modes := []string{core.ModeDev, core.ModeTest, core.ModePlan, core.ModeAll, core.ModeDev}
	for m := 0; m < modeWorkers; m++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			if _, err := mgr.UpdateMode("concurrent-sess", modes[idx]); err != nil {
				errCh <- fmt.Errorf("modeWorker %d UpdateMode: %w", idx, err)
			}
		}(m)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent error: %v", err)
	}

	got, err := mgr.Get("concurrent-sess")
	if err != nil {
		t.Fatalf("Get after concurrent writes: %v", err)
	}
	if got.ViolationCount != totalExpected {
		t.Errorf("ViolationCount = %d, want %d (Lost Update detected!)", got.ViolationCount, totalExpected)
	}
	if len(got.Events) != session.MaxEvents {
		t.Errorf("len(Events) = %d, want %d", len(got.Events), session.MaxEvents)
	}
}

// TC-P2-016: ShortID 경계값
func TestPhase2_Session_ShortIDBoundaries(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "", want: ""},
		{in: "1234567", want: "1234567"},
		{in: "12345678", want: "12345678"},
		{in: "123456789", want: "12345678"},
		{in: "550e8400-e29b-41d4-a716-446655440000", want: "550e8400"},
	}

	for _, tc := range tests {
		if got := session.ShortID(tc.in); got != tc.want {
			t.Errorf("ShortID(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
