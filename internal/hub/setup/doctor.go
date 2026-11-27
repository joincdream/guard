package setup

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"guard/internal/hub/policy"
	"guard/internal/hub/session"
	"guard/internal/i18n"
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
		Name:    i18n.T("cli.doctor.name.doc001"),
		Fixable: false,
	}
	binPath, err := lookPath("guard")
	if err != nil {
		res.Severity = SeverityError
		res.Message = i18n.T("cli.doctor.msg.doc001_err", err)
		return res
	}
	res.Severity = SeverityOK
	res.Message = i18n.T("cli.doctor.msg.doc001_ok", binPath)
	return res
}

func checkHooksJSON(workspaceRoot string, fix bool) CheckResult {
	res := CheckResult{
		Code: CodeHooksJSON,
		Name: i18n.T("cli.doctor.name.doc002"),
	}
	exists, validJSON, hasHook, err := InspectWorkspaceHook(workspaceRoot)
	if err != nil {
		res.Severity = SeverityError
		res.Fixable = false
		res.Message = i18n.T("cli.doctor.msg.doc002_inspect_err", err)
		return res
	}
	if exists && !validJSON {
		res.Severity = SeverityError
		res.Fixable = false
		res.Message = i18n.T("cli.doctor.msg.doc002_invalid_json")
		return res
	}
	if exists && validJSON && hasHook {
		if dupKeys, dupErr := FindGuardPreToolUseHookKeys(workspaceRoot); dupErr == nil && len(dupKeys) > 1 {
			res.Severity = SeverityWarn
			res.Fixable = false
			res.Message = i18n.T("cli.doctor.msg.doc002_duplicate", strings.Join(dupKeys, ", "))
			return res
		}
		res.Severity = SeverityOK
		res.Fixable = true
		res.Message = i18n.T("cli.doctor.msg.doc002_ok")
		return res
	}

	res.Fixable = true
	if fix {
		if _, mergeErr := MergeWorkspaceHook(workspaceRoot, false); mergeErr != nil {
			res.Severity = SeverityError
			res.Message = i18n.T("cli.doctor.msg.doc002_fix_err", mergeErr)
			return res
		}
		res.Severity = SeverityFixed
		res.Fixed = true
		res.Message = i18n.T("cli.doctor.msg.doc002_fixed")
		return res
	}

	res.Severity = SeverityError
	if !exists {
		res.Message = i18n.T("cli.doctor.msg.doc002_missing")
	} else {
		res.Message = i18n.T("cli.doctor.msg.doc002_no_hook")
	}
	return res
}

func checkPolicyYAML(workspaceRoot string, fix bool) CheckResult {
	res := CheckResult{
		Code: CodePolicyYAML,
		Name: i18n.T("cli.doctor.name.doc003"),
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
		res.Message = i18n.T("cli.doctor.msg.doc003_parse_err", strings.Join(polRes.Warnings, "; "))
		return res
	}

	if polRes.Source == policy.BuiltinSource {
		policyPath := filepath.Join(workspaceRoot, "guard-policy.yaml")
		if _, statErr := os.Stat(policyPath); statErr == nil {
			res.Severity = SeverityError
			res.Fixable = false
			res.Message = i18n.T("cli.doctor.msg.doc003_exists_unloaded", policyPath)
			return res
		}
		res.Fixable = true
		if fix {
			preset, _ := DetectStack(workspaceRoot)
			yamlContent, err := RenderPolicyYAML(preset)
			if err != nil {
				res.Severity = SeverityError
				res.Message = i18n.T("cli.doctor.msg.doc003_render_err", preset, err)
				return res
			}
			if err := writeFileAtomic(policyPath, []byte(yamlContent), 0o644); err != nil {
				res.Severity = SeverityError
				res.Message = i18n.T("cli.doctor.msg.doc003_write_err", err)
				return res
			}
			res.Severity = SeverityFixed
			res.Fixed = true
			res.Message = i18n.T("cli.doctor.msg.doc003_fixed", preset)
			return res
		}
		res.Severity = SeverityWarn
		res.Message = i18n.T("cli.doctor.msg.doc003_builtin")
		return res
	}

	res.Severity = SeverityOK
	res.Fixable = false
	res.Message = i18n.T("cli.doctor.msg.doc003_ok", polRes.Source, len(polRes.Policy.Rules))
	return res
}

func checkWorkflowRule(workspaceRoot string, fix bool) CheckResult {
	res := CheckResult{
		Code:    CodeWorkflowRule,
		Name:    i18n.T("cli.doctor.name.doc004"),
		Fixable: true,
	}

	rulePath := filepath.Join(workspaceRoot, ".agents", "rules", "guard-workflow.md")
	data, err := os.ReadFile(rulePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if fix {
				if _, _, createErr := EnsureWorkflowRule(workspaceRoot); createErr != nil {
					res.Severity = SeverityError
					res.Message = i18n.T("cli.doctor.msg.doc004_create_err", createErr)
					return res
				}
				res.Severity = SeverityFixed
				res.Fixed = true
				res.Message = i18n.T("cli.doctor.msg.doc004_created")
				return res
			}
			res.Severity = SeverityWarn
			res.Message = i18n.T("cli.doctor.msg.doc004_missing")
			return res
		}
		res.Severity = SeverityError
		res.Fixable = false
		res.Message = i18n.T("cli.doctor.msg.doc004_read_err", err)
		return res
	}

	content := string(data)
	if HasAlwaysOnFrontmatter(content) {
		res.Severity = SeverityOK
		res.Message = i18n.T("cli.doctor.msg.doc004_ok")
		return res
	}

	if fix {
		updated := InjectAlwaysOnFrontmatter(content)
		if writeErr := writeFileAtomic(rulePath, []byte(updated), 0o644); writeErr != nil {
			res.Severity = SeverityError
			res.Message = i18n.T("cli.doctor.msg.doc004_inject_err", writeErr)
			return res
		}
		res.Severity = SeverityFixed
		res.Fixed = true
		res.Message = i18n.T("cli.doctor.msg.doc004_injected")
		return res
	}

	res.Severity = SeverityWarn
	res.Message = i18n.T("cli.doctor.msg.doc004_no_frontmatter")
	return res
}

func checkSessionStore(sessMgr *session.Manager, fix bool) CheckResult {
	res := CheckResult{
		Code:    CodeSessionStore,
		Name:    i18n.T("cli.doctor.name.doc005"),
		Fixable: true,
	}

	dir := sessMgr.Dir()
	dirCreated := false
	if _, err := os.Stat(dir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if fix {
				if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
					res.Severity = SeverityError
					res.Message = i18n.T("cli.doctor.msg.doc005_mkdir_err", dir, mkErr)
					return res
				}
				dirCreated = true
			} else {
				res.Severity = SeverityWarn
				res.Message = i18n.T("cli.doctor.msg.doc005_missing", dir)
				return res
			}
		} else {
			res.Severity = SeverityError
			res.Fixable = false
			res.Message = i18n.T("cli.doctor.msg.doc005_access_err", dir, err)
			return res
		}
	}

	if err := checkDirWritable(dir); err != nil {
		res.Severity = SeverityError
		res.Fixable = false
		res.Message = i18n.T("cli.doctor.msg.doc005_not_writable", dir, err)
		return res
	}

	sessions, err := sessMgr.List()
	if err != nil {
		res.Severity = SeverityError
		res.Fixable = false
		res.Message = i18n.T("cli.doctor.msg.doc005_list_err", dir, err)
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
				res.Message = i18n.T("cli.doctor.msg.doc005_clean_err", cleanErr)
				return res
			}
			res.Severity = SeverityFixed
			res.Fixed = true
			res.Message = i18n.T("cli.doctor.msg.doc005_cleaned", len(removed), dir)
			return res
		}
		res.Severity = SeverityWarn
		res.Message = i18n.T("cli.doctor.msg.doc005_stale", staleCount, dir)
		return res
	}

	if dirCreated {
		res.Severity = SeverityFixed
		res.Fixed = true
		res.Message = i18n.T("cli.doctor.msg.doc005_created", dir)
		return res
	}

	res.Severity = SeverityOK
	res.Message = i18n.T("cli.doctor.msg.doc005_ok", dir, len(sessions))
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
		Name:    i18n.T("cli.doctor.name.doc006"),
		Fixable: false,
	}

	if strings.TrimSpace(workspaceRoot) == "" {
		res.Severity = SeverityWarn
		res.Message = i18n.T("cli.doctor.msg.doc006_empty")
		return res
	}

	cleanRoot := filepath.Clean(workspaceRoot)
	resolved, err := filepath.EvalSymlinks(cleanRoot)
	if err != nil {
		res.Severity = SeverityWarn
		res.Message = i18n.T("cli.doctor.msg.doc006_resolve_err", cleanRoot, err)
		return res
	}
	resolved = filepath.Clean(resolved)
	if cleanRoot != resolved {
		res.Severity = SeverityWarn
		res.Message = i18n.T("cli.doctor.msg.doc006_mismatch", cleanRoot, resolved)
		return res
	}

	res.Severity = SeverityOK
	res.Message = i18n.T("cli.doctor.msg.doc006_ok", resolved)
	return res
}
