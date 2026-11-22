package setup

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"guard/internal/hub/policy"
	"guard/internal/hub/session"
)

type CheckSeverity string

const (
	SeverityOK    CheckSeverity = "OK"
	SeverityWarn  CheckSeverity = "WARN"
	SeverityError CheckSeverity = "ERROR"
	SeverityFixed CheckSeverity = "FIXED"
)

const (
	CodeBinaryPath   = "DOC-001"
	CodeHooksJSON    = "DOC-002"
	CodePolicyYAML   = "DOC-003"
	CodeWorkflowRule = "DOC-004"
	CodeSessionStore = "DOC-005"
	CodeSymlinkPath  = "DOC-006"
)

type DoctorOptions struct {
	WorkspaceRoot string
	SessionMgr    *session.Manager
	Fix           bool
	LookPath      func(file string) (string, error)
}

type CheckResult struct {
	Code     string        `json:"code"`
	Name     string        `json:"name"`
	Severity CheckSeverity `json:"severity"`
	Message  string        `json:"message"`
	Fixable  bool          `json:"fixable"`
	Fixed    bool          `json:"fixed,omitempty"`
}

type DoctorReport struct {
	WorkspaceRoot string        `json:"workspace_root"`
	Checks        []CheckResult `json:"checks"`
	ErrorCount    int           `json:"error_count"`
	WarnCount     int           `json:"warn_count"`
	FixedCount    int           `json:"fixed_count"`
}

// ExitCode returns 1 if any ERROR check exists, otherwise 0.
func (r DoctorReport) ExitCode() int {
	if r.ErrorCount > 0 {
		return 1
	}
	return 0
}

// RunDoctor executes checks DOC-001 through DOC-006 in order and optionally fixes fixable issues.
func RunDoctor(opts DoctorOptions) DoctorReport {
	lookPath := opts.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	sessMgr := opts.SessionMgr
	if sessMgr == nil {
		sessMgr = session.NewManager("")
	}

	report := DoctorReport{
		WorkspaceRoot: opts.WorkspaceRoot,
		Checks:        make([]CheckResult, 0, 6),
	}

	checks := []CheckResult{
		checkBinaryPath(lookPath),
		checkHooksJSON(opts.WorkspaceRoot, opts.Fix),
		checkPolicyYAML(opts.WorkspaceRoot, opts.Fix),
		checkWorkflowRule(opts.WorkspaceRoot, opts.Fix),
		checkSessionStore(sessMgr, opts.Fix),
		checkSymlinkPath(opts.WorkspaceRoot),
	}

	for _, c := range checks {
		report.Checks = append(report.Checks, c)
		switch c.Severity {
		case SeverityError:
			report.ErrorCount++
		case SeverityWarn:
			report.WarnCount++
		case SeverityFixed:
			report.FixedCount++
		}
	}

	return report
}

func checkBinaryPath(lookPath func(string) (string, error)) CheckResult {
	res := CheckResult{
		Code:    CodeBinaryPath,
		Name:    "guard binary in PATH",
		Fixable: false,
	}
	binPath, err := lookPath("guard")
	if err != nil {
		res.Severity = SeverityError
		res.Message = fmt.Sprintf("guard binary not found in PATH (%v); install with 'make install' or add it to PATH", err)
		return res
	}
	res.Severity = SeverityOK
	res.Message = fmt.Sprintf("found at %s", binPath)
	return res
}

func checkHooksJSON(workspaceRoot string, fix bool) CheckResult {
	res := CheckResult{
		Code: CodeHooksJSON,
		Name: ".agents/hooks.json",
	}
	exists, validJSON, hasHook, err := InspectWorkspaceHook(workspaceRoot)
	if err != nil {
		res.Severity = SeverityError
		res.Fixable = false
		res.Message = fmt.Sprintf("failed to inspect .agents/hooks.json: %v", err)
		return res
	}
	if exists && !validJSON {
		res.Severity = SeverityError
		res.Fixable = false
		res.Message = ".agents/hooks.json contains invalid JSON (manual fix required to avoid data loss)"
		return res
	}
	if exists && validJSON && hasHook {
		res.Severity = SeverityOK
		res.Fixable = true
		res.Message = "PreToolUse 'guard hook' is registered"
		return res
	}

	res.Fixable = true
	if fix {
		if _, mergeErr := MergeWorkspaceHook(workspaceRoot, false); mergeErr != nil {
			res.Severity = SeverityError
			res.Message = fmt.Sprintf("failed to register guard hook: %v", mergeErr)
			return res
		}
		res.Severity = SeverityFixed
		res.Fixed = true
		res.Message = "registered PreToolUse 'guard hook' in .agents/hooks.json"
		return res
	}

	res.Severity = SeverityError
	if !exists {
		res.Message = ".agents/hooks.json is missing (run 'guard init' or 'guard doctor --fix')"
	} else {
		res.Message = ".agents/hooks.json does not register PreToolUse 'guard hook' (run 'guard doctor --fix')"
	}
	return res
}

func checkPolicyYAML(workspaceRoot string, fix bool) CheckResult {
	res := CheckResult{
		Code: CodePolicyYAML,
		Name: "guard-policy.yaml",
	}

	homeDir, _ := os.UserHomeDir()
	polRes := policy.LoadDetailedWithOptions(policy.LoadOptions{
		EnvPath:       os.Getenv(policy.EnvPolicyPath),
		CWD:           workspaceRoot,
		WorkspaceRoot: workspaceRoot,
		HomeDir:       homeDir,
	})
	if len(polRes.Warnings) > 0 {
		res.Severity = SeverityError
		res.Fixable = false
		res.Message = fmt.Sprintf("policy parse error: %s", strings.Join(polRes.Warnings, "; "))
		return res
	}

	if polRes.Source == policy.BuiltinSource {
		policyPath := filepath.Join(workspaceRoot, "guard-policy.yaml")
		if _, statErr := os.Stat(policyPath); statErr == nil {
			res.Severity = SeverityError
			res.Fixable = false
			res.Message = fmt.Sprintf("%s exists but could not be loaded; refusing to overwrite", policyPath)
			return res
		}
		res.Fixable = true
		if fix {
			preset, _ := DetectStack(workspaceRoot)
			yamlContent, err := RenderPolicyYAML(preset)
			if err != nil {
				res.Severity = SeverityError
				res.Message = fmt.Sprintf("failed to render preset %s: %v", preset, err)
				return res
			}
			if err := writeFileAtomic(policyPath, []byte(yamlContent), 0o644); err != nil {
				res.Severity = SeverityError
				res.Message = fmt.Sprintf("failed to write guard-policy.yaml: %v", err)
				return res
			}
			res.Severity = SeverityFixed
			res.Fixed = true
			res.Message = fmt.Sprintf("created guard-policy.yaml (preset: %s)", preset)
			return res
		}
		res.Severity = SeverityWarn
		res.Message = "no guard-policy.yaml found; using builtin fallback (run 'guard init' or 'guard doctor --fix')"
		return res
	}

	res.Severity = SeverityOK
	res.Fixable = false
	res.Message = fmt.Sprintf("loaded %s (%d rules)", polRes.Source, len(polRes.Policy.Rules))
	return res
}

func checkWorkflowRule(workspaceRoot string, fix bool) CheckResult {
	res := CheckResult{
		Code:    CodeWorkflowRule,
		Name:    ".agents/rules/guard-workflow.md",
		Fixable: true,
	}

	rulePath := filepath.Join(workspaceRoot, ".agents", "rules", "guard-workflow.md")
	data, err := os.ReadFile(rulePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if fix {
				if _, _, createErr := EnsureWorkflowRule(workspaceRoot); createErr != nil {
					res.Severity = SeverityError
					res.Message = fmt.Sprintf("failed to create guard-workflow.md: %v", createErr)
					return res
				}
				res.Severity = SeverityFixed
				res.Fixed = true
				res.Message = "created .agents/rules/guard-workflow.md with trigger: always_on"
				return res
			}
			res.Severity = SeverityWarn
			res.Message = ".agents/rules/guard-workflow.md is missing (run 'guard init' or 'guard doctor --fix')"
			return res
		}
		res.Severity = SeverityError
		res.Fixable = false
		res.Message = fmt.Sprintf("failed to read guard-workflow.md: %v", err)
		return res
	}

	content := string(data)
	if HasAlwaysOnFrontmatter(content) {
		res.Severity = SeverityOK
		res.Message = "present with trigger: always_on frontmatter"
		return res
	}

	if fix {
		updated := InjectAlwaysOnFrontmatter(content)
		if writeErr := writeFileAtomic(rulePath, []byte(updated), 0o644); writeErr != nil {
			res.Severity = SeverityError
			res.Message = fmt.Sprintf("failed to inject trigger: always_on: %v", writeErr)
			return res
		}
		res.Severity = SeverityFixed
		res.Fixed = true
		res.Message = "injected trigger: always_on frontmatter (existing content preserved)"
		return res
	}

	res.Severity = SeverityWarn
	res.Message = "missing 'trigger: always_on' YAML frontmatter (run 'guard doctor --fix')"
	return res
}

func checkSessionStore(sessMgr *session.Manager, fix bool) CheckResult {
	res := CheckResult{
		Code:    CodeSessionStore,
		Name:    "session store (~/.guard/sessions)",
		Fixable: true,
	}

	dir := sessMgr.Dir()
	dirCreated := false
	if _, err := os.Stat(dir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if fix {
				if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
					res.Severity = SeverityError
					res.Message = fmt.Sprintf("failed to create session dir %s: %v", dir, mkErr)
					return res
				}
				dirCreated = true
			} else {
				res.Severity = SeverityWarn
				res.Message = fmt.Sprintf("session directory %s does not exist yet (run 'guard doctor --fix')", dir)
				return res
			}
		} else {
			res.Severity = SeverityError
			res.Fixable = false
			res.Message = fmt.Sprintf("cannot access session dir %s: %v", dir, err)
			return res
		}
	}

	if err := checkDirWritable(dir); err != nil {
		res.Severity = SeverityError
		res.Fixable = false
		res.Message = fmt.Sprintf("session dir %s is not writable: %v", dir, err)
		return res
	}

	sessions, err := sessMgr.List()
	if err != nil {
		res.Severity = SeverityError
		res.Fixable = false
		res.Message = fmt.Sprintf("failed to list sessions in %s: %v", dir, err)
		return res
	}

	staleCount := 0
	for _, s := range sessions {
		if s.PID <= 0 || !session.IsProcessAlive(s.PID) {
			staleCount++
		}
	}

	if staleCount > 0 {
		if fix {
			removed, cleanErr := sessMgr.CleanStaleSessions()
			if cleanErr != nil {
				res.Severity = SeverityError
				res.Message = fmt.Sprintf("failed to clean stale sessions: %v", cleanErr)
				return res
			}
			res.Severity = SeverityFixed
			res.Fixed = true
			res.Message = fmt.Sprintf("cleaned %d stale session(s) in %s", len(removed), dir)
			return res
		}
		res.Severity = SeverityWarn
		res.Message = fmt.Sprintf("found %d stale session(s) in %s (run 'guard doctor --fix' or 'guard clean')", staleCount, dir)
		return res
	}

	if dirCreated {
		res.Severity = SeverityFixed
		res.Fixed = true
		res.Message = fmt.Sprintf("created writable session dir %s", dir)
		return res
	}

	res.Severity = SeverityOK
	res.Message = fmt.Sprintf("writable (%s, %d active session(s))", dir, len(sessions))
	return res
}

func checkDirWritable(dir string) error {
	f, err := os.CreateTemp(dir, ".guard-doctor-*")
	if err != nil {
		return err
	}
	name := f.Name()
	if err := f.Close(); err != nil {
		_ = os.Remove(name)
		return err
	}
	return os.Remove(name)
}

func checkSymlinkPath(workspaceRoot string) CheckResult {
	res := CheckResult{
		Code:    CodeSymlinkPath,
		Name:    "workspace symlink consistency",
		Fixable: false,
	}

	if strings.TrimSpace(workspaceRoot) == "" {
		res.Severity = SeverityWarn
		res.Message = "workspace root is empty"
		return res
	}

	cleanRoot := filepath.Clean(workspaceRoot)
	resolved, err := filepath.EvalSymlinks(cleanRoot)
	if err != nil {
		res.Severity = SeverityWarn
		res.Message = fmt.Sprintf("failed to resolve symlinks for %s: %v", cleanRoot, err)
		return res
	}
	resolved = filepath.Clean(resolved)
	if cleanRoot != resolved {
		res.Severity = SeverityWarn
		res.Message = fmt.Sprintf("workspace path uses symlink (%s -> %s); consider using canonical path", cleanRoot, resolved)
		return res
	}

	res.Severity = SeverityOK
	res.Message = fmt.Sprintf("canonical path (%s)", resolved)
	return res
}
