package session

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestManager_CRUDAndAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	sess := &Session{
		ID:        "sess-001",
		Mode:      "dev",
		PID:       os.Getpid(),
		Workspace: "/tmp/workspace",
	}
	if err := mgr.Create(sess); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Verify no leftover temp files after atomic write
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("leftover temp file found: %s", e.Name())
		}
	}

	// Verify Get
	got, err := mgr.Get("sess-001")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Mode != "DEV" {
		t.Errorf("expected normalized Mode DEV, got %q", got.Mode)
	}
	if got.PID != os.Getpid() {
		t.Errorf("expected PID %d, got %d", os.Getpid(), got.PID)
	}

	// Verify UpdateMode
	updated, err := mgr.UpdateMode("sess-001", "test")
	if err != nil {
		t.Fatalf("UpdateMode failed: %v", err)
	}
	if updated.Mode != "TEST" {
		t.Errorf("expected Mode TEST, got %q", updated.Mode)
	}

	// Verify List ignores corrupted JSON files
	if err := os.WriteFile(filepath.Join(dir, "broken.json"), []byte("{not-json"), 0o644); err != nil {
		t.Fatalf("write broken json: %v", err)
	}
	list, err := mgr.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 1 || list[0].ID != "sess-001" {
		t.Fatalf("expected 1 valid session, got %d", len(list))
	}

	// Verify Delete
	if err := mgr.Delete("sess-001"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if _, err := mgr.Get("sess-001"); !os.IsNotExist(err) {
		t.Fatalf("expected os.ErrNotExist after Delete, got %v", err)
	}
}

func TestManager_AppendEventRotationAndResetViolations(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	if err := mgr.Create(&Session{
		ID:        "sess-events",
		Mode:      "DEV",
		PID:       os.Getpid(),
		Workspace: "/repo",
	}); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Append 60 events (30 allow, 30 deny) -> should rotate to MaxEvents (50) and count 30 violations
	for i := 1; i <= 60; i++ {
		decision := "allow"
		if i%2 == 0 {
			decision = "deny"
		}
		_, err := mgr.AppendEvent("sess-events", Event{
			Decision: decision,
			Tool:     "read_file",
			Target:   fmt.Sprintf("file_%02d.go", i),
			Reason:   "test reason",
		})
		if err != nil {
			t.Fatalf("AppendEvent %d failed: %v", i, err)
		}
	}

	got, err := mgr.Get("sess-events")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if len(got.Events) != MaxEvents {
		t.Fatalf("expected %d rotated events, got %d", MaxEvents, len(got.Events))
	}
	if got.Events[0].Target != "file_11.go" {
		t.Errorf("expected oldest retained event to be file_11.go, got %s", got.Events[0].Target)
	}
	if got.Events[len(got.Events)-1].Target != "file_60.go" {
		t.Errorf("expected newest retained event to be file_60.go, got %s", got.Events[len(got.Events)-1].Target)
	}
	if got.ViolationCount != 30 {
		t.Errorf("expected ViolationCount 30, got %d", got.ViolationCount)
	}

	// ResetViolations
	reset, err := mgr.ResetViolations("sess-events")
	if err != nil {
		t.Fatalf("ResetViolations failed: %v", err)
	}
	if reset.ViolationCount != 0 {
		t.Errorf("expected ViolationCount 0 after reset, got %d", reset.ViolationCount)
	}
}

func TestManager_CleanStaleSessions(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	// Spawn a short-lived process and wait for it to exit to obtain a guaranteed dead PID.
	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Fatalf("run helper process: %v", err)
	}
	deadPID := cmd.Process.Pid

	// Sanity check real IsProcessAlive behavior
	if !IsProcessAlive(os.Getpid()) {
		t.Fatalf("expected current process PID %d to be alive", os.Getpid())
	}
	if IsProcessAlive(deadPID) {
		t.Fatalf("expected exited process PID %d to be dead", deadPID)
	}

	// Create 3 sessions: alive, dead, unbound (PID 0)
	_ = mgr.Create(&Session{ID: "alive-sess", Mode: "DEV", PID: os.Getpid(), Workspace: "/ws1"})
	time.Sleep(5 * time.Millisecond)
	_ = mgr.Create(&Session{ID: "dead-sess", Mode: "PLAN", PID: deadPID, Workspace: "/ws2"})
	time.Sleep(5 * time.Millisecond)
	_ = mgr.Create(&Session{ID: "unbound-sess", Mode: "TEST", PID: 0, Workspace: "/ws3"})

	removed, err := mgr.CleanStaleSessions()
	if err != nil {
		t.Fatalf("CleanStaleSessions failed: %v", err)
	}
	removedSet := make(map[string]bool, len(removed))
	for _, id := range removed {
		removedSet[id] = true
	}
	if len(removed) != 2 || !removedSet["dead-sess"] || !removedSet["unbound-sess"] {
		t.Fatalf("expected [dead-sess unbound-sess] to be removed, got %v", removed)
	}

	remaining, err := mgr.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(remaining) != 1 || remaining[0].ID != "alive-sess" {
		t.Fatalf("expected 1 remaining session (alive-sess), got %+v", remaining)
	}

	// Verify DeleteAll cleans everything
	count, err := mgr.DeleteAll()
	if err != nil {
		t.Fatalf("DeleteAll failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected DeleteAll to remove 1 file, got %d", count)
	}
}
