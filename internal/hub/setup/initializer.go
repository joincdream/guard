package setup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type InitOptions struct {
	WorkspaceRoot string
	Preset        string
	Mode          string
	DryRun        bool
	Force         bool
	NoPolicy      bool
	NoRules       bool
	NoHooks       bool
}

type StepResult struct {
	Target     string     `json:"target"`
	Status     StepStatus `json:"status"`
	Detail     string     `json:"detail,omitempty"`
	BackupPath string     `json:"backup_path,omitempty"`
}

type InitResult struct {
	WorkspaceRoot  string       `json:"workspace_root"`
	DetectedPreset string       `json:"detected_preset"`
	DetectedMarker string       `json:"detected_marker,omitempty"`
	DryRun         bool         `json:"dry_run"`
	Steps          []StepResult `json:"steps"`
}

// RunInit initializes guard configuration files in opts.WorkspaceRoot idempotently.
// Validation runs fail-fast before any filesystem mutation occurs.
func RunInit(opts InitOptions) (InitResult, error) {
	if strings.TrimSpace(opts.WorkspaceRoot) == "" {
		return InitResult{}, errors.New("workspace root is required")
	}
	if err := ValidatePreset(opts.Preset); err != nil {
		return InitResult{}, err
	}
	mode, err := normalizeInitMode(opts.Mode)
	if err != nil {
		return InitResult{}, err
	}

	preset := strings.ToLower(strings.TrimSpace(opts.Preset))
	marker := ""
	if preset == "" || preset == PresetAuto {
		preset, marker = DetectStack(opts.WorkspaceRoot)
	}

	res := InitResult{
		WorkspaceRoot:  opts.WorkspaceRoot,
		DetectedPreset: preset,
		DetectedMarker: marker,
		DryRun:         opts.DryRun,
	}

	// 1. .agents/hooks.json
	if !opts.NoHooks {
		hookRes, err := MergeWorkspaceHook(opts.WorkspaceRoot, opts.DryRun)
		if err != nil {
			return res, err
		}
		detail := ""
		if len(hookRes.PreservedKeys) > 0 {
			detail = "preserved: " + strings.Join(hookRes.PreservedKeys, ", ")
		}
		res.Steps = append(res.Steps, StepResult{
			Target: ".agents/hooks.json",
			Status: hookRes.Status,
			Detail: detail,
		})
	}

	// 2. guard-policy.yaml
	if !opts.NoPolicy {
		policyContent, err := RenderPolicyYAML(preset)
		if err != nil {
			return res, err
		}
		policyPath := filepath.Join(opts.WorkspaceRoot, "guard-policy.yaml")
		step, err := writeManagedFile(policyPath, "guard-policy.yaml", []byte(policyContent), "preset: "+preset, opts.DryRun, opts.Force)
		if err != nil {
			return res, err
		}
		res.Steps = append(res.Steps, step)
	}

	// 3. .agents/rules/guard-workflow.md
	if !opts.NoRules {
		rulePath := filepath.Join(opts.WorkspaceRoot, ".agents", "rules", "guard-workflow.md")
		step, err := writeManagedFile(rulePath, ".agents/rules/guard-workflow.md", []byte(WorkflowRuleContent()), "trigger: always_on", opts.DryRun, opts.Force)
		if err != nil {
			return res, err
		}
		res.Steps = append(res.Steps, step)
	}

	// 4. .agents/current_mode
	modePath := filepath.Join(opts.WorkspaceRoot, ".agents", "current_mode")
	modeStep, err := writeCurrentModeFile(modePath, mode, opts.DryRun)
	if err != nil {
		return res, err
	}
	res.Steps = append(res.Steps, modeStep)

	return res, nil
}

func writeManagedFile(absPath, relTarget string, content []byte, detail string, dryRun, force bool) (StepResult, error) {
	step := StepResult{
		Target: relTarget,
		Detail: detail,
	}

	existingData, err := os.ReadFile(absPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return step, fmt.Errorf("read %s: %w", absPath, err)
	}
	exists := err == nil

	if exists && !force {
		step.Status = StatusSkipped
		return step, nil
	}

	if dryRun {
		step.Status = StatusPlanned
		if exists && force {
			step.BackupPath = relTarget + ".bak"
		}
		return step, nil
	}

	if exists && force {
		backupAbs := absPath + ".bak"
		if err := writeFileAtomic(backupAbs, existingData, 0o644); err != nil {
			return step, fmt.Errorf("backup %s: %w", absPath, err)
		}
		if err := writeFileAtomic(absPath, content, 0o644); err != nil {
			return step, err
		}
		step.Status = StatusBackedUp
		step.BackupPath = relTarget + ".bak"
		return step, nil
	}

	if err := writeFileAtomic(absPath, content, 0o644); err != nil {
		return step, err
	}
	step.Status = StatusCreated
	return step, nil
}

func writeCurrentModeFile(modePath, mode string, dryRun bool) (StepResult, error) {
	step := StepResult{
		Target: ".agents/current_mode",
	}
	existing, err := os.ReadFile(modePath)
	if err == nil {
		step.Status = StatusSkipped
		current := strings.TrimSpace(string(existing))
		if current != "" {
			step.Detail = current
		}
		return step, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return step, fmt.Errorf("read %s: %w", modePath, err)
	}

	step.Detail = mode
	if dryRun {
		step.Status = StatusPlanned
		return step, nil
	}

	if err := writeFileAtomic(modePath, []byte(mode+"\n"), 0o644); err != nil {
		return step, err
	}
	step.Status = StatusCreated
	return step, nil
}
