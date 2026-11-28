package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"guard/internal/adapters/hook"
	"guard/internal/adapters/launcher"
	"guard/internal/adapters/tui"
	"guard/internal/core"
	"guard/internal/hub/handoff"
	"guard/internal/hub/policy"
	"guard/internal/hub/session"
	"guard/internal/i18n"
)

type cliApp struct {
	sessMgr       *session.Manager
	handoffMgr    *handoff.Manager
	workspaceRoot string
	stdin         io.Reader
	stdout        io.Writer
	stderr        io.Writer
	stdinPiped    bool
}

func main() {
	i18n.Init()
	app := &cliApp{
		sessMgr:       session.NewManager(""),
		workspaceRoot: findWorkspaceRoot(),
		stdin:         os.Stdin,
		stdout:        os.Stdout,
		stderr:        os.Stderr,
		stdinPiped:    isStdinPiped(),
	}
	os.Exit(app.run(os.Args[1:]))
}

func (a *cliApp) run(args []string) int {
	if len(args) == 0 {
		if a.stdinPiped {
			return a.runHook()
		}
		a.printHelp()
		return 0
	}

	cmd := strings.ToLower(strings.TrimSpace(args[0]))

	switch cmd {
	case "init":
		return a.runInit(args[1:])

	case "doctor":
		return a.runDoctor(args[1:])

	case "hook":
		return a.runHook()

	case "handoff":
		return a.runHandoff(args[1:])

	case "dev", "test", "plan", "all":
		return a.runModeCommand(core.NormalizeMode(cmd), args[1:])

	case "run":
		if len(args) < 3 {
			fmt.Fprintln(a.stderr, i18n.T("cli.usage.run"))
			return 1
		}
		l := launcher.New(a.sessMgr, a.stdout, a.stderr)
		return l.Run(args[1], a.workspaceRoot, args[2:])

	case "status":
		a.printStatus()
		return 0

	case "clean":
		cleanAll := len(args) > 1 && (args[1] == "--all" || args[1] == "-a")
		return a.runClean(cleanAll)

	case "reset":
		if len(args) < 2 {
			fmt.Fprintln(a.stderr, i18n.T("cli.usage.reset"))
			return 1
		}
		if _, err := a.sessMgr.ResetViolations(args[1]); err != nil {
			fmt.Fprint(a.stderr, i18n.T("cli.reset.failed", args[1], err))
			return 1
		}
		fmt.Fprint(a.stdout, i18n.T("cli.reset.success", args[1]))
		return 0

	case "top", "ui":
		hMgr, err := a.ensureHandoffManager()
		if err != nil {
			fmt.Fprintf(a.stderr, "error: initialize handoff manager: %v\n", err)
			return 1
		}
		if err := tui.StartWithHandoff(a.sessMgr, hMgr, a.workspaceRoot); err != nil {
			fmt.Fprint(a.stderr, i18n.T("cli.tui.failed", err))
			return 1
		}
		return 0

	case "doc", "sample":
		a.printDoc()
		return 0

	case "help", "-h", "--help":
		a.printHelp()
		return 0

	default:
		// Check if cmd matches a custom mode defined in guard-policy.yaml.
		polRes := policy.LoadDetailed(a.workspaceRoot)
		if core.HasMode(polRes.Policy, cmd) {
			return a.runModeCommand(core.NormalizeMode(cmd), args[1:])
		}
		// If the first argument is an executable command (e.g. `guard agy`, `guard bash`),
		// launch it directly using the current workspace default mode.
		if _, err := exec.LookPath(args[0]); err == nil {
			defaultMode := policy.ResolveWorkspaceMode(a.workspaceRoot)
			l := launcher.New(a.sessMgr, a.stdout, a.stderr)
			return l.Run(defaultMode, a.workspaceRoot, args)
		}
		fmt.Fprint(a.stderr, i18n.T("cli.unknown_subcommand", cmd))
		a.printHelp()
		return 1
	}
}

func (a *cliApp) runModeCommand(mode string, rest []string) int {
	if len(rest) > 0 {
		// Zero-Config Launcher: guard <mode> <cmd...>
		l := launcher.New(a.sessMgr, a.stdout, a.stderr)
		return l.Run(mode, a.workspaceRoot, rest)
	}
	return a.switchMode(mode)
}

func (a *cliApp) runHook() int {
	h := hook.NewHandler(a.sessMgr, a.workspaceRoot, a.stdout, a.stderr)
	if err := h.Handle(a.stdin); err != nil {
		return 1
	}
	return 0
}

func (a *cliApp) switchMode(mode string) int {
	modeFile := filepath.Join(a.workspaceRoot, ".agents", "current_mode")
	if err := os.MkdirAll(filepath.Dir(modeFile), 0o755); err != nil {
		fmt.Fprint(a.stderr, i18n.T("cli.switch.mkdir_failed", err))
		return 1
	}
	if err := os.WriteFile(modeFile, []byte(mode+"\n"), 0o644); err != nil {
		fmt.Fprint(a.stderr, i18n.T("cli.switch.write_failed", err))
		return 1
	}

	// If inside an active guard session, update the session file as well.
	for _, envKey := range []string{"GUARD_SESSION_ID", "SESSION_ID", "CONVERSATION_ID"} {
		if sid := strings.TrimSpace(os.Getenv(envKey)); sid != "" {
			if _, err := a.sessMgr.UpdateMode(sid, mode); err == nil {
				fmt.Fprint(a.stdout, i18n.T("cli.switch.active_updated", sid))
			}
			break
		}
	}

	fmt.Fprint(a.stdout, i18n.T("cli.switch.success", mode))
	switch mode {
	case core.ModeDev:
		fmt.Fprintln(a.stdout, i18n.T("cli.switch.desc.dev"))
	case core.ModeTest:
		fmt.Fprintln(a.stdout, i18n.T("cli.switch.desc.test"))
	case core.ModePlan:
		fmt.Fprintln(a.stdout, i18n.T("cli.switch.desc.plan"))
	case core.ModeAll:
		fmt.Fprintln(a.stdout, i18n.T("cli.switch.desc.all"))
	default:
		fmt.Fprintf(a.stdout, "Custom policy mode: %s\n", mode)
	}
	return 0
}

func (a *cliApp) printStatus() {
	wsMode := policy.ResolveWorkspaceMode(a.workspaceRoot)
	polRes := policy.LoadDetailed(a.workspaceRoot)

	fmt.Fprint(a.stdout, i18n.T("cli.status.title"))
	fmt.Fprint(a.stdout, i18n.T("cli.status.workspace", a.workspaceRoot))
	fmt.Fprint(a.stdout, i18n.T("cli.status.default_mode", wsMode))
	fmt.Fprint(a.stdout, i18n.T("cli.status.policy_source", polRes.Source, len(polRes.Policy.Rules)))
	for _, w := range polRes.Warnings {
		fmt.Fprintf(a.stdout, "   ⚠️  [Policy Warning] %s\n", w)
	}
	fmt.Fprint(a.stdout, i18n.T("cli.status.session_dir", a.sessMgr.Dir()))

	sessions, _, err := a.sessMgr.CleanAndList()
	if err != nil {
		fmt.Fprint(a.stderr, i18n.T("cli.status.list_failed", err))
		return
	}

	if len(sessions) == 0 {
		fmt.Fprintln(a.stdout, i18n.T("cli.status.empty"))
		return
	}

	fmt.Fprintf(a.stdout, "%-10s %-6s %-8s %-6s %-10s %s\n", "ID", "MODE", "PID", "DENY", "UPDATED", "WORKSPACE")
	fmt.Fprintln(a.stdout, strings.Repeat("-", 72))
	for _, s := range sessions {
		pidStr := "-"
		if s.PID > 0 {
			pidStr = fmt.Sprintf("%d", s.PID)
		}
		fmt.Fprintf(a.stdout, "%-10s %-6s %-8s %-6d %-10s %s\n",
			session.ShortID(s.ID), s.Mode, pidStr, s.ViolationCount, i18n.RelativeTime(s.UpdatedAt), s.Workspace)
	}
}

func (a *cliApp) runClean(all bool) int {
	if all {
		count, err := a.sessMgr.DeleteAll()
		if err != nil {
			fmt.Fprint(a.stderr, i18n.T("cli.clean.all_failed", err))
			return 1
		}
		fmt.Fprint(a.stdout, i18n.T("cli.clean.all_success", count))
		return 0
	}

	removed, err := a.sessMgr.CleanStaleSessions()
	if err != nil {
		fmt.Fprint(a.stderr, i18n.T("cli.clean.stale_failed", err))
		return 1
	}
	fmt.Fprint(a.stdout, i18n.T("cli.clean.stale_success", len(removed)))
	return 0
}

func (a *cliApp) printHelp() {
	fmt.Fprintln(a.stdout, i18n.T("cli.help"))
}

func (a *cliApp) printDoc() {
	fmt.Fprint(a.stdout, PolicyDoc())
}

func findWorkspaceRoot() string {
	if ws := strings.TrimSpace(os.Getenv("WORKSPACE_ROOT")); ws != "" {
		return resolveCanonicalPath(ws)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, ".agents")); err == nil {
			return resolveCanonicalPath(dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return resolveCanonicalPath(cwd)
}

func resolveCanonicalPath(p string) string {
	cleaned := filepath.Clean(p)
	if resolved, err := filepath.EvalSymlinks(cleaned); err == nil {
		return filepath.Clean(resolved)
	}
	return cleaned
}

func isStdinPiped() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) == 0
}
