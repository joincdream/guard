package main

import (
	"encoding/json"
	"flag"
	"fmt"

	"guard/internal/hub/setup"
	"guard/internal/i18n"
)

func (a *cliApp) runInit(args []string) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(a.stderr)

	preset := fs.String("preset", setup.PresetAuto, "policy preset (auto, go, node, python, generic)")
	mode := fs.String("mode", "DEV", "initial workspace mode (DEV, PLAN, TEST, ALL)")
	dryRun := fs.Bool("dry-run", false, "preview changes without writing files")
	force := fs.Bool("force", false, "backup existing managed files (.bak) and regenerate")
	noPolicy := fs.Bool("no-policy", false, "skip generating guard-policy.yaml")
	noRules := fs.Bool("no-rules", false, "skip generating .agents/rules/guard-workflow.md")
	noHooks := fs.Bool("no-hooks", false, "skip merging .agents/hooks.json")

	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() > 0 {
		fmt.Fprint(a.stderr, i18n.T("cli.init.unexpected_args", fs.Args()))
		return 1
	}

	res, err := setup.RunInit(setup.InitOptions{
		WorkspaceRoot: a.workspaceRoot,
		Preset:        *preset,
		Mode:          *mode,
		DryRun:        *dryRun,
		Force:         *force,
		NoPolicy:      *noPolicy,
		NoRules:       *noRules,
		NoHooks:       *noHooks,
	})
	if err != nil {
		fmt.Fprint(a.stderr, i18n.T("cli.init.failed", err))
		return 1
	}

	if res.DryRun {
		fmt.Fprint(a.stdout, i18n.T("cli.init.start_dry_run", res.WorkspaceRoot))
	} else {
		fmt.Fprint(a.stdout, i18n.T("cli.init.start", res.WorkspaceRoot))
	}

	stackLabel := res.DetectedPreset
	if res.DetectedMarker != "" {
		stackLabel = fmt.Sprintf("%s (%s)", res.DetectedPreset, res.DetectedMarker)
	}
	fmt.Fprint(a.stdout, i18n.T("cli.init.detected_stack", stackLabel))

	for _, step := range res.Steps {
		icon, statusLabel := formatStepStatus(step.Status)
		detail := step.Detail
		if step.BackupPath != "" {
			if detail != "" {
				detail = fmt.Sprintf("%s, backup: %s", detail, step.BackupPath)
			} else {
				detail = fmt.Sprintf("backup: %s", step.BackupPath)
			}
		}
		if detail != "" {
			fmt.Fprintf(a.stdout, "   %s %-34s %s (%s)\n", icon, step.Target, statusLabel, detail)
		} else {
			fmt.Fprintf(a.stdout, "   %s %-34s %s\n", icon, step.Target, statusLabel)
		}
	}

	if res.DryRun {
		fmt.Fprint(a.stdout, i18n.T("cli.init.done_dry_run"))
	} else {
		fmt.Fprint(a.stdout, i18n.T("cli.init.done"))
	}
	return 0
}

func formatStepStatus(status setup.StepStatus) (icon string, label string) {
	switch status {
	case setup.StatusCreated:
		return "✅", i18n.T("cli.init.status.created")
	case setup.StatusMerged:
		return "✅", i18n.T("cli.init.status.merged")
	case setup.StatusBackedUp:
		return "✅", i18n.T("cli.init.status.backed_up")
	case setup.StatusPlanned:
		return "📝", i18n.T("cli.init.status.planned")
	case setup.StatusSkipped:
		return "⏭️", i18n.T("cli.init.status.skipped")
	default:
		return "•", string(status)
	}
}

func (a *cliApp) runDoctor(args []string) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(a.stderr)

	fix := fs.Bool("fix", false, "automatically fix fixable issues")
	jsonOut := fs.Bool("json", false, "output diagnostic report as JSON")

	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() > 0 {
		fmt.Fprint(a.stderr, i18n.T("cli.doctor.unexpected_args", fs.Args()))
		return 1
	}

	report := setup.RunDoctor(setup.DoctorOptions{
		WorkspaceRoot: a.workspaceRoot,
		SessionMgr:    a.sessMgr,
		Fix:           *fix,
	})

	if *jsonOut {
		enc := json.NewEncoder(a.stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprint(a.stderr, i18n.T("cli.doctor.json_failed", err))
			return 1
		}
		return report.ExitCode()
	}

	fmt.Fprint(a.stdout, i18n.T("cli.doctor.title", report.WorkspaceRoot))
	for _, c := range report.Checks {
		icon := formatSeverityIcon(c.Severity)
		fmt.Fprintf(a.stdout, "   %s [%s] %-32s %s\n", icon, c.Code, c.Name, c.Message)
	}
	fmt.Fprint(a.stdout, i18n.T("cli.doctor.summary", report.ErrorCount, report.WarnCount, report.FixedCount))
	return report.ExitCode()
}

func formatSeverityIcon(sev setup.CheckSeverity) string {
	switch sev {
	case setup.SeverityOK:
		return "✅"
	case setup.SeverityWarn:
		return "⚠️"
	case setup.SeverityError:
		return "❌"
	case setup.SeverityFixed:
		return "🔧"
	default:
		return "•"
	}
}
