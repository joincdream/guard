package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"guard/internal/core"
	"guard/internal/hub/handoff"
	"guard/internal/hub/policy"
	"guard/internal/hub/session"
	"guard/internal/i18n"
)

type multiStringFlag []string

func (m *multiStringFlag) String() string {
	return strings.Join(*m, ",")
}

func (m *multiStringFlag) Set(val string) error {
	for _, part := range strings.Split(val, ",") {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			*m = append(*m, trimmed)
		}
	}
	return nil
}

func (a *cliApp) knownModes() []string {
	polRes := policy.LoadDetailed(a.workspaceRoot)
	return core.ExtractModes(polRes.Policy)
}

func (a *cliApp) ensureHandoffManager() (*handoff.Manager, error) {
	if a.handoffMgr != nil {
		return a.handoffMgr, nil
	}
	mgr, err := handoff.NewManager("", a.sessMgr.Dir())
	if err != nil {
		return nil, err
	}
	mgr.SetKnownModes(a.knownModes())
	a.handoffMgr = mgr
	return mgr, nil
}

func (a *cliApp) runHandoff(args []string) int {
	if len(args) == 0 {
		a.printHandoffHelp()
		return 1
	}

	sub := strings.ToLower(strings.TrimSpace(args[0]))
	switch sub {
	case "send":
		return a.runHandoffSend(args[1:])
	case "inbox":
		return a.runHandoffInbox(args[1:])
	case "approve":
		return a.runHandoffApprove(args[1:])
	case "reject":
		return a.runHandoffReject(args[1:])
	case "ack":
		return a.runHandoffAck(args[1:])
	case "help", "-h", "--help":
		a.printHandoffHelp()
		return 0
	default:
		fmt.Fprintf(a.stderr, "unknown handoff subcommand: %s\n", sub)
		a.printHandoffHelp()
		return 1
	}
}

func (a *cliApp) runHandoffSend(args []string) int {
	fs := flag.NewFlagSet("handoff send", flag.ContinueOnError)
	fs.SetOutput(a.stderr)

	var (
		toRaw    string
		typeRaw  string
		parentID string
		summary  string
		payload  string
		refs     multiStringFlag
		targets  multiStringFlag
	)

	fs.StringVar(&toRaw, "to", "", "Target mode (plan|dev|test|all) or session ID")
	fs.StringVar(&typeRaw, "type", string(handoff.TypeTask), "Handoff type (task|verify|fix|review)")
	fs.StringVar(&parentID, "parent", "", "Parent handoff ID for thread tracking")
	fs.StringVar(&summary, "m", "", "Short summary message (required)")
	fs.StringVar(&summary, "summary", "", "Short summary message (required)")
	fs.StringVar(&payload, "payload", "", "Optional inline instructions")
	fs.Var(&refs, "ref", "Reference document path (repeatable or comma-separated)")
	fs.Var(&targets, "targets", "Target file/glob path (repeatable or comma-separated)")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	// Allow positional arguments as additional reference paths if provided.
	for _, pos := range fs.Args() {
		trimmed := strings.TrimSpace(pos)
		if trimmed != "" {
			refs = append(refs, trimmed)
		}
	}

	knownModes := a.knownModes()
	toNode, err := handoff.ParseTargetWithModes(toRaw, knownModes)
	if err != nil {
		fmt.Fprintf(a.stderr, "error: %v\n", err)
		return 1
	}

	mgr, err := a.ensureHandoffManager()
	if err != nil {
		fmt.Fprintf(a.stderr, "error: initialize handoff manager: %v\n", err)
		return 1
	}

	fromNode := a.resolveSenderNode(knownModes)
	item := handoff.HandoffItem{
		ParentID:   strings.TrimSpace(parentID),
		Type:       handoff.HandoffType(strings.ToLower(strings.TrimSpace(typeRaw))),
		From:       fromNode,
		To:         toNode,
		Summary:    strings.TrimSpace(summary),
		References: []string(refs),
		Targets:    []string(targets),
		Payload:    strings.TrimSpace(payload),
	}

	enqueued, err := mgr.Enqueue(item)
	if err != nil {
		fmt.Fprintf(a.stderr, "error: enqueue handoff: %v\n", err)
		return 1
	}

	fmt.Fprintf(a.stdout, "Handoff queued: %s [%s] -> %s (%s)\n",
		enqueued.ID,
		strings.ToUpper(string(enqueued.Type)),
		formatHandoffNode(enqueued.To),
		enqueued.Summary,
	)
	return 0
}

func (a *cliApp) runHandoffInbox(args []string) int {
	fs := flag.NewFlagSet("handoff inbox", flag.ContinueOnError)
	fs.SetOutput(a.stderr)

	var (
		showAll   bool
		mode      string
		sessionID string
		asJSON    bool
	)

	fs.BoolVar(&showAll, "all", false, "Include completed (acked/rejected) tickets")
	fs.StringVar(&mode, "mode", "", "Filter by target mode (plan|dev|test|all)")
	fs.StringVar(&sessionID, "session", "", "Filter by target session ID")
	fs.BoolVar(&asJSON, "json", false, "Output as JSON")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	mgr, err := a.ensureHandoffManager()
	if err != nil {
		fmt.Fprintf(a.stderr, "error: initialize handoff manager: %v\n", err)
		return 1
	}

	items, err := mgr.List(handoff.ListFilter{
		ToMode:      mode,
		ToSessionID: sessionID,
		OnlyActive:  !showAll,
	})
	if err != nil {
		fmt.Fprintf(a.stderr, "error: list handoff queue: %v\n", err)
		return 1
	}

	if asJSON {
		out, err := json.MarshalIndent(items, "", "  ")
		if err != nil {
			fmt.Fprintf(a.stderr, "error: marshal handoff items: %v\n", err)
			return 1
		}
		fmt.Fprintln(a.stdout, string(out))
		return 0
	}

	if len(items) == 0 {
		fmt.Fprintln(a.stdout, "No handoff tickets found.")
		return 0
	}

	fmt.Fprintf(a.stdout, "%-22s %-7s %-18s %-9s %-10s %s\n", "ID", "TYPE", "ROUTE", "STATUS", "UPDATED", "SUMMARY")
	fmt.Fprintln(a.stdout, strings.Repeat("-", 92))
	for _, it := range items {
		route := fmt.Sprintf("%s -> %s", formatHandoffNode(it.From), formatHandoffNode(it.To))
		fmt.Fprintf(a.stdout, "%-22s %-7s %-18s %-9s %-10s %s\n",
			it.ID,
			strings.ToUpper(string(it.Type)),
			route,
			string(it.Status),
			i18n.RelativeTime(it.UpdatedAt),
			it.Summary,
		)
	}
	return 0
}

func (a *cliApp) runHandoffApprove(args []string) int {
	if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
		fmt.Fprintln(a.stderr, "usage: guard handoff approve <id>")
		return 1
	}

	mgr, err := a.ensureHandoffManager()
	if err != nil {
		fmt.Fprintf(a.stderr, "error: initialize handoff manager: %v\n", err)
		return 1
	}

	res, err := handoff.ApproveAndRoute(mgr, a.sessMgr, a.workspaceRoot, strings.TrimSpace(args[0]))
	if err != nil {
		fmt.Fprintf(a.stderr, "error: approve handoff: %v\n", err)
		return 1
	}

	if res.Buffered {
		fmt.Fprintf(a.stdout, "Handoff approved (buffered): %s -> mode %s (will deliver on next session start)\n",
			res.Item.ID, strings.ToUpper(res.Item.To.Mode))
		return 0
	}

	scopeNote := ""
	if res.ScopeLocked {
		scopeNote = fmt.Sprintf(" [Scope Locked: %s]", strings.Join(res.Item.Targets, ", "))
	}
	fmt.Fprintf(a.stdout, "Handoff approved and delivered: %s -> session %s (%s)%s\n",
		res.Item.ID, session.ShortID(res.SessionID), res.InboxPath, scopeNote)
	return 0
}

func (a *cliApp) runHandoffReject(args []string) int {
	fs := flag.NewFlagSet("handoff reject", flag.ContinueOnError)
	fs.SetOutput(a.stderr)

	var reason string
	fs.StringVar(&reason, "m", "", "Rejection reason")
	fs.StringVar(&reason, "reason", "", "Rejection reason")

	if err := fs.Parse(normalizeRejectArgs(args)); err != nil {
		return 1
	}
	if fs.NArg() != 1 || strings.TrimSpace(fs.Arg(0)) == "" {
		fmt.Fprintln(a.stderr, "usage: guard handoff reject <id> [-m \"reason\"]")
		return 1
	}

	mgr, err := a.ensureHandoffManager()
	if err != nil {
		fmt.Fprintf(a.stderr, "error: initialize handoff manager: %v\n", err)
		return 1
	}

	updated, err := mgr.UpdateStatus(strings.TrimSpace(fs.Arg(0)), handoff.StatusRejected, reason)
	if err != nil {
		fmt.Fprintf(a.stderr, "error: reject handoff: %v\n", err)
		return 1
	}
	fmt.Fprintf(a.stdout, "Handoff rejected: %s\n", updated.ID)
	return 0
}

// normalizeRejectArgs reorders flags placed after the positional ticket ID so both
// `guard handoff reject <id> -m "reason"` and `guard handoff reject -m "reason" <id>` work.
func normalizeRejectArgs(args []string) []string {
	var flags []string
	var positionals []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positionals = append(positionals, args[i:]...)
			break
		}
		if strings.HasPrefix(arg, "-") && arg != "-" {
			flags = append(flags, arg)
			name := strings.TrimLeft(arg, "-")
			if (name == "m" || name == "reason") && !strings.Contains(arg, "=") && i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
			continue
		}
		positionals = append(positionals, arg)
	}
	return append(flags, positionals...)
}

func (a *cliApp) runHandoffAck(args []string) int {
	if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
		fmt.Fprintln(a.stderr, "usage: guard handoff ack <id>")
		return 1
	}

	mgr, err := a.ensureHandoffManager()
	if err != nil {
		fmt.Fprintf(a.stderr, "error: initialize handoff manager: %v\n", err)
		return 1
	}

	updated, err := mgr.UpdateStatus(strings.TrimSpace(args[0]), handoff.StatusAcked, "")
	if err != nil {
		fmt.Fprintf(a.stderr, "error: ack handoff: %v\n", err)
		return 1
	}
	fmt.Fprintf(a.stdout, "Handoff acknowledged: %s\n", updated.ID)
	return 0
}

func (a *cliApp) resolveSenderNode(knownModes []string) handoff.HandoffNode {
	var node handoff.HandoffNode
	for _, envKey := range []string{"GUARD_SESSION_ID", "SESSION_ID", "CONVERSATION_ID"} {
		if sid := strings.TrimSpace(os.Getenv(envKey)); sid != "" && handoff.ValidSafeID(sid) {
			node.SessionID = sid
			if sess, err := a.sessMgr.Get(sid); err == nil && sess != nil {
				m := strings.ToLower(strings.TrimSpace(sess.Mode))
				if handoff.IsKnownMode(m, knownModes) {
					node.Mode = m
				}
			}
			break
		}
	}
	if node.Mode == "" {
		m := strings.ToLower(strings.TrimSpace(os.Getenv("AGENT_MODE")))
		if handoff.IsKnownMode(m, knownModes) {
			node.Mode = m
		}
	}
	return node
}

func formatHandoffNode(n handoff.HandoffNode) string {
	mode := strings.ToUpper(strings.TrimSpace(n.Mode))
	sid := strings.TrimSpace(n.SessionID)
	switch {
	case mode != "" && sid != "":
		return fmt.Sprintf("%s(%s)", mode, session.ShortID(sid))
	case mode != "":
		return mode
	case sid != "":
		return session.ShortID(sid)
	default:
		return "-"
	}
}

func (a *cliApp) printHandoffHelp() {
	fmt.Fprintln(a.stdout, `Usage: guard handoff <subcommand> [flags]

Subcommands:
  send     --to <plan|dev|test|all|sessionId> [--type <task|verify|fix|review>]
           [--ref <file>...] [--targets <glob>...] [--parent <id>] -m "<summary>" [--payload "<text>"]
  inbox    [--all] [--mode <mode>] [--session <id>] [--json]
  approve  <id>
  reject   <id> [-m "<reason>"]
  ack      <id>`)
}
