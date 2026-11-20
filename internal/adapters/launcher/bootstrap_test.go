package launcher

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureAgentSkill_CreatesOnceAndPreservesExisting(t *testing.T) {
	home := t.TempDir()

	// First call: should create SKILL.md
	skillPath, created, err := EnsureAgentSkill(home)
	if err != nil {
		t.Fatalf("EnsureAgentSkill first call failed: %v", err)
	}
	if !created {
		t.Fatalf("expected created=true on first call")
	}
	expectedPath := filepath.Join(home, ".gemini", "antigravity-cli", "skills", "guard", "SKILL.md")
	if skillPath != expectedPath {
		t.Fatalf("expected path %q, got %q", expectedPath, skillPath)
	}

	data, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("read generated SKILL.md: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "name: guard") || !strings.Contains(content, "guard doc") {
		t.Errorf("unexpected SKILL.md content: %s", content)
	}

	// Modify file to ensure second call preserves user customization
	custom := "# custom skill\n"
	if err := os.WriteFile(skillPath, []byte(custom), 0o644); err != nil {
		t.Fatalf("overwrite SKILL.md: %v", err)
	}

	// Second call: should not overwrite
	_, created2, err := EnsureAgentSkill(home)
	if err != nil {
		t.Fatalf("EnsureAgentSkill second call failed: %v", err)
	}
	if created2 {
		t.Fatalf("expected created=false on second call")
	}
	after, _ := os.ReadFile(skillPath)
	if string(after) != custom {
		t.Errorf("expected existing SKILL.md to be preserved, got %q", string(after))
	}
}

func TestHasWorkspaceHook(t *testing.T) {
	ws := t.TempDir()
	if HasWorkspaceHook(ws) {
		t.Fatalf("expected false when .agents/hooks.json does not exist")
	}
	hookPath := filepath.Join(ws, ".agents", "hooks.json")
	_ = os.MkdirAll(filepath.Dir(hookPath), 0o755)
	_ = os.WriteFile(hookPath, []byte(`{"PreToolUse":[{"command":"guard"}]}`), 0o644)
	if !HasWorkspaceHook(ws) {
		t.Fatalf("expected true when .agents/hooks.json references guard")
	}
}

func TestEnsureWorkspaceHook_PreservesExistingHooks(t *testing.T) {
	ws := t.TempDir()
	hookFile := filepath.Join(ws, ".agents", "hooks.json")
	_ = os.MkdirAll(filepath.Dir(hookFile), 0o755)
	_ = os.WriteFile(hookFile, []byte(`{"chatlog":{"Stop":[{"type":"command","command":"chatlog hook"}]}}`), 0o644)

	path, updated, err := EnsureWorkspaceHook(ws)
	if err != nil {
		t.Fatalf("EnsureWorkspaceHook failed: %v", err)
	}
	if !updated || path != hookFile {
		t.Fatalf("expected updated=true and path=%q, got updated=%v path=%q", hookFile, updated, path)
	}

	raw, _ := os.ReadFile(hookFile)
	content := string(raw)
	if !strings.Contains(content, `"chatlog"`) || !strings.Contains(content, `"guard"`) || !strings.Contains(content, `"PreToolUse"`) {
		t.Fatalf("expected merged hooks.json with chatlog and guard PreToolUse, got: %s", content)
	}

	// Second call should be a no-op
	_, updated2, err := EnsureWorkspaceHook(ws)
	if err != nil || updated2 {
		t.Fatalf("expected second call to be no-op, got updated=%v err=%v", updated2, err)
	}
}
