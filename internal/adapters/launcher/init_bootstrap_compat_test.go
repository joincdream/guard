package launcher_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"guard/internal/adapters/launcher"
	"guard/internal/hub/setup"
)

// TC-LCH-001: EnsureWorkspaceHook 생성 · 타 키 보존 병합 · 멱등성
func TestInitCompat_Launcher_EnsureWorkspaceHook(t *testing.T) {
	t.Run("create_on_empty_workspace", func(t *testing.T) {
		ws := t.TempDir()
		hookPath, updated, err := launcher.EnsureWorkspaceHook(ws)
		if err != nil {
			t.Fatalf("EnsureWorkspaceHook(empty): %v", err)
		}
		if !updated {
			t.Errorf("updated = false, want true")
		}
		if !launcher.HasWorkspaceHook(ws) {
			t.Errorf("HasWorkspaceHook = false after EnsureWorkspaceHook")
		}
		if hookPath == "" {
			t.Errorf("expected non-empty hookPath")
		}
	})

	t.Run("merge_preserves_other_keys_and_is_idempotent", func(t *testing.T) {
		ws := t.TempDir()
		hookPath := filepath.Join(ws, ".agents", "hooks.json")
		if err := os.MkdirAll(filepath.Dir(hookPath), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		orig := `{"obsidian-chatlog":{"PostToolUse":[{"command":"chatlog"}]}}`
		if err := os.WriteFile(hookPath, []byte(orig), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		_, updated, err := launcher.EnsureWorkspaceHook(ws)
		if err != nil {
			t.Fatalf("EnsureWorkspaceHook(merge): %v", err)
		}
		if !updated {
			t.Errorf("updated = false, want true on merge")
		}
		mergedBytes, err := os.ReadFile(hookPath)
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		if !strings.Contains(string(mergedBytes), "obsidian-chatlog") || !strings.Contains(string(mergedBytes), "guard hook") {
			t.Errorf("merged hooks.json missing expected keys:\n%s", string(mergedBytes))
		}

		// 재호출 시 멱등성 (updated == false)
		_, updated2, err := launcher.EnsureWorkspaceHook(ws)
		if err != nil {
			t.Fatalf("EnsureWorkspaceHook(idempotent): %v", err)
		}
		if updated2 {
			t.Errorf("updated2 = true, want false on already-registered hook")
		}
	})
}

// TC-LCH-002: EnsureWorkspaceHook 깨진 JSON 보호
func TestInitCompat_Launcher_EnsureWorkspaceHook_CorruptedJSONProtected(t *testing.T) {
	ws := t.TempDir()
	hookPath := filepath.Join(ws, ".agents", "hooks.json")
	if err := os.MkdirAll(filepath.Dir(hookPath), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	const broken = "{broken json"
	if err := os.WriteFile(hookPath, []byte(broken), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, updated, err := launcher.EnsureWorkspaceHook(ws)
	if err == nil {
		t.Errorf("EnsureWorkspaceHook(broken) expected error, got nil")
	}
	if updated {
		t.Errorf("updated = true on error, want false")
	}
	after, _ := os.ReadFile(hookPath)
	if string(after) != broken {
		t.Errorf("corrupted hooks.json modified! got %q, want %q", string(after), broken)
	}
}

// TC-LCH-003 & TC-LCH-004: EnsureWorkflowRule always_on 포함 생성 및 기존 파일 보존
func TestInitCompat_Launcher_EnsureWorkflowRule(t *testing.T) {
	ws := t.TempDir()

	// TC-LCH-003: 신규 생성 시 trigger: always_on 및 managed marker 포함
	rulePath, created, err := launcher.EnsureWorkflowRule(ws)
	if err != nil {
		t.Fatalf("EnsureWorkflowRule(new): %v", err)
	}
	if !created {
		t.Errorf("created = false, want true")
	}
	raw, err := os.ReadFile(rulePath)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", rulePath, err)
	}
	if !setup.HasAlwaysOnFrontmatter(string(raw)) {
		t.Errorf("launcher.EnsureWorkflowRule created file without trigger: always_on:\n%s", string(raw))
	}
	if !strings.Contains(string(raw), setup.WorkflowManagedMarker) {
		t.Errorf("launcher.EnsureWorkflowRule created file without marker %q", setup.WorkflowManagedMarker)
	}

	// TC-LCH-004: 기존 파일 보존
	const custom = "# User custom workflow rule\n"
	if err := os.WriteFile(rulePath, []byte(custom), 0o644); err != nil {
		t.Fatalf("WriteFile custom: %v", err)
	}
	_, created2, err := launcher.EnsureWorkflowRule(ws)
	if err != nil {
		t.Fatalf("EnsureWorkflowRule(existing): %v", err)
	}
	if created2 {
		t.Errorf("created2 = true, want false for existing rule")
	}
	after, _ := os.ReadFile(rulePath)
	if string(after) != custom {
		t.Errorf("launcher.EnsureWorkflowRule overwrote existing file")
	}
}
