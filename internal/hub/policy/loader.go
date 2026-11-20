package policy

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"guard/internal/core"
)

const (
	// EnvPolicyPath is the environment variable name for specifying a custom policy file path.
	EnvPolicyPath = "GUARD_POLICY_PATH"
	// BuiltinSource indicates that the built-in fallback policy was loaded.
	BuiltinSource = "builtin"
	// PolicyFileName is the default policy file name.
	PolicyFileName = "guard-policy.yaml"
)

// LoadOptions configures candidate lookup directories for policy loading.
type LoadOptions struct {
	EnvPath       string
	CWD           string
	WorkspaceRoot string
	HomeDir       string
}

// LoadResult holds the resolved policy, its source path (or "builtin"),
// and any non-fatal warnings encountered while parsing candidate policy files.
type LoadResult struct {
	Policy   *Policy
	Source   string
	Warnings []string
}

// Load resolves and parses guard-policy.yaml using the default environment and workspace root.
// Returns the loaded policy and the source path (or "builtin" if fallback was used).
func Load(workspaceRoot string) (*Policy, string) {
	res := LoadDetailed(workspaceRoot)
	return res.Policy, res.Source
}

// LoadDetailed resolves and parses guard-policy.yaml and returns any parse warnings encountered.
func LoadDetailed(workspaceRoot string) LoadResult {
	cwd, _ := os.Getwd()
	home, _ := os.UserHomeDir()
	return LoadDetailedWithOptions(LoadOptions{
		EnvPath:       os.Getenv(EnvPolicyPath),
		CWD:           cwd,
		WorkspaceRoot: workspaceRoot,
		HomeDir:       home,
	})
}

// LoadWithOptions searches for guard-policy.yaml in strict priority order:
//  1. GUARD_POLICY_PATH (opts.EnvPath)
//  2. ./guard-policy.yaml (opts.CWD)
//  3. <workspaceRoot>/.agents/guard-policy.yaml (opts.WorkspaceRoot)
//  4. ~/.config/guard/guard-policy.yaml (opts.HomeDir)
//  5. Built-in fallback policy
func LoadWithOptions(opts LoadOptions) (*Policy, string) {
	res := LoadDetailedWithOptions(opts)
	return res.Policy, res.Source
}

// LoadDetailedWithOptions searches candidate policy files in priority order and collects
// warnings when an existing candidate file fails to parse or contains no rules.
func LoadDetailedWithOptions(opts LoadOptions) LoadResult {
	candidates := candidatePaths(opts)
	var warnings []string
	for _, path := range candidates {
		p, err := LoadFile(path)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				warnings = append(warnings, fmt.Sprintf("%s: %v", path, err))
			}
			continue
		}
		if p != nil && len(p.Rules) > 0 {
			return LoadResult{
				Policy:   p,
				Source:   path,
				Warnings: warnings,
			}
		}
		warnings = append(warnings, fmt.Sprintf("%s: no rules defined", path))
	}
	return LoadResult{
		Policy:   DefaultPolicy(),
		Source:   BuiltinSource,
		Warnings: warnings,
	}
}

// ResolveWorkspaceMode determines the fallback guardrail mode from environment variables
// (AGENT_MODE, AGENT_ROLE) or <workspaceRoot>/.agents/current_mode, defaulting to core.ModeDev.
func ResolveWorkspaceMode(workspaceRoot string) string {
	for _, key := range []string{"AGENT_MODE", "AGENT_ROLE"} {
		if m := core.NormalizeMode(os.Getenv(key)); m != "" {
			return m
		}
	}
	if strings.TrimSpace(workspaceRoot) != "" {
		data, err := os.ReadFile(filepath.Join(workspaceRoot, ".agents", "current_mode"))
		if err == nil {
			if m := core.NormalizeMode(string(data)); m != "" {
				return m
			}
		}
	}
	return core.ModeDev
}

// LoadFile reads and unmarshals a single YAML policy file.
func LoadFile(path string) (*Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p Policy
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func candidatePaths(opts LoadOptions) []string {
	var paths []string
	seen := make(map[string]bool)

	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" {
			return
		}
		cleaned := filepath.Clean(p)
		if !seen[cleaned] {
			seen[cleaned] = true
			paths = append(paths, cleaned)
		}
	}

	// 1. GUARD_POLICY_PATH environment variable
	add(opts.EnvPath)

	// 2. Current working directory: ./guard-policy.yaml
	if strings.TrimSpace(opts.CWD) != "" {
		add(filepath.Join(opts.CWD, PolicyFileName))
	}

	// 3. Workspace root: .agents/guard-policy.yaml
	if strings.TrimSpace(opts.WorkspaceRoot) != "" {
		add(filepath.Join(opts.WorkspaceRoot, ".agents", PolicyFileName))
	}

	// 4. User config directory: ~/.config/guard/guard-policy.yaml
	if strings.TrimSpace(opts.HomeDir) != "" {
		add(filepath.Join(opts.HomeDir, ".config", "guard", PolicyFileName))
	}

	return paths
}
