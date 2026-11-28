package handoff_test

import (
	"errors"
	"path/filepath"
	"testing"

	"guard/internal/hub/handoff"
)

func TestContract01_IsKnownMode(t *testing.T) {
	known := []string{"REVIEW", "SECURITY"}

	tests := []struct {
		name  string
		mode  string
		known []string
		want  bool
	}{
		{name: "builtin dev", mode: "dev", known: nil, want: true},
		{name: "builtin PLAN padded", mode: " PLAN ", known: nil, want: true},
		{name: "builtin test", mode: "Test", known: nil, want: true},
		{name: "builtin all", mode: "ALL", known: nil, want: true},
		{name: "empty", mode: "", known: known, want: false},
		{name: "whitespace", mode: "   ", known: known, want: false},
		{name: "custom lowercase", mode: "review", known: known, want: true},
		{name: "custom mixed case", mode: "Security", known: known, want: true},
		{name: "unknown mode", mode: "deploy", known: known, want: false},
		{name: "custom without known list", mode: "review", known: nil, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := handoff.IsKnownMode(tc.mode, tc.known); got != tc.want {
				t.Errorf("IsKnownMode(%q, %v) = %v, want %v", tc.mode, tc.known, got, tc.want)
			}
		})
	}
}

func TestContract01_ParseTargetWithModes(t *testing.T) {
	known := []string{"REVIEW", "SECURITY"}

	t.Run("empty returns ErrInvalidItem", func(t *testing.T) {
		_, err := handoff.ParseTargetWithModes("   ", known)
		if !errors.Is(err, handoff.ErrInvalidItem) {
			t.Fatalf("expected ErrInvalidItem, got %v", err)
		}
	})

	t.Run("builtin mode parsed as Mode", func(t *testing.T) {
		node, err := handoff.ParseTarget(" PLAN ")
		if err != nil {
			t.Fatalf("ParseTarget(PLAN) unexpected error: %v", err)
		}
		if node.Mode != "plan" || node.SessionID != "" {
			t.Fatalf("got %+v, want Mode=plan SessionID=''", node)
		}
	})

	t.Run("custom mode parsed as lowercase Mode when known", func(t *testing.T) {
		node, err := handoff.ParseTargetWithModes(" REVIEW ", known)
		if err != nil {
			t.Fatalf("ParseTargetWithModes(REVIEW) unexpected error: %v", err)
		}
		if node.Mode != "review" || node.SessionID != "" {
			t.Fatalf("got %+v, want Mode=review SessionID=''", node)
		}
	})

	t.Run("custom mode name treated as SessionID when not in knownModes", func(t *testing.T) {
		node, err := handoff.ParseTarget("review")
		if err != nil {
			t.Fatalf("ParseTarget(review) unexpected error: %v", err)
		}
		if node.Mode != "" || node.SessionID != "review" {
			t.Fatalf("got %+v, want Mode='' SessionID=review", node)
		}
	})

	t.Run("session ID parsed as SessionID even with knownModes", func(t *testing.T) {
		node, err := handoff.ParseTargetWithModes("sess-abc123", known)
		if err != nil {
			t.Fatalf("ParseTargetWithModes(sess-abc123) unexpected error: %v", err)
		}
		if node.Mode != "" || node.SessionID != "sess-abc123" {
			t.Fatalf("got %+v, want Mode='' SessionID=sess-abc123", node)
		}
	})

	t.Run("invalid session ID returns ErrInvalidItem", func(t *testing.T) {
		for _, raw := range []string{"../evil", "bad/id", "a b"} {
			_, err := handoff.ParseTargetWithModes(raw, known)
			if !errors.Is(err, handoff.ErrInvalidItem) {
				t.Errorf("ParseTargetWithModes(%q) expected ErrInvalidItem, got %v", raw, err)
			}
		}
	})
}

func TestContract01_ValidateWithModes(t *testing.T) {
	known := []string{"REVIEW"}

	item := handoff.HandoffItem{
		Type:    handoff.TypeReview,
		From:    handoff.HandoffNode{Mode: "dev"},
		To:      handoff.HandoffNode{Mode: "review"},
		Summary: "Review custom mode",
		Status:  handoff.StatusPending,
	}

	if err := item.Validate(); !errors.Is(err, handoff.ErrInvalidItem) {
		t.Fatalf("Validate() without knownModes expected ErrInvalidItem, got %v", err)
	}
	if err := item.ValidateWithModes(known); err != nil {
		t.Fatalf("ValidateWithModes(known) unexpected error: %v", err)
	}

	// Sender custom mode
	senderItem := handoff.HandoffItem{
		Type:    handoff.TypeTask,
		From:    handoff.HandoffNode{Mode: "review"},
		To:      handoff.HandoffNode{Mode: "dev"},
		Summary: "From custom mode",
	}
	if err := senderItem.From.ValidateAsSender(); !errors.Is(err, handoff.ErrInvalidItem) {
		t.Fatalf("ValidateAsSender() expected ErrInvalidItem, got %v", err)
	}
	if err := senderItem.From.ValidateAsSenderWithModes(known); err != nil {
		t.Fatalf("ValidateAsSenderWithModes(known) unexpected error: %v", err)
	}
	if err := item.To.ValidateAsTarget(); !errors.Is(err, handoff.ErrInvalidItem) {
		t.Fatalf("ValidateAsTarget() expected ErrInvalidItem, got %v", err)
	}
	if err := item.To.ValidateAsTargetWithModes(known); err != nil {
		t.Fatalf("ValidateAsTargetWithModes(known) unexpected error: %v", err)
	}
}

func TestContract01_ManagerSetKnownModes(t *testing.T) {
	tmp := t.TempDir()
	mgr, err := handoff.NewManager(filepath.Join(tmp, "handoff"), filepath.Join(tmp, "sessions"))
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	item := handoff.HandoffItem{
		Type:    handoff.TypeReview,
		From:    handoff.HandoffNode{Mode: "dev"},
		To:      handoff.HandoffNode{Mode: "review"},
		Summary: "Custom mode handoff ticket",
	}

	// Before SetKnownModes: fails
	if _, err := mgr.Enqueue(item); !errors.Is(err, handoff.ErrInvalidItem) {
		t.Fatalf("Enqueue before SetKnownModes expected ErrInvalidItem, got %v", err)
	}

	// After SetKnownModes: succeeds
	mgr.SetKnownModes([]string{"REVIEW"})
	created, err := mgr.Enqueue(item)
	if err != nil {
		t.Fatalf("Enqueue after SetKnownModes failed: %v", err)
	}
	if created.To.Mode != "review" {
		t.Fatalf("created.To.Mode = %q, want %q", created.To.Mode, "review")
	}

	got, err := mgr.Get(created.ID)
	if err != nil {
		t.Fatalf("Get(%s) failed: %v", created.ID, err)
	}
	if got.Summary != item.Summary {
		t.Errorf("Get summary = %q, want %q", got.Summary, item.Summary)
	}
}

func TestContract01_EdgeCases_HandoffModes(t *testing.T) {
	t.Run("SetKnownModes(nil) revokes custom modes", func(t *testing.T) {
		tmp := t.TempDir()
		mgr, err := handoff.NewManager(filepath.Join(tmp, "handoff"), filepath.Join(tmp, "sessions"))
		if err != nil {
			t.Fatalf("NewManager: %v", err)
		}

		item := handoff.HandoffItem{
			Type:    handoff.TypeReview,
			From:    handoff.HandoffNode{Mode: "dev"},
			To:      handoff.HandoffNode{Mode: "review"},
			Summary: "Custom mode ticket",
		}

		mgr.SetKnownModes([]string{"REVIEW"})
		created, err := mgr.Enqueue(item)
		if err != nil {
			t.Fatalf("Enqueue with REVIEW failed: %v", err)
		}

		got, err := mgr.Get(created.ID)
		if err != nil {
			t.Fatalf("Get(%s) failed: %v", created.ID, err)
		}
		if got.To.Mode != "review" {
			t.Fatalf("expected To.Mode='review', got %q", got.To.Mode)
		}

		// Reset known modes to nil -> custom mode rejected again
		mgr.SetKnownModes(nil)
		if _, err := mgr.Enqueue(item); !errors.Is(err, handoff.ErrInvalidItem) {
			t.Fatalf("expected ErrInvalidItem after SetKnownModes(nil), got %v", err)
		}
	})

	t.Run("Empty From/To and similar non-mode target", func(t *testing.T) {
		known := []string{"REVIEW", "", "  "}
		if handoff.IsKnownMode("", known) {
			t.Errorf("IsKnownMode('') should be false even if knownModes contains empty string")
		}

		// Similar string not in knownModes is parsed as SessionID
		node, err := handoff.ParseTargetWithModes("review-v2", known)
		if err != nil {
			t.Fatalf("ParseTargetWithModes(review-v2): %v", err)
		}
		if node.Mode != "" || node.SessionID != "review-v2" {
			t.Fatalf("got %+v, want SessionID=review-v2", node)
		}

		// Mixed case padded custom mode normalized to lowercase
		node2, err := handoff.ParseTargetWithModes("  ReViEw  ", known)
		if err != nil {
			t.Fatalf("ParseTargetWithModes(ReViEw): %v", err)
		}
		if node2.Mode != "review" || node2.SessionID != "" {
			t.Fatalf("got %+v, want Mode=review", node2)
		}

		// Empty From/To fails validation
		emptyItem := handoff.HandoffItem{
			Type:    handoff.TypeTask,
			From:    handoff.HandoffNode{},
			To:      handoff.HandoffNode{},
			Summary: "Empty nodes",
			Status:  handoff.StatusPending,
		}
		if err := emptyItem.ValidateWithModes(known); !errors.Is(err, handoff.ErrInvalidItem) {
			t.Fatalf("expected ErrInvalidItem for empty From/To, got %v", err)
		}
	})
}
