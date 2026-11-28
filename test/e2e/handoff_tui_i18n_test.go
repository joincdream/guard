package e2e_test

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"guard/internal/adapters/tui"
	"guard/internal/hub/handoff"
	"guard/internal/hub/session"
	"guard/internal/i18n"
)

// helper: Bubble Tea Cmd를 재귀적으로 실행해 모델에 반영 (tick 루프 무한 반복 방지를 위해 depth 제한)
func applyTUICmd(m tea.Model, cmd tea.Cmd, maxDepth int) tea.Model {
	if cmd == nil || maxDepth <= 0 {
		return m
	}
	msg := cmd()
	if msg == nil {
		return m
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, sub := range batch {
			m = applyTUICmd(m, sub, maxDepth-1)
		}
		return m
	}
	nextModel, nextCmd := m.Update(msg)
	// tick Cmd는 time.After(500ms)를 기다리므로 1단계 갱신만 수행하고 재귀 중단
	_ = nextCmd
	return nextModel
}

func refreshTUIModel(m tea.Model) tea.Model {
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	return applyTUICmd(next, cmd, 2)
}

func sendRuneKey(m tea.Model, r rune) tea.Model {
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	return applyTUICmd(next, cmd, 2)
}

func sendSpecialKey(m tea.Model, kt tea.KeyType) tea.Model {
	next, cmd := m.Update(tea.KeyMsg{Type: kt})
	return applyTUICmd(next, cmd, 2)
}

// TC-TUI-01: 요약 모달 인터랙션 및 [H] 패널 전환
func TestHandoffTUI01_ModalAndPanelToggle(t *testing.T) {
	i18n.SetLang("en")
	defer i18n.SetLang("en")

	handoffMgr, _, sessionDir := newIsolatedHandoffManager(t)
	sessMgr := session.NewManager(sessionDir)
	ws := t.TempDir()

	// 1. 빈 상태에서 [H] 하단 패널 전환 검증
	var m tea.Model = tui.NewModelWithHandoff(sessMgr, handoffMgr, ws)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = refreshTUIModel(m)

	viewBeforeToggle := m.View()
	m = sendRuneKey(m, 'h')
	viewAfterToggle := m.View()
	if viewBeforeToggle == viewAfterToggle {
		t.Fatalf("expected [H] key to toggle bottom panel view, but View() did not change")
	}
	m = sendRuneKey(m, 'h')
	viewAfterSecondToggle := m.View()
	if viewAfterSecondToggle != viewBeforeToggle {
		t.Errorf("expected second [H] key to restore original bottom panel view")
	}

	// 2. pending 티켓 A 생성 -> 갱신 -> [Enter] 승인
	ticketA, err := handoffMgr.Enqueue(handoff.HandoffItem{
		Type:    handoff.TypeFix,
		From:    handoff.HandoffNode{Mode: handoff.ModeTest},
		To:      handoff.HandoffNode{Mode: handoff.ModeDev},
		Summary: "TUI Enter 승인 테스트",
	})
	if err != nil {
		t.Fatalf("Enqueue ticketA failed: %v", err)
	}

	m = refreshTUIModel(m)
	modalViewA := m.View()
	if !strings.Contains(modalViewA, ticketA.Summary) {
		t.Errorf("expected pending handoff modal to display summary %q\n--- view ---\n%s", ticketA.Summary, modalViewA)
	}

	m = sendSpecialKey(m, tea.KeyEnter)
	gotA, err := handoffMgr.Get(ticketA.ID)
	if err != nil {
		t.Fatalf("Get(ticketA) failed: %v", err)
	}
	if gotA.Status != handoff.StatusApproved {
		t.Fatalf("expected ticketA status %q after [Enter], got %q", handoff.StatusApproved, gotA.Status)
	}

	// 3. pending 티켓 B 생성 -> 갱신 -> [Esc] 보류(pending 유지) 후 [X] 반려
	ticketB, err := handoffMgr.Enqueue(handoff.HandoffItem{
		Type:    handoff.TypeReview,
		From:    handoff.HandoffNode{Mode: handoff.ModeDev},
		To:      handoff.HandoffNode{Mode: handoff.ModePlan},
		Summary: "TUI Esc 보류 및 X 반려 테스트",
	})
	if err != nil {
		t.Fatalf("Enqueue ticketB failed: %v", err)
	}

	m = refreshTUIModel(m)
	modalViewB := m.View()
	if !strings.Contains(modalViewB, ticketB.Summary) {
		t.Errorf("expected pending handoff modal to display summary %q\n--- view ---\n%s", ticketB.Summary, modalViewB)
	}

	m = sendSpecialKey(m, tea.KeyEsc)
	gotB, err := handoffMgr.Get(ticketB.ID)
	if err != nil {
		t.Fatalf("Get(ticketB) failed: %v", err)
	}
	if gotB.Status != handoff.StatusPending {
		t.Fatalf("expected ticketB status %q after [Esc] dismiss, got %q", handoff.StatusPending, gotB.Status)
	}

	// [Esc]로 보류된 후 [H] 또는 신규 티켓으로 모달 진입 대신, 새 티켓 C 생성 후 [X] 반려 검증
	ticketC, err := handoffMgr.Enqueue(handoff.HandoffItem{
		Type:    handoff.TypeReview,
		From:    handoff.HandoffNode{Mode: handoff.ModeDev},
		To:      handoff.HandoffNode{Mode: handoff.ModePlan},
		Summary: "TUI X 반려 테스트",
	})
	if err != nil {
		t.Fatalf("Enqueue ticketC failed: %v", err)
	}
	m = refreshTUIModel(m)
	m = sendRuneKey(m, 'x')
	gotC, err := handoffMgr.Get(ticketC.ID)
	if err != nil {
		t.Fatalf("Get(ticketC) failed: %v", err)
	}
	if gotC.Status != handoff.StatusRejected {
		t.Fatalf("expected ticketC status %q after [X], got %q", handoff.StatusRejected, gotC.Status)
	}
}

// TC-TUI-02: 오버레이 포커스 가로채기 방지 (Priority Invariant)
func TestHandoffTUI02_OverlayPriorityNoSteal(t *testing.T) {
	i18n.SetLang("en")
	defer i18n.SetLang("en")

	cases := []struct {
		name    string
		openKey tea.KeyMsg
	}{
		{
			name:    "help_overlay_question_mark",
			openKey: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}},
		},
		{
			name:    "event_detail_modal_space",
			openKey: tea.KeyMsg{Type: tea.KeySpace},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handoffMgr, _, sessionDir := newIsolatedHandoffManager(t)
			sessMgr := session.NewManager(sessionDir)
			ws := t.TempDir()

			if err := sessMgr.Create(&session.Session{
				ID:        "sess-tui-priority",
				Mode:      "DEV",
				PID:       os.Getpid(),
				Workspace: ws,
			}); err != nil {
				t.Fatalf("Create session failed: %v", err)
			}

			var m tea.Model = tui.NewModelWithHandoff(sessMgr, handoffMgr, ws)
			m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
			m = refreshTUIModel(m)

			// 도움말(?) 또는 이벤트 상세(Space) 오버레이 열기
			var cmd tea.Cmd
			m, cmd = m.Update(tc.openKey)
			m = applyTUICmd(m, cmd, 2)
			viewWithOverlay := m.View()

			// 백그라운드에서 새 pending 핸드오프 등록 후 갱신
			ticket, err := handoffMgr.Enqueue(handoff.HandoffItem{
				Type:    handoff.TypeFix,
				From:    handoff.HandoffNode{Mode: handoff.ModeTest},
				To:      handoff.HandoffNode{Mode: handoff.ModeDev},
				Summary: "포커스 가로채기 금지 테스트",
			})
			if err != nil {
				t.Fatalf("Enqueue failed: %v", err)
			}

			// Init()에서 반환되는 초기/틱 로딩 메시지 또는 r 갱신 전달
			m = refreshTUIModel(m)
			viewAfterEnqueue := m.View()

			if strings.Contains(viewAfterEnqueue, ticket.Summary) {
				t.Fatalf("handoff modal stole focus while %s was open!\n--- view ---\n%s", tc.name, viewAfterEnqueue)
			}
			if viewAfterEnqueue != viewWithOverlay {
				// 세션 activity 시간 등 미세 변화 외에 오버레이가 닫히지 않았는지 Esc 동작으로 교차 검증
				t.Logf("view changed slightly during refresh under %s; verifying via Esc behavior", tc.name)
			}

			// [Esc] 1회 입력 시 현재 오버레이가 닫혀야 하며, 핸드오프 티켓이 반려되면 안 됨
			m = sendSpecialKey(m, tea.KeyEsc)
			got, err := handoffMgr.Get(ticket.ID)
			if err != nil {
				t.Fatalf("Get(ticket) failed: %v", err)
			}
			if got.Status != handoff.StatusPending {
				t.Fatalf("Esc rejected handoff ticket (%q) instead of closing %s — focus was stolen", got.Status, tc.name)
			}
		})
	}
}

// TC-TUI-03: 256KB 초과 참조 문서 뷰어 방어
func TestHandoffTUI03_LargeRefDocViewerCap(t *testing.T) {
	i18n.SetLang("en")
	defer i18n.SetLang("en")

	handoffMgr, _, sessionDir := newIsolatedHandoffManager(t)
	sessMgr := session.NewManager(sessionDir)
	ws := t.TempDir()

	// 300KB (307,200 바이트) 참조 문서 생성
	const docSize = 300 * 1024
	largeContent := strings.Repeat("A", docSize)
	const refRelPath = "large_ref.md"
	if err := os.WriteFile(filepath.Join(ws, refRelPath), []byte(largeContent), 0o644); err != nil {
		t.Fatalf("WriteFile(large_ref.md) failed: %v", err)
	}

	if _, err := handoffMgr.Enqueue(handoff.HandoffItem{
		Type:       handoff.TypeReview,
		From:       handoff.HandoffNode{Mode: handoff.ModeDev},
		To:         handoff.HandoffNode{Mode: handoff.ModePlan},
		Summary:    "대용량 참조 문서 테스트",
		References: []string{refRelPath},
	}); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	var m tea.Model = tui.NewModelWithHandoff(sessMgr, handoffMgr, ws)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = refreshTUIModel(m)

	// [V] 키로 참조 문서 뷰어 진입
	m = sendRuneKey(m, 'v')
	viewerOut := m.View()

	const maxBytes = 256 * 1024
	if len(viewerOut) > maxBytes {
		t.Fatalf("rendered viewer output exceeded 256KB cap: len=%d > %d", len(viewerOut), maxBytes)
	}

	truncMsg := i18n.T("tui.handoff.ref_truncated")
	if truncMsg == "" || truncMsg == "tui.handoff.ref_truncated" {
		t.Fatalf("i18n key tui.handoff.ref_truncated not resolved: got %q", truncMsg)
	}
	if !strings.Contains(viewerOut, truncMsg) {
		t.Errorf("viewer output missing truncation notice %q\n--- view (first 800 chars) ---\n%s",
			truncMsg, viewerOut[:min(800, len(viewerOut))])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TC-TUI-04: ko.json / en.json 카탈로그 키 및 포맷 동사 100% 일치 + 로케일 감지
func TestHandoffTUI04_I18nCatalogAndLocale(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	i18nDir := filepath.Clean(filepath.Join(wd, "..", "..", "internal", "i18n"))

	var koPath, enPath string
	if err := filepath.WalkDir(i18nDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		switch d.Name() {
		case "ko.json":
			koPath = path
		case "en.json":
			enPath = path
		}
		return nil
	}); err != nil {
		t.Fatalf("WalkDir(%s) failed: %v", i18nDir, err)
	}
	if koPath == "" || enPath == "" {
		t.Fatalf("failed to locate ko.json (%q) and en.json (%q) under %s", koPath, enPath, i18nDir)
	}

	loadCatalog := func(p string) map[string]string {
		t.Helper()
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("ReadFile(%s) failed: %v", p, err)
		}
		var m map[string]string
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatalf("Unmarshal(%s) failed: %v", p, err)
		}
		return m
	}

	koCat := loadCatalog(koPath)
	enCat := loadCatalog(enPath)

	// 1. 필수 handoff 키 존재 확인
	requiredKeys := []string{
		"tui.footer.handoff",
		"tui.handoff.ref_truncated",
	}
	for _, k := range requiredKeys {
		if _, ok := koCat[k]; !ok {
			t.Errorf("ko.json missing required key %q", k)
		}
		if _, ok := enCat[k]; !ok {
			t.Errorf("en.json missing required key %q", k)
		}
	}

	// 2. 전체 키 집합 100% 일치 확인
	for k := range enCat {
		if _, ok := koCat[k]; !ok {
			t.Errorf("key %q exists in en.json but missing in ko.json", k)
		}
	}
	for k := range koCat {
		if _, ok := enCat[k]; !ok {
			t.Errorf("key %q exists in ko.json but missing in en.json", k)
		}
	}

	// 3. 동일 키에 대한 fmt 포맷 동사 개수 100% 일치 확인 (%% 이스케이프 제외)
	verbRe := regexp.MustCompile(`%(\[\d+\])?[#+\- 0]*(\*|\d+)?(\.(\*|\d+))?[vTtbcdoqxXUeEfFgGsp]`)
	countVerbs := func(s string) int {
		cleaned := strings.ReplaceAll(s, "%%", "")
		return len(verbRe.FindAllString(cleaned, -1))
	}
	for k, enVal := range enCat {
		koVal, ok := koCat[k]
		if !ok {
			continue
		}
		enCount := countVerbs(enVal)
		koCount := countVerbs(koVal)
		if enCount != koCount {
			t.Errorf("format verb count mismatch for key %q: en=%d (%q) vs ko=%d (%q)", k, enCount, enVal, koCount, koVal)
		}
	}

	// 4. DetectLocaleFromLookup 우선순위 (GUARD_LANG > LC_ALL > LC_MESSAGES > LANG > "en") 및 C/POSIX 폴백
	localeCases := []struct {
		name string
		env  map[string]string
		want string
	}{
		{
			name: "GUARD_LANG_beats_all",
			env:  map[string]string{"GUARD_LANG": "ko", "LC_ALL": "en_US.UTF-8", "LC_MESSAGES": "en_US.UTF-8", "LANG": "en_US.UTF-8"},
			want: "ko",
		},
		{
			name: "LC_ALL_beats_LC_MESSAGES_and_LANG",
			env:  map[string]string{"LC_ALL": "ko_KR.UTF-8", "LC_MESSAGES": "en_US.UTF-8", "LANG": "en_US.UTF-8"},
			want: "ko",
		},
		{
			name: "LC_MESSAGES_beats_LANG",
			env:  map[string]string{"LC_MESSAGES": "ko_KR.UTF-8", "LANG": "en_US.UTF-8"},
			want: "ko",
		},
		{
			name: "LANG_used_when_others_empty",
			env:  map[string]string{"LANG": "ko_KR.UTF-8"},
			want: "ko",
		},
		{
			name: "default_en_when_all_empty",
			env:  map[string]string{},
			want: "en",
		},
		{
			name: "C_fallback_to_en",
			env:  map[string]string{"LANG": "C"},
			want: "en",
		},
		{
			name: "POSIX_fallback_to_en",
			env:  map[string]string{"LC_ALL": "POSIX"},
			want: "en",
		},
		{
			name: "unsupported_locale_fallback_to_en",
			env:  map[string]string{"LANG": "fr_FR.UTF-8"},
			want: "en",
		},
	}

	for _, tc := range localeCases {
		t.Run("locale/"+tc.name, func(t *testing.T) {
			got := i18n.DetectLocaleFromLookup(func(k string) string {
				return tc.env[k]
			})
			if got != tc.want {
				t.Errorf("DetectLocaleFromLookup(%v): want %q, got %q", tc.env, tc.want, got)
			}
		})
	}
}
