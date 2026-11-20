package launcher

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"guard/internal/core"
	"guard/internal/hub/policy"
	"guard/internal/hub/session"
	"guard/internal/i18n"
)

// Launcher spawns an agent command under an isolated guardrail session and manages its lifecycle.
type Launcher struct {
	sessionMgr *session.Manager
	stdout     io.Writer
	stderr     io.Writer
}

// New creates a new Launcher instance.
func New(sessionMgr *session.Manager, stdout, stderr io.Writer) *Launcher {
	if sessionMgr == nil {
		sessionMgr = session.NewManager("")
	}
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}
	return &Launcher{
		sessionMgr: sessionMgr,
		stdout:     stdout,
		stderr:     stderr,
	}
}

// Run performs stale session GC, prints the briefing banner, binds a new session to the spawned
// child process PID, and guarantees graceful session cleanup on exit. Returns the child exit code.
func (l *Launcher) Run(mode string, workspace string, cmdArgs []string) int {
	if len(cmdArgs) == 0 {
		fmt.Fprintln(l.stderr, i18n.T("launcher.err.no_cmd"))
		return 1
	}

	normalizedMode := core.NormalizeMode(mode)
	if normalizedMode == "" {
		normalizedMode = core.ModeDev
	}

	// Self-healing GC before launching a new session (FR-4.2)
	_, _ = l.sessionMgr.CleanStaleSessions()

	sessionID := generateSessionID()
	polRes := policy.LoadDetailed(workspace)

	// Bootstrap Antigravity Skill and PreToolUse hook before spawning the child process.
	homeDir, _ := os.UserHomeDir()
	skillPath, skillCreated, skillErr := EnsureAgentSkill(homeDir)
	if skillErr != nil {
		fmt.Fprintf(l.stderr, "⚠️  [Bootstrap] Skill registration warning: %v\n", skillErr)
	}

	var hookPath string
	var hookUpdated bool
	if filepath.Base(cmdArgs[0]) == "agy" {
		var hookErr error
		hookPath, hookUpdated, hookErr = EnsureWorkspaceHook(workspace)
		if hookErr != nil {
			fmt.Fprintf(l.stderr, "⚠️  [Bootstrap] Hook registration warning: %v\n", hookErr)
		}
	}

	PrintBanner(l.stdout, BannerInfo{
		Mode:           normalizedMode,
		SessionID:      sessionID,
		Workspace:      workspace,
		PolicySource:   polRes.Source,
		PolicyWarnings: polRes.Warnings,
		CmdName:        cmdArgs[0],
		SkillPath:      skillPath,
		SkillCreated:   skillCreated,
		HookPath:       hookPath,
		HookUpdated:    hookUpdated,
		HasHook:        HasWorkspaceHook(workspace),
	})

	// Set terminal tab/window title to reflect the active guard mode
	setTerminalTitle(l.stdout, fmt.Sprintf("[%s] %s (%s)", normalizedMode, filepath.Base(cmdArgs[0]), sessionID))

	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = l.stdout
	cmd.Stderr = l.stderr
	cmd.Env = mergeEnv(os.Environ(), map[string]string{
		"AGENT_MODE":       normalizedMode,
		"GUARD_SESSION_ID": sessionID,
	})

	if err := cmd.Start(); err != nil {
		fmt.Fprint(l.stderr, i18n.T("launcher.err.start_failed", cmdArgs[0], err))
		return 1
	}

	// Bind session file to the actual child process PID
	if err := l.sessionMgr.Create(&session.Session{
		ID:        sessionID,
		Mode:      normalizedMode,
		PID:       cmd.Process.Pid,
		Workspace: workspace,
	}); err != nil {
		fmt.Fprintf(l.stderr, "⚠️  [Session] Failed to persist session %s: %v\n", session.ShortID(sessionID), err)
	}
	defer func() {
		_ = l.sessionMgr.Delete(sessionID)
	}()

	// Forward termination signals to the child process so defer cleanup runs cleanly.
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	doneCh := make(chan struct{})
	go func() {
		for {
			select {
			case sig := <-sigCh:
				if cmd.Process != nil {
					_ = cmd.Process.Signal(sig)
				}
			case <-doneCh:
				return
			}
		}
	}()

	// Watch session file for mode changes made via 'guard top' and update terminal title dynamically
	go func() {
		lastMode := normalizedMode
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				sess, err := l.sessionMgr.Get(sessionID)
				if err == nil && sess.Mode != "" && sess.Mode != lastMode {
					lastMode = sess.Mode
					setTerminalTitle(l.stdout, fmt.Sprintf("[%s] %s (%s)", sess.Mode, filepath.Base(cmdArgs[0]), sessionID))
				}
			case <-doneCh:
				return
			}
		}
	}()

	err := cmd.Wait()
	close(doneCh)

	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		return 1
	}
	return 0
}

func generateSessionID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("pid%d", os.Getpid())
	}
	return hex.EncodeToString(b)
}

func mergeEnv(base []string, overrides map[string]string) []string {
	out := make([]string, 0, len(base)+len(overrides))
	for _, kv := range base {
		k, _, ok := strings.Cut(kv, "=")
		if ok {
			if _, replaced := overrides[k]; replaced {
				continue
			}
		}
		out = append(out, kv)
	}
	for k, v := range overrides {
		out = append(out, k+"="+v)
	}
	return out
}

func setTerminalTitle(w io.Writer, title string) {
	if isTerminalWriter(w) {
		// OSC 0 ; <title> BEL (sets terminal window and icon/tab title)
		fmt.Fprintf(w, "\033]0;%s\007", title)
	}
}

func isTerminalWriter(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok || f == nil {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}
