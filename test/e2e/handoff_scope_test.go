package e2e_test

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"guard/internal/hub/handoff"
	"guard/internal/hub/session"
)

// TC-SCOPE-01 ~ TC-SCOPE-07: 조건부 Scope Lock 매트릭스 전수 검증
func TestHandoffScope01To07_Matrix(t *testing.T) {
	initialScope := []string{"existing/keep.go"}

	cases := []struct {
		id              string
		handoffType     handoff.HandoffType
		targets         []string
		sessionMode     string // "DEV", "ALL", "TEST", "PLAN"
		toMode          string // "dev", "all", "test", "plan"
		wantScopeLocked bool
		wantScopePaths  []string
		wantHeader      string
		forbidSubstrs   []string
	}{
		{
			id:              "TC-SCOPE-01/fix_targets_DEV",
			handoffType:     handoff.TypeFix,
			targets:         []string{"pkg/a.go"},
			sessionMode:     "DEV",
			toMode:          handoff.ModeDev,
			wantScopeLocked: true,
			wantScopePaths:  []string{"pkg/a.go"},
			wantHeader:      "수정 허용 대상 (Scope Locked)",
		},
		{
			id:              "TC-SCOPE-02/task_targets_ALL",
			handoffType:     handoff.TypeTask,
			targets:         []string{"pkg/b.go", "pkg/c.go"},
			sessionMode:     "ALL",
			toMode:          handoff.ModeAll,
			wantScopeLocked: true,
			wantScopePaths:  []string{"pkg/b.go", "pkg/c.go"},
			wantHeader:      "수정 허용 대상 (Scope Locked)",
		},
		{
			id:              "TC-SCOPE-03/verify_targets_TEST",
			handoffType:     handoff.TypeVerify,
			targets:         []string{"pkg/a.go"},
			sessionMode:     "TEST",
			toMode:          handoff.ModeTest,
			wantScopeLocked: false,
			wantScopePaths:  initialScope,
			wantHeader:      "작업 대상",
			forbidSubstrs:   []string{"Scope Locked"},
		},
		{
			id:              "TC-SCOPE-04/review_targets_PLAN",
			handoffType:     handoff.TypeReview,
			targets:         []string{"doc/a.md"},
			sessionMode:     "PLAN",
			toMode:          handoff.ModePlan,
			wantScopeLocked: false,
			wantScopePaths:  initialScope,
			wantHeader:      "작업 대상",
			forbidSubstrs:   []string{"Scope Locked"},
		},
		{
			id:              "TC-SCOPE-05/fix_empty_targets_DEV",
			handoffType:     handoff.TypeFix,
			targets:         []string{},
			sessionMode:     "DEV",
			toMode:          handoff.ModeDev,
			wantScopeLocked: false,
			wantScopePaths:  initialScope,
			wantHeader:      "",
			forbidSubstrs:   []string{"작업 대상", "Scope Locked"},
		},
		{
			id:              "TC-SCOPE-06/fix_targets_TEST",
			handoffType:     handoff.TypeFix,
			targets:         []string{"pkg/a.go"},
			sessionMode:     "TEST",
			toMode:          handoff.ModeTest,
			wantScopeLocked: false,
			wantScopePaths:  initialScope,
			wantHeader:      "작업 대상",
			forbidSubstrs:   []string{"Scope Locked"},
		},
		{
			id:              "TC-SCOPE-07/task_targets_PLAN",
			handoffType:     handoff.TypeTask,
			targets:         []string{"pkg/a.go"},
			sessionMode:     "PLAN",
			toMode:          handoff.ModePlan,
			wantScopeLocked: false,
			wantScopePaths:  initialScope,
			wantHeader:      "작업 대상",
			forbidSubstrs:   []string{"Scope Locked"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			handoffMgr, _, sessionDir := newIsolatedHandoffManager(t)
			sessMgr := session.NewManager(sessionDir)
			ws := t.TempDir()

			sessID := "sess-scope-" + tc.toMode
			if err := sessMgr.Create(&session.Session{
				ID:                sessID,
				Mode:              tc.sessionMode,
				PID:               os.Getpid(),
				Workspace:         ws,
				AllowedWritePaths: append([]string(nil), initialScope...),
			}); err != nil {
				t.Fatalf("Create session failed: %v", err)
			}

			item, err := handoffMgr.Enqueue(handoff.HandoffItem{
				Type:    tc.handoffType,
				From:    handoff.HandoffNode{Mode: handoff.ModePlan},
				To:      handoff.HandoffNode{Mode: tc.toMode},
				Summary: "scope lock matrix test: " + tc.id,
				Targets: tc.targets,
			})
			if err != nil {
				t.Fatalf("Enqueue failed: %v", err)
			}

			// ShouldApplyScopeLock 직접 검증
			if got := handoff.ShouldApplyScopeLock(item, tc.sessionMode); got != tc.wantScopeLocked {
				t.Errorf("ShouldApplyScopeLock(mode=%s): want %v, got %v", tc.sessionMode, tc.wantScopeLocked, got)
			}

			// ApproveAndRoute 실행
			res, err := handoff.ApproveAndRoute(handoffMgr, sessMgr, ws, item.ID)
			if err != nil {
				t.Fatalf("ApproveAndRoute failed: %v", err)
			}
			if res.Buffered {
				t.Fatalf("expected active session delivery (Buffered=false), got Buffered=true")
			}
			if res.SessionID != sessID {
				t.Fatalf("expected SessionID=%q, got %q", sessID, res.SessionID)
			}
			if res.ScopeLocked != tc.wantScopeLocked {
				t.Fatalf("ApproveResult.ScopeLocked: want %v, got %v", tc.wantScopeLocked, res.ScopeLocked)
			}

			// 세션 AllowedWritePaths 검증
			sess, err := sessMgr.Get(sessID)
			if err != nil {
				t.Fatalf("sessMgr.Get failed: %v", err)
			}
			if !reflect.DeepEqual(sess.AllowedWritePaths, tc.wantScopePaths) {
				t.Fatalf("Session.AllowedWritePaths: want %v, got %v", tc.wantScopePaths, sess.AllowedWritePaths)
			}

			// .inbox Envelope 내용 검증
			rawInbox, err := os.ReadFile(res.InboxPath)
			if err != nil {
				t.Fatalf("ReadFile(%q) failed: %v", res.InboxPath, err)
			}
			envelope := string(rawInbox)
			if tc.wantHeader != "" && !strings.Contains(envelope, tc.wantHeader) {
				t.Errorf("envelope missing expected header %q\n--- envelope ---\n%s", tc.wantHeader, envelope)
			}
			for _, forbid := range tc.forbidSubstrs {
				if strings.Contains(envelope, forbid) {
					t.Errorf("envelope must NOT contain %q\n--- envelope ---\n%s", forbid, envelope)
				}
			}
		})
	}
}

// TC-SCOPE-08: FormatEnvelope 필수 필드 누락 방어
func TestHandoffScope08_FormatEnvelopeMissingRequiredFields(t *testing.T) {
	cases := []struct {
		name string
		item handoff.HandoffItem
	}{
		{
			name: "missing_ID",
			item: handoff.HandoffItem{
				ID:      "",
				Type:    handoff.TypeFix,
				Summary: "요약 있음",
			},
		},
		{
			name: "missing_Type",
			item: handoff.HandoffItem{
				ID:      "hnd-20261121-12345678",
				Type:    "",
				Summary: "요약 있음",
			},
		},
		{
			name: "missing_Summary",
			item: handoff.HandoffItem{
				ID:      "hnd-20261121-12345678",
				Type:    handoff.TypeFix,
				Summary: "",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, locked := range []bool{true, false} {
				_, err := handoff.FormatEnvelope(tc.item, locked)
				if !errors.Is(err, handoff.ErrInvalidItem) {
					t.Errorf("FormatEnvelope(scopeLocked=%v): expected ErrInvalidItem, got %v", locked, err)
				}
			}
		})
	}
}
