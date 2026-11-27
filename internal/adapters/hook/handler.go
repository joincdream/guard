package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"guard/internal/core"
	"guard/internal/hub/policy"
	"guard/internal/hub/session"
)

// Handler bridges Antigravity PreToolUse stdin/stdout JSON with Core Evaluator and Session/Policy Hubs.
type Handler struct {
	sessionMgr    *session.Manager
	workspaceRoot string
	stdout        io.Writer
	stderr        io.Writer
}

// NewHandler creates a Hook Adapter handler.
func NewHandler(sessionMgr *session.Manager, workspaceRoot string, stdout, stderr io.Writer) *Handler {
	if sessionMgr == nil {
		sessionMgr = session.NewManager("")
	}
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}
	return &Handler{
		sessionMgr:    sessionMgr,
		workspaceRoot: workspaceRoot,
		stdout:        stdout,
		stderr:        stderr,
	}
}

// Handle reads a PreToolUse JSON payload from r, evaluates guardrail policy, records the event,
// and writes the decision JSON to stdout.
func (h *Handler) Handle(r io.Reader) error {
	raw, err := io.ReadAll(r)
	if err != nil || len(strings.TrimSpace(string(raw))) == 0 {
		return h.writeDecision(Output{Decision: "allow"})
	}

	var input Input
	if err := json.Unmarshal(raw, &input); err != nil {
		// Fail-open on malformed JSON to avoid crashing the agent runtime (NFR-4).
		h.debugf("malformed hook JSON payload: %v", err)
		return h.writeDecision(Output{Decision: "allow"})
	}

	toolName := input.ExtractToolName()
	targetPath := input.ExtractTargetPath()
	cmdLine := input.ExtractCommandLine()

	effectiveWorkspace := h.workspaceRoot
	effectiveCWD, _ := os.Getwd()
	if payloadCWD := input.ExtractCWD(); payloadCWD != "" {
		effectiveCWD = payloadCWD
		effectiveWorkspace = payloadCWD
	}

	sessionID, mode, hasSession := h.resolveSessionAndMode(&input, effectiveWorkspace)

	homeDir, _ := os.UserHomeDir()
	polRes := policy.LoadDetailedWithOptions(policy.LoadOptions{
		EnvPath:       os.Getenv(policy.EnvPolicyPath),
		CWD:           effectiveCWD,
		WorkspaceRoot: effectiveWorkspace,
		HomeDir:       homeDir,
	})
	for _, w := range polRes.Warnings {
		h.debugf("policy warning: %s", w)
	}
	dec := core.EvaluateRequest(polRes.Policy, core.EvalRequest{
		Mode:        mode,
		ToolName:    toolName,
		TargetPath:  targetPath,
		CommandLine: cmdLine,
	})
	h.debugf("evaluated mode=%s source=%s tool=%s allowed=%v", mode, polRes.Source, toolName, dec.Allowed)

	// Record event into session hub only when a tracked session exists.
	if sessionID != "" && hasSession {
		target := targetPath
		if target == "" {
			target = cmdLine
		}
		decisionStr := "allow"
		if !dec.Allowed {
			decisionStr = "deny"
		}
		if _, err := h.sessionMgr.AppendEvent(sessionID, session.Event{
			Timestamp:  time.Now(),
			ToolCallID: input.ExtractToolCallID(),
			Decision:   decisionStr,
			Tool:       toolName,
			Target:     target,
			RuleName:   dec.RuleName,
			Reason:     dec.Reason,
		}); err != nil {
			h.debugf("append session event failed (%s): %v", sessionID, err)
		}
	}

	if !dec.Allowed {
		_, _ = fmt.Fprintln(h.stderr, dec.Reason)
		return h.writeDecision(Output{
			Decision: "deny",
			Reason:   dec.Reason,
		})
	}

	return h.writeDecision(Output{Decision: "allow"})
}

func (h *Handler) resolveSessionAndMode(input *Input, workspaceRoot string) (string, string, bool) {
	candidates := candidateSessionIDs(input)

	// 1. Highest priority: first candidate ID that exists in the active session store.
	// Checking all candidates prevents an outer GUARD_SESSION_ID (e.g. during nested tests)
	// from masking a valid session present in the payload.
	for _, id := range candidates {
		if sess, err := h.sessionMgr.Get(id); err == nil {
			if m := core.NormalizeMode(sess.Mode); m != "" {
				return id, m, true
			}
		}
	}

	fallbackID := ""
	if len(candidates) > 0 {
		fallbackID = candidates[0]
	}

	// 2. Fallback: environment variables -> workspace .agents/current_mode -> DEV
	return fallbackID, policy.ResolveWorkspaceMode(workspaceRoot), false
}

func (h *Handler) debugf(format string, args ...any) {
	if strings.TrimSpace(os.Getenv("GUARD_DEBUG")) == "1" && h.stderr != nil {
		_, _ = fmt.Fprintf(h.stderr, "[guard:debug] "+format+"\n", args...)
	}
}

func candidateSessionIDs(input *Input) []string {
	raw := []string{
		os.Getenv("GUARD_SESSION_ID"),
		input.ExtractSessionID(),
		os.Getenv("SESSION_ID"),
		os.Getenv("CONVERSATION_ID"),
	}
	var out []string
	seen := make(map[string]bool, len(raw))
	for _, v := range raw {
		id := strings.TrimSpace(v)
		if id != "" && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

func (h *Handler) writeDecision(out Output) error {
	b, err := json.Marshal(out)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(h.stdout, string(b))
	return err
}
