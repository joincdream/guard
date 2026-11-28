package tui_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"guard/internal/adapters/tui"
	"guard/internal/core"
	"guard/internal/hub/handoff"
	"guard/internal/hub/session"
	"guard/internal/i18n"
)

func execTUICmd(m tea.Model, cmd tea.Cmd, depth int) tea.Model {
	if cmd == nil || depth <= 0 {
		return m
	}
	msg := cmd()
	if msg == nil {
		return m
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, sub := range batch {
			m = execTUICmd(m, sub, depth-1)
		}
		return m
	}
	next, _ := m.Update(msg)
	return next
}

func pressRune(m tea.Model, r rune) tea.Model {
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	return execTUICmd(next, cmd, 2)
}

func pressKey(m tea.Model, kt tea.KeyType) tea.Model {
	next, cmd := m.Update(tea.KeyMsg{Type: kt})
	return execTUICmd(next, cmd, 2)
}

func setupHandoffTUI(t *testing.T) (*handoff.Manager, *session.Manager, string, tea.Model) {
	t.Helper()
	tmp := t.TempDir()
	handoffDir := filepath.Join(tmp, "handoff")
	sessionDir := filepath.Join(tmp, "sessions")
	ws := filepath.Join(tmp, "ws")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatalf("mkdir ws: %v", err)
	}

	hMgr, err := handoff.NewManager(handoffDir, sessionDir)
	if err != nil {
		t.Fatalf("handoff.NewManager: %v", err)
	}
	sMgr := session.NewManager(sessionDir)

	var m tea.Model = tui.NewModelWithHandoff(sMgr, hMgr, ws)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return hMgr, sMgr, ws, m
}

func TestContract02_SummaryModalPayloadTruncationAndFooter(t *testing.T) {
	prevLang := i18n.Lang()
	defer i18n.SetLang(prevLang)
	i18n.SetLang("en")

	hMgr, _, _, m := setupHandoffTUI(t)

	var lines []string
	for i := 1; i <= 15; i++ {
		lines = append(lines, fmt.Sprintf("Payload line %02d", i))
	}
	payload := strings.Join(lines, "\n")

	_, err := hMgr.Enqueue(handoff.HandoffItem{
		Type:    handoff.TypeTask,
		From:    handoff.HandoffNode{Mode: handoff.ModePlan},
		To:      handoff.HandoffNode{Mode: handoff.ModeDev},
		Summary: "Verify summary modal payload truncation and footer keys",
		Payload: payload,
	})
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	m = pressRune(m, 'r')
	view := m.View()

	if !strings.Contains(view, "Payload line 01") || !strings.Contains(view, "Payload line 12") {
		t.Errorf("expected first 12 payload lines in view\n%s", view)
	}
	if strings.Contains(view, "Payload line 13") {
		t.Errorf("expected line 13 to be truncated in summary modal\n%s", view)
	}
	if !strings.Contains(view, "... (+3 lines)") {
		t.Errorf("expected '... (+3 lines)' truncation marker in view\n%s", view)
	}

	for _, keyHint := range []string{"[Enter]", "[V]", "[X/C]", "[Esc]"} {
		if !strings.Contains(view, keyHint) {
			t.Errorf("expected footer key hint %q in summary modal\n%s", keyHint, view)
		}
	}
}

func TestContract02_SummaryModal_EscDismissVsXCRejectVsEnterApprove(t *testing.T) {
	prevLang := i18n.Lang()
	defer i18n.SetLang(prevLang)
	i18n.SetLang("en")

	hMgr, _, _, m := setupHandoffTUI(t)

	// 1. Esc dismisses without changing status and prevents auto-repopup on refresh
	t1, err := hMgr.Enqueue(handoff.HandoffItem{
		Type:    handoff.TypeReview,
		From:    handoff.HandoffNode{Mode: handoff.ModeDev},
		To:      handoff.HandoffNode{Mode: handoff.ModeTest},
		Summary: "Ticket 1 Esc Dismiss",
	})
	if err != nil {
		t.Fatalf("Enqueue t1: %v", err)
	}

	m = pressRune(m, 'r')
	if !strings.Contains(m.View(), "Ticket 1 Esc Dismiss") {
		t.Fatalf("expected t1 modal to open")
	}

	m = pressKey(m, tea.KeyEsc)
	got1, err := hMgr.Get(t1.ID)
	if err != nil {
		t.Fatalf("Get(t1): %v", err)
	}
	if got1.Status != handoff.StatusPending {
		t.Fatalf("expected t1 status %q after Esc, got %q", handoff.StatusPending, got1.Status)
	}
	if strings.Contains(m.View(), "Ticket 1 Esc Dismiss") {
		t.Fatalf("expected modal to close after Esc")
	}

	// Refresh ('r'): dismissed ticket must NOT re-popup automatically
	m = pressRune(m, 'r')
	if strings.Contains(m.View(), "Ticket 1 Esc Dismiss") {
		t.Fatalf("dismissed ticket re-popped up after refresh")
	}

	// 2. X / C keys reject with "rejected via TUI"
	for _, rejectKey := range []rune{'x', 'X', 'c', 'C'} {
		t.Run(fmt.Sprintf("reject_key_%c", rejectKey), func(t *testing.T) {
			hMgr2, _, _, m2 := setupHandoffTUI(t)
			tk, err := hMgr2.Enqueue(handoff.HandoffItem{
				Type:    handoff.TypeFix,
				From:    handoff.HandoffNode{Mode: handoff.ModeTest},
				To:      handoff.HandoffNode{Mode: handoff.ModeDev},
				Summary: fmt.Sprintf("Reject with %c", rejectKey),
			})
			if err != nil {
				t.Fatalf("Enqueue: %v", err)
			}

			m2 = pressRune(m2, 'r')
			if !strings.Contains(m2.View(), tk.Summary) {
				t.Fatalf("expected modal for %s", tk.Summary)
			}

			m2 = pressRune(m2, rejectKey)
			got, err := hMgr2.Get(tk.ID)
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if got.Status != handoff.StatusRejected {
				t.Fatalf("status after %c = %q, want %q", rejectKey, got.Status, handoff.StatusRejected)
			}
			if got.RejectReason != "rejected via TUI" {
				t.Errorf("RejectReason = %q, want %q", got.RejectReason, "rejected via TUI")
			}
		})
	}

	// 3. Enter approves
	tApprove, err := hMgr.Enqueue(handoff.HandoffItem{
		Type:    handoff.TypeTask,
		From:    handoff.HandoffNode{Mode: handoff.ModePlan},
		To:      handoff.HandoffNode{Mode: handoff.ModeDev},
		Summary: "Ticket Enter Approve",
	})
	if err != nil {
		t.Fatalf("Enqueue tApprove: %v", err)
	}
	m = pressRune(m, 'r')
	if !strings.Contains(m.View(), tApprove.Summary) {
		t.Fatalf("expected new pending ticket modal to open")
	}
	m = pressKey(m, tea.KeyEnter)
	gotApprove, err := hMgr.Get(tApprove.ID)
	if err != nil {
		t.Fatalf("Get(tApprove): %v", err)
	}
	if gotApprove.Status != handoff.StatusApproved {
		t.Fatalf("status after Enter = %q, want %q", gotApprove.Status, handoff.StatusApproved)
	}
}

func TestContract02_RefViewer_EmptyRefsAndMultiRefCycling(t *testing.T) {
	prevLang := i18n.Lang()
	defer i18n.SetLang(prevLang)
	i18n.SetLang("en")

	// Case 1: Empty References -> 'v' stays in summary modal (no ref viewer)
	t.Run("empty references shows toast and stays in summary", func(t *testing.T) {
		hMgr, _, _, m := setupHandoffTUI(t)
		_, err := hMgr.Enqueue(handoff.HandoffItem{
			Type:    handoff.TypeTask,
			From:    handoff.HandoffNode{Mode: handoff.ModePlan},
			To:      handoff.HandoffNode{Mode: handoff.ModeDev},
			Summary: "No refs ticket",
		})
		if err != nil {
			t.Fatalf("Enqueue: %v", err)
		}
		m = pressRune(m, 'r')
		m = pressRune(m, 'v')
		view := m.View()
		if !strings.Contains(view, "No refs ticket") {
			t.Errorf("expected to remain on summary modal when References is empty\n%s", view)
		}
		if strings.Contains(view, "[1/") {
			t.Errorf("unexpected ref viewer header when References is empty")
		}
	})

	// Case 2: Multiple References -> [1/3] cycling & return
	t.Run("multi references cycling and return", func(t *testing.T) {
		hMgr, _, ws, m := setupHandoffTUI(t)
		refs := []string{"doc/ref1.md", "doc/ref2.md", "doc/ref3.md"}
		if err := os.MkdirAll(filepath.Join(ws, "doc"), 0o755); err != nil {
			t.Fatalf("mkdir doc: %v", err)
		}
		for i, rel := range refs {
			content := fmt.Sprintf("# Document %d\nUniqueMarkerDoc%d\n", i+1, i+1)
			for line := 0; line < 40; line++ {
				content += fmt.Sprintf("Line %d of doc %d\n", line, i+1)
			}
			if err := os.WriteFile(filepath.Join(ws, rel), []byte(content), 0o644); err != nil {
				t.Fatalf("write %s: %v", rel, err)
			}
		}

		_, err := hMgr.Enqueue(handoff.HandoffItem{
			Type:       handoff.TypeTask,
			From:       handoff.HandoffNode{Mode: handoff.ModePlan},
			To:         handoff.HandoffNode{Mode: handoff.ModeDev},
			Summary:    "Multi refs ticket",
			References: refs,
		})
		if err != nil {
			t.Fatalf("Enqueue: %v", err)
		}

		m = pressRune(m, 'r')
		m = pressRune(m, 'V') // open ref viewer

		v1 := m.View()
		if !strings.Contains(v1, "[1/3]") || !strings.Contains(v1, "UniqueMarkerDoc1") {
			t.Fatalf("expected [1/3] and UniqueMarkerDoc1 in ref viewer\n%s", v1)
		}
		if !strings.Contains(v1, "[Tab / ←/→]") {
			t.Errorf("expected '[Tab / ←/→]' footer hint in multi-ref viewer\n%s", v1)
		}

		// Tab -> [2/3]
		m = pressKey(m, tea.KeyTab)
		if v2 := m.View(); !strings.Contains(v2, "[2/3]") || !strings.Contains(v2, "UniqueMarkerDoc2") {
			t.Fatalf("expected [2/3] after Tab\n%s", v2)
		}

		// Right -> [3/3]
		m = pressKey(m, tea.KeyRight)
		if v3 := m.View(); !strings.Contains(v3, "[3/3]") || !strings.Contains(v3, "UniqueMarkerDoc3") {
			t.Fatalf("expected [3/3] after Right\n%s", v3)
		}

		// 'l' wraps around -> [1/3]
		m = pressRune(m, 'l')
		if vWrap := m.View(); !strings.Contains(vWrap, "[1/3]") || !strings.Contains(vWrap, "UniqueMarkerDoc1") {
			t.Fatalf("expected [1/3] wrap-around after 'l'\n%s", vWrap)
		}

		// Shift+Tab wraps backward -> [3/3]
		m = pressKey(m, tea.KeyShiftTab)
		if vBack1 := m.View(); !strings.Contains(vBack1, "[3/3]") || !strings.Contains(vBack1, "UniqueMarkerDoc3") {
			t.Fatalf("expected [3/3] after Shift+Tab\n%s", vBack1)
		}

		// Left -> [2/3]
		m = pressKey(m, tea.KeyLeft)
		if vBack2 := m.View(); !strings.Contains(vBack2, "[2/3]") || !strings.Contains(vBack2, "UniqueMarkerDoc2") {
			t.Fatalf("expected [2/3] after Left\n%s", vBack2)
		}

		// 'h' -> [1/3]
		m = pressRune(m, 'h')
		if vBack3 := m.View(); !strings.Contains(vBack3, "[1/3]") || !strings.Contains(vBack3, "UniqueMarkerDoc1") {
			t.Fatalf("expected [1/3] after 'h'\n%s", vBack3)
		}

		// Scroll keys must work without panic
		for _, k := range []tea.KeyType{tea.KeyDown, tea.KeyUp, tea.KeyPgDown, tea.KeyPgUp} {
			m = pressKey(m, k)
		}
		m = pressRune(m, 'j')
		m = pressRune(m, 'k')

		// Esc returns to summary modal
		m = pressKey(m, tea.KeyEsc)
		if vSummary := m.View(); !strings.Contains(vSummary, "Multi refs ticket") || strings.Contains(vSummary, "[1/3]") {
			t.Fatalf("expected return to summary modal after Esc\n%s", vSummary)
		}

		// Re-open with 'v' and return with 'V'
		m = pressRune(m, 'v')
		if !strings.Contains(m.View(), "[1/3]") {
			t.Fatalf("expected ref viewer to open on 'v'")
		}
		m = pressRune(m, 'V')
		if !strings.Contains(m.View(), "Multi refs ticket") || strings.Contains(m.View(), "[1/3]") {
			t.Fatalf("expected return to summary modal after 'V'")
		}
	})
}

func TestContract02_SessionModeCycle_MKey(t *testing.T) {
	prevLang := i18n.Lang()
	defer i18n.SetLang(prevLang)
	i18n.SetLang("en")

	t.Run("no session selected handles m/M safely", func(t *testing.T) {
		_, _, _, m := setupHandoffTUI(t)
		m = pressRune(m, 'm')
		m = pressRune(m, 'M')
		if strings.TrimSpace(m.View()) == "" {
			t.Fatalf("View() empty after pressing m/M with no sessions")
		}
	})

	t.Run("cycles through policy ExtractModes including custom mode", func(t *testing.T) {
		_, sMgr, ws, m := setupHandoffTUI(t)
		policyYAML := `version: "1.0"
rules:
  - name: "dev-rule"
    mode: "DEV"
    type: "read"
    patterns: ["*_test.go"]
  - name: "review-rule"
    mode: "REVIEW"
    type: "write"
    patterns: ["internal/**"]
`
		agentsDir := filepath.Join(ws, ".agents")
		if err := os.MkdirAll(agentsDir, 0o755); err != nil {
			t.Fatalf("mkdir .agents: %v", err)
		}
		if err := os.WriteFile(filepath.Join(agentsDir, "guard-policy.yaml"), []byte(policyYAML), 0o644); err != nil {
			t.Fatalf("write .agents/guard-policy.yaml: %v", err)
		}

		if err := sMgr.Create(&session.Session{
			ID:        "sess-cycle-1",
			Mode:      core.ModeDev,
			PID:       os.Getpid(),
			Workspace: ws,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}); err != nil {
			t.Fatalf("Create session: %v", err)
		}

		m = pressRune(m, 'r')

		// ExtractModes order: DEV -> REVIEW -> ALL -> DEV
		for _, wantMode := range []string{"REVIEW", "ALL", "DEV"} {
			m = pressRune(m, 'm')
			s, err := sMgr.Get("sess-cycle-1")
			if err != nil {
				t.Fatalf("Get session: %v", err)
			}
			if s.Mode != wantMode {
				t.Fatalf("after 'm': Mode = %q, want %q", s.Mode, wantMode)
			}
		}

		// Uppercase 'M' also cycles
		m = pressRune(m, 'M')
		s, err := sMgr.Get("sess-cycle-1")
		if err != nil {
			t.Fatalf("Get session: %v", err)
		}
		if s.Mode != "REVIEW" {
			t.Fatalf("after 'M': Mode = %q, want %q", s.Mode, "REVIEW")
		}
	})
}

func TestContract02_EdgeCases_SummaryBoundariesAndRefErrors(t *testing.T) {
	prevLang := i18n.Lang()
	defer i18n.SetLang(prevLang)
	i18n.SetLang("en")

	t.Run("exactly 12 payload lines has no truncation marker", func(t *testing.T) {
		hMgr, _, _, m := setupHandoffTUI(t)
		var lines []string
		for i := 1; i <= 12; i++ {
			lines = append(lines, fmt.Sprintf("ExactLine%02d", i))
		}
		_, err := hMgr.Enqueue(handoff.HandoffItem{
			Type:    handoff.TypeTask,
			From:    handoff.HandoffNode{Mode: handoff.ModePlan},
			To:      handoff.HandoffNode{Mode: handoff.ModeDev},
			Summary: "Boundary 12 lines",
			Payload: strings.Join(lines, "\n"),
		})
		if err != nil {
			t.Fatalf("Enqueue: %v", err)
		}
		m = pressRune(m, 'r')
		view := m.View()
		if !strings.Contains(view, "ExactLine12") {
			t.Errorf("expected ExactLine12 in view\n%s", view)
		}
		if strings.Contains(view, "... (+") {
			t.Errorf("unexpected truncation marker for exactly 12 lines\n%s", view)
		}
	})

	t.Run("whitespace-only payload does not render Details section", func(t *testing.T) {
		hMgr, _, _, m := setupHandoffTUI(t)
		_, err := hMgr.Enqueue(handoff.HandoffItem{
			Type:    handoff.TypeTask,
			From:    handoff.HandoffNode{Mode: handoff.ModePlan},
			To:      handoff.HandoffNode{Mode: handoff.ModeDev},
			Summary: "Whitespace payload",
			Payload: "   \n\t  \n ",
		})
		if err != nil {
			t.Fatalf("Enqueue: %v", err)
		}
		m = pressRune(m, 'r')
		view := m.View()
		if strings.Contains(view, "Details") || strings.Contains(view, "상세 내용") {
			t.Errorf("expected no Details section for whitespace-only payload\n%s", view)
		}
	})

	t.Run("long single-line payload wraps and truncates on narrow width", func(t *testing.T) {
		hMgr, _, _, m := setupHandoffTUI(t)
		m, _ = m.Update(tea.WindowSizeMsg{Width: 40, Height: 25})
		longPayload := strings.Repeat("word ", 120)
		_, err := hMgr.Enqueue(handoff.HandoffItem{
			Type:    handoff.TypeTask,
			From:    handoff.HandoffNode{Mode: handoff.ModePlan},
			To:      handoff.HandoffNode{Mode: handoff.ModeDev},
			Summary: strings.Repeat("LongSummaryWord ", 10),
			Payload: longPayload,
		})
		if err != nil {
			t.Fatalf("Enqueue: %v", err)
		}
		m = pressRune(m, 'r')
		view := m.View()
		if !strings.Contains(view, "... (+") {
			t.Errorf("expected wrapped long payload to truncate over 12 lines on narrow terminal\n%s", view)
		}
	})

	t.Run("single reference omits [1/1] and Tab/arrow footer hint", func(t *testing.T) {
		hMgr, _, ws, m := setupHandoffTUI(t)
		if err := os.MkdirAll(filepath.Join(ws, "doc"), 0o755); err != nil {
			t.Fatalf("mkdir doc: %v", err)
		}
		if err := os.WriteFile(filepath.Join(ws, "doc", "only.md"), []byte("# Only Ref\nHello"), 0o644); err != nil {
			t.Fatalf("write only.md: %v", err)
		}
		_, err := hMgr.Enqueue(handoff.HandoffItem{
			Type:       handoff.TypeTask,
			From:       handoff.HandoffNode{Mode: handoff.ModePlan},
			To:         handoff.HandoffNode{Mode: handoff.ModeDev},
			Summary:    "Single ref ticket",
			References: []string{"doc/only.md"},
		})
		if err != nil {
			t.Fatalf("Enqueue: %v", err)
		}
		m = pressRune(m, 'r')
		m = pressRune(m, 'v')
		view := m.View()
		if !strings.Contains(view, "Only Ref") {
			t.Fatalf("expected Only Ref content in viewer\n%s", view)
		}
		if strings.Contains(view, "[1/1]") || strings.Contains(view, "[Tab / ←/→]") {
			t.Errorf("did not expect multi-ref indicator/footer for single reference\n%s", view)
		}
	})

	t.Run("missing or outside-workspace reference does not panic", func(t *testing.T) {
		hMgr, _, _, m := setupHandoffTUI(t)
		_, err := hMgr.Enqueue(handoff.HandoffItem{
			Type:       handoff.TypeTask,
			From:       handoff.HandoffNode{Mode: handoff.ModePlan},
			To:         handoff.HandoffNode{Mode: handoff.ModeDev},
			Summary:    "Invalid ref path ticket",
			References: []string{"../outside_secret.txt", "doc/missing.md"},
		})
		if err != nil {
			t.Fatalf("Enqueue: %v", err)
		}
		m = pressRune(m, 'r')
		m = pressRune(m, 'v')
		if strings.TrimSpace(m.View()) == "" {
			t.Fatalf("View() empty after opening invalid ref")
		}
		m = pressKey(m, tea.KeyTab)
		if strings.TrimSpace(m.View()) == "" {
			t.Fatalf("View() empty after switching to missing ref")
		}
	})
}

func TestContract02_EdgeCases_OverlayIsolationAndModeCycleFallback(t *testing.T) {
	prevLang := i18n.Lang()
	defer i18n.SetLang(prevLang)
	i18n.SetLang("en")

	t.Run("m/M ignored while help overlay is open", func(t *testing.T) {
		_, sMgr, ws, m := setupHandoffTUI(t)
		if err := sMgr.Create(&session.Session{
			ID:        "sess-help-iso",
			Mode:      core.ModeDev,
			PID:       os.Getpid(),
			Workspace: ws,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}); err != nil {
			t.Fatalf("Create: %v", err)
		}
		m = pressRune(m, 'r')
		m = pressRune(m, '?') // open help
		m = pressRune(m, 'm')
		m = pressRune(m, 'M')

		s, err := sMgr.Get("sess-help-iso")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if s.Mode != core.ModeDev {
			t.Fatalf("expected Mode to remain %q while help overlay is open, got %q", core.ModeDev, s.Mode)
		}
	})

	t.Run("unknown session mode cycles safely to first policy mode", func(t *testing.T) {
		_, sMgr, ws, m := setupHandoffTUI(t)
		if err := sMgr.Create(&session.Session{
			ID:        "sess-unknown-mode",
			Mode:      "LEGACY_UNKNOWN",
			PID:       os.Getpid(),
			Workspace: ws,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}); err != nil {
			t.Fatalf("Create: %v", err)
		}
		m = pressRune(m, 'r')
		m = pressRune(m, 'm')

		s, err := sMgr.Get("sess-unknown-mode")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if s.Mode == "LEGACY_UNKNOWN" || s.Mode == "" {
			t.Fatalf("expected m key to cycle LEGACY_UNKNOWN to a valid policy mode, got %q", s.Mode)
		}
	})
}
