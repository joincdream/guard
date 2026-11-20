package policy

import (
	"os"
	"path/filepath"
	"testing"

	"guard/internal/core"
)

func writePolicyYAML(t *testing.T, path string, ruleName string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	content := "version: \"1.0\"\nrules:\n  - name: \"" + ruleName + "\"\n    mode: \"DEV\"\n    type: \"read\"\n    patterns:\n      - \"*_test.go\"\n    message: \"denied by " + ruleName + "\"\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestLoadWithOptions_PriorityOrder(t *testing.T) {
	tmp := t.TempDir()

	envFile := filepath.Join(tmp, "env", "custom-policy.yaml")
	cwdDir := filepath.Join(tmp, "cwd")
	cwdFile := filepath.Join(cwdDir, "guard-policy.yaml")
	wsDir := filepath.Join(tmp, "workspace")
	wsFile := filepath.Join(wsDir, ".agents", "guard-policy.yaml")
	homeDir := filepath.Join(tmp, "home")
	homeFile := filepath.Join(homeDir, ".config", "guard", "guard-policy.yaml")

	// Start with only home config -> Priority 4
	writePolicyYAML(t, homeFile, "from-home")
	p, src := LoadWithOptions(LoadOptions{
		EnvPath:       envFile,
		CWD:           cwdDir,
		WorkspaceRoot: wsDir,
		HomeDir:       homeDir,
	})
	if src != homeFile || p.Rules[0].Name != "from-home" {
		t.Fatalf("priority 4 failed: got src=%q rule=%q", src, p.Rules[0].Name)
	}

	// Add workspace .agents/guard-policy.yaml -> Priority 3 overrides 4
	writePolicyYAML(t, wsFile, "from-workspace")
	p, src = LoadWithOptions(LoadOptions{
		EnvPath:       envFile,
		CWD:           cwdDir,
		WorkspaceRoot: wsDir,
		HomeDir:       homeDir,
	})
	if src != wsFile || p.Rules[0].Name != "from-workspace" {
		t.Fatalf("priority 3 failed: got src=%q rule=%q", src, p.Rules[0].Name)
	}

	// Add CWD ./guard-policy.yaml -> Priority 2 overrides 3
	writePolicyYAML(t, cwdFile, "from-cwd")
	p, src = LoadWithOptions(LoadOptions{
		EnvPath:       envFile,
		CWD:           cwdDir,
		WorkspaceRoot: wsDir,
		HomeDir:       homeDir,
	})
	if src != cwdFile || p.Rules[0].Name != "from-cwd" {
		t.Fatalf("priority 2 failed: got src=%q rule=%q", src, p.Rules[0].Name)
	}

	// Add GUARD_POLICY_PATH -> Priority 1 overrides 2
	writePolicyYAML(t, envFile, "from-env")
	p, src = LoadWithOptions(LoadOptions{
		EnvPath:       envFile,
		CWD:           cwdDir,
		WorkspaceRoot: wsDir,
		HomeDir:       homeDir,
	})
	if src != envFile || p.Rules[0].Name != "from-env" {
		t.Fatalf("priority 1 failed: got src=%q rule=%q", src, p.Rules[0].Name)
	}
}

func TestLoadWithOptions_FallbackAndCorruptedYAML(t *testing.T) {
	tmp := t.TempDir()

	// 1. No files at all -> built-in fallback
	p, src := LoadWithOptions(LoadOptions{
		CWD:           filepath.Join(tmp, "empty-cwd"),
		WorkspaceRoot: filepath.Join(tmp, "empty-ws"),
		HomeDir:       filepath.Join(tmp, "empty-home"),
	})
	if src != BuiltinSource {
		t.Fatalf("expected %q source, got %q", BuiltinSource, src)
	}
	if len(p.Rules) < 3 {
		t.Fatalf("expected built-in policy with >=3 rules, got %d", len(p.Rules))
	}

	// Verify built-in policy integrates cleanly with core.Evaluate
	dec := core.Evaluate("DEV", p, "read_file", "pkg/foo_test.go", "")
	if dec.Allowed {
		t.Fatalf("expected built-in policy to deny DEV read on *_test.go")
	}

	// 2. Corrupted YAML in priority 1 falls through to priority 2
	brokenEnv := filepath.Join(tmp, "broken.yaml")
	if err := os.WriteFile(brokenEnv, []byte(":::not valid yaml::: ["), 0o644); err != nil {
		t.Fatalf("write broken yaml: %v", err)
	}
	cwdDir := filepath.Join(tmp, "cwd")
	cwdFile := filepath.Join(cwdDir, "guard-policy.yaml")
	writePolicyYAML(t, cwdFile, "valid-cwd")

	p, src = LoadWithOptions(LoadOptions{
		EnvPath: brokenEnv,
		CWD:     cwdDir,
	})
	if src != cwdFile || p.Rules[0].Name != "valid-cwd" {
		t.Fatalf("expected fallthrough to valid cwd file, got src=%q rule=%q", src, p.Rules[0].Name)
	}
}
