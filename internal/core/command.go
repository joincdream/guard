package core

import (
	"path"
	"strings"
)

var fileViewers = map[string]bool{
	"cat":     true,
	"head":    true,
	"tail":    true,
	"more":    true,
	"less":    true,
	"strings": true,
	"nl":      true,
	"tac":     true,
	"bat":     true,
}

var inlineScriptRunners = map[string]bool{
	"python":  true,
	"python3": true,
	"node":    true,
	"ruby":    true,
	"perl":    true,
	"bash":    true,
	"sh":      true,
	"zsh":     true,
}

const (
	ruleHandoffSelfApproveBlock   = "handoff-self-approve-block"
	reasonHandoffSelfApproveBlock = "Agent self-approval is blocked: 'guard handoff approve' must be executed by a human operator via 'guard top' or a direct terminal."
)

// evaluateProtectedGuardCommand detects attempts by an agent to invoke protected
// human-gatekeeper commands (such as `guard handoff approve`), including via
// shell chaining or inline script wrappers.
func evaluateProtectedGuardCommand(cmdLine string) (string, string) {
	return evaluateProtectedGuardCommandDepth(cmdLine, 0)
}

func evaluateProtectedGuardCommandDepth(cmdLine string, depth int) (string, string) {
	if depth > 3 {
		return "", ""
	}
	segments := splitCommandSegments(cmdLine)
	for _, seg := range segments {
		trimmed := strings.TrimSpace(seg)
		if trimmed == "" {
			continue
		}
		tokens := tokenizeSegment(trimmed)
		cmdIdx := findCommandIndex(tokens)
		if cmdIdx < 0 {
			continue
		}
		baseCmd := strings.ToLower(path.Base(tokens[cmdIdx]))
		args := tokens[cmdIdx+1:]

		if baseCmd == "guard" || baseCmd == "guard.exe" {
			posArgs := extractPathArgs(args)
			if len(posArgs) >= 2 &&
				strings.EqualFold(posArgs[0], "handoff") &&
				strings.EqualFold(posArgs[1], "approve") {
				return ruleHandoffSelfApproveBlock, reasonHandoffSelfApproveBlock
			}
		}

		if inlineScriptRunners[baseCmd] {
			for i := 0; i < len(args); i++ {
				if (args[i] == "-c" || args[i] == "-e" || args[i] == "-lc") && i+1 < len(args) {
					script := args[i+1]
					if baseCmd == "bash" || baseCmd == "sh" || baseCmd == "zsh" {
						if ruleName, reason := evaluateProtectedGuardCommandDepth(script, depth+1); reason != "" {
							return ruleName, reason
						}
					}
					if containsGuardHandoffApprove(script) {
						return ruleHandoffSelfApproveBlock, reasonHandoffSelfApproveBlock
					}
					i++
				}
			}
		}
	}
	return "", ""
}

func containsGuardHandoffApprove(script string) bool {
	candidates := extractScriptCandidates(script)
	stage := 0
	for _, c := range candidates {
		tok := strings.ToLower(path.Base(strings.TrimSpace(c)))
		switch stage {
		case 0:
			if tok == "guard" || tok == "guard.exe" {
				stage = 1
			}
		case 1:
			if tok == "handoff" {
				stage = 2
			}
		case 2:
			if tok == "approve" {
				return true
			}
		}
	}
	return false
}

// evaluateCommandRules parses a shell command line into segments and inspects each segment
// for explicit command execution rules, read/write policy violations, and shell bypass attempts.
func evaluateCommandRules(policy *Policy, currentMode string, cmdLine string) (string, string) {
	if hasCommandRules(policy, currentMode) {
		if ruleName, reason := evaluateCommandExecRulesDepth(policy, currentMode, cmdLine, 0); reason != "" {
			return ruleName, reason
		}
	}

	segments := splitCommandSegments(cmdLine)
	if len(segments) == 0 {
		return "", ""
	}

	hasReadRules := hasRulesFor(policy, currentMode, RuleTypeRead)
	hasWriteRules := hasRulesFor(policy, currentMode, RuleTypeWrite)

	for _, seg := range segments {
		trimmed := strings.TrimSpace(seg)
		if trimmed == "" {
			continue
		}

		tokens := tokenizeSegment(trimmed)
		if len(tokens) == 0 {
			continue
		}

		if isSafeSegment(tokens) {
			continue
		}

		// Check read bypass attempts (e.g. DEV mode test-file read, TEST mode prod-code read).
		if hasReadRules {
			if ruleName, reason := inspectReadSegment(policy, currentMode, trimmed, tokens); reason != "" {
				return ruleName, reason
			}
		}

		// Check write bypass attempts (e.g. PLAN mode source code write).
		if hasWriteRules {
			if ruleName, reason := inspectWriteSegment(policy, currentMode, trimmed, tokens); reason != "" {
				return ruleName, reason
			}
		}
	}

	return "", ""
}

func hasCommandRules(policy *Policy, currentMode string) bool {
	if policy == nil {
		return false
	}
	for _, r := range policy.Rules {
		if (r.Mode == "" || strings.EqualFold(r.Mode, currentMode)) && isCommandRuleType(r.Type) {
			return true
		}
	}
	return false
}

func evaluateCommandExecRulesDepth(policy *Policy, currentMode string, cmdLine string, depth int) (string, string) {
	if depth > 3 {
		return "", ""
	}

	// Inspect command substitutions $(...) and `...` first.
	for _, sub := range extractCommandSubstitutions(cmdLine) {
		if ruleName, reason := evaluateCommandExecRulesDepth(policy, currentMode, sub, depth+1); reason != "" {
			return ruleName, reason
		}
	}

	segments := splitCommandSegments(cmdLine)
	for _, seg := range segments {
		trimmed := trimSubshellParens(strings.TrimSpace(seg))
		if trimmed == "" {
			continue
		}

		tokens := tokenizeSegment(trimmed)
		if len(tokens) == 0 {
			continue
		}

		cleanTokens := stripRedirectionTokens(tokens)
		if len(cleanTokens) == 0 {
			continue
		}

		for _, cmdSlice := range collectUnwrappedCommands(cleanTokens) {
			candidates := buildCommandCandidates(cmdSlice)
			if ruleName, reason := evaluateExecCommandRules(policy, currentMode, candidates); reason != "" {
				return ruleName, reason
			}

			baseCmd := strings.ToLower(path.Base(cmdSlice[0]))
			args := cmdSlice[1:]

			if isShellInterpreter(baseCmd) {
				for i := 0; i < len(args); i++ {
					if isShellInlineFlag(args[i]) && i+1 < len(args) {
						if ruleName, reason := evaluateCommandExecRulesDepth(policy, currentMode, args[i+1], depth+1); reason != "" {
							return ruleName, reason
						}
						i++
					}
				}
			} else if inlineScriptRunners[baseCmd] {
				for i := 0; i < len(args); i++ {
					if (args[i] == "-c" || args[i] == "-e" || args[i] == "--eval") && i+1 < len(args) {
						if ruleName, reason := inspectInlineScriptCommandExec(policy, currentMode, args[i+1]); reason != "" {
							return ruleName, reason
						}
						i++
					}
				}
			}
		}
	}

	return "", ""
}

func isShellInterpreter(baseCmd string) bool {
	switch baseCmd {
	case "sh", "bash", "zsh", "dash", "ksh":
		return true
	default:
		return false
	}
}

func isShellInlineFlag(arg string) bool {
	if !strings.HasPrefix(arg, "-") || strings.HasPrefix(arg, "--") || len(arg) < 2 {
		return false
	}
	return strings.ContainsRune(arg[1:], 'c')
}

func inspectInlineScriptCommandExec(policy *Policy, currentMode string, script string) (string, string) {
	stmts := strings.FieldsFunc(script, func(r rune) bool {
		return r == ';' || r == '\n'
	})
	for _, stmt := range stmts {
		cands := extractScriptCandidates(stmt)
		for k := 0; k < len(cands); k++ {
			joined := strings.Join(cands[k:], " ")
			baseJoined := path.Base(cands[k])
			if len(cands[k+1:]) > 0 {
				baseJoined += " " + strings.Join(cands[k+1:], " ")
			}
			checkList := []string{baseJoined}
			if joined != baseJoined {
				checkList = append(checkList, joined)
			}
			if ruleName, reason := evaluateExecCommandRules(policy, currentMode, checkList); reason != "" {
				return ruleName, reason
			}
		}
	}
	return "", ""
}

func stripRedirectionTokens(tokens []string) []string {
	out := make([]string, 0, len(tokens))
	for i := 0; i < len(tokens); i++ {
		if isRedirectionOp(tokens[i]) {
			i++ // skip redirection target
			continue
		}
		out = append(out, tokens[i])
	}
	return out
}

func isEnvAssignmentToken(tok string) bool {
	return strings.Contains(tok, "=") && !strings.HasPrefix(tok, "=") && !strings.Contains(tok, "/")
}

// collectUnwrappedCommands returns slices starting at the outer executable and any inner executable
// unwrapped past common command wrappers (env, sudo, command, nohup, time, nice, timeout).
func collectUnwrappedCommands(tokens []string) [][]string {
	var slices [][]string
	cur := skipEnvAssignments(tokens)
	for depth := 0; depth < 5 && len(cur) > 0; depth++ {
		slices = append(slices, cur)
		next := unwrapOnce(cur)
		if len(next) == 0 || len(next) == len(cur) {
			break
		}
		cur = skipEnvAssignments(next)
	}
	return slices
}

func skipEnvAssignments(tokens []string) []string {
	i := 0
	for i < len(tokens) && (tokens[i] == "" || isEnvAssignmentToken(tokens[i])) {
		i++
	}
	return tokens[i:]
}

func unwrapOnce(tokens []string) []string {
	if len(tokens) < 2 {
		return nil
	}
	base := strings.ToLower(path.Base(tokens[0]))
	args := tokens[1:]

	switch base {
	case "env":
		i := 0
		for i < len(args) {
			a := args[i]
			if a == "-u" || a == "--unset" || a == "-C" || a == "--chdir" || a == "-S" {
				i += 2
				continue
			}
			if strings.HasPrefix(a, "-") || isEnvAssignmentToken(a) {
				i++
				continue
			}
			break
		}
		if i < len(args) {
			return args[i:]
		}
	case "sudo":
		i := 0
		for i < len(args) {
			a := args[i]
			if a == "-u" || a == "-g" || a == "-C" || a == "-D" || a == "-R" || a == "-T" {
				i += 2
				continue
			}
			if strings.HasPrefix(a, "-") {
				i++
				continue
			}
			break
		}
		if i < len(args) {
			return args[i:]
		}
	case "command", "nohup", "time":
		i := 0
		for i < len(args) && strings.HasPrefix(args[i], "-") {
			if base == "command" && (args[i] == "-v" || args[i] == "-V") {
				return nil // command -v/-V only queries path, does not execute
			}
			i++
		}
		if i < len(args) {
			return args[i:]
		}
	case "nice":
		i := 0
		for i < len(args) {
			if args[i] == "-n" || args[i] == "--adjustment" {
				i += 2
				continue
			}
			if strings.HasPrefix(args[i], "-") {
				i++
				continue
			}
			break
		}
		if i < len(args) {
			return args[i:]
		}
	case "timeout":
		i := 0
		for i < len(args) {
			a := args[i]
			if a == "-k" || a == "-s" || a == "--kill-after" || a == "--signal" {
				i += 2
				continue
			}
			if strings.HasPrefix(a, "-") {
				i++
				continue
			}
			break
		}
		// Skip the duration positional argument before the wrapped command.
		if i+1 < len(args) {
			return args[i+1:]
		}
	}
	return nil
}

func buildCommandCandidates(cmdTokens []string) []string {
	if len(cmdTokens) == 0 {
		return nil
	}
	rawExec := cmdTokens[0]
	baseCmd := path.Base(rawExec)
	args := cmdTokens[1:]

	var cands []string
	seen := make(map[string]bool, 3)
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s != "" && !seen[s] {
			seen[s] = true
			cands = append(cands, s)
		}
	}

	if len(args) > 0 {
		rest := strings.Join(args, " ")
		add(baseCmd + " " + rest)
		if strings.Contains(rawExec, "/") {
			add(normalizePath(rawExec) + " " + rest)
		}
		if stripped := stripLeadingSubcommandFlags(baseCmd, args); len(stripped) > 0 && len(stripped) != len(args) {
			add(baseCmd + " " + strings.Join(stripped, " "))
		}
	} else {
		add(baseCmd)
		if strings.Contains(rawExec, "/") {
			add(normalizePath(rawExec))
		}
	}

	return cands
}

func stripLeadingSubcommandFlags(baseCmd string, args []string) []string {
	switch strings.ToLower(baseCmd) {
	case "go":
		i := 0
		for i < len(args) {
			if args[i] == "-C" && i+1 < len(args) {
				i += 2
				continue
			}
			if strings.HasPrefix(args[i], "-C=") {
				i++
				continue
			}
			break
		}
		return args[i:]
	case "git":
		i := 0
		for i < len(args) {
			a := args[i]
			if (a == "-C" || a == "-c" || a == "--git-dir" || a == "--work-tree") && i+1 < len(args) {
				i += 2
				continue
			}
			if strings.HasPrefix(a, "-") {
				i++
				continue
			}
			break
		}
		return args[i:]
	case "make":
		var targets []string
		for i := 0; i < len(args); i++ {
			a := args[i]
			if (a == "-C" || a == "-f" || a == "--directory" || a == "--file") && i+1 < len(args) {
				i++
				continue
			}
			if strings.HasPrefix(a, "-") || isEnvAssignmentToken(a) {
				continue
			}
			targets = append(targets, a)
		}
		return targets
	}
	return args
}

func trimSubshellParens(seg string) string {
	s := strings.TrimSpace(seg)
	for strings.HasPrefix(s, "(") {
		s = strings.TrimSpace(s[1:])
	}
	openCount, closeCount := countUnquotedParens(s)
	for strings.HasSuffix(s, ")") && closeCount > openCount {
		s = strings.TrimSpace(s[:len(s)-1])
		openCount, closeCount = countUnquotedParens(s)
	}
	return s
}

func countUnquotedParens(s string) (int, int) {
	inSingle, inDouble, escaped := false, false, false
	opens, closes := 0, 0
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' && !inSingle {
			escaped = true
			continue
		}
		if ch == '\'' && !inDouble {
			inSingle = !inSingle
			continue
		}
		if ch == '"' && !inSingle {
			inDouble = !inDouble
			continue
		}
		if !inSingle && !inDouble {
			if ch == '(' {
				opens++
			} else if ch == ')' {
				closes++
			}
		}
	}
	return opens, closes
}

func extractCommandSubstitutions(cmdLine string) []string {
	var subs []string
	inSingle := false
	escaped := false
	n := len(cmdLine)

	for i := 0; i < n; i++ {
		ch := cmdLine[i]
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' && !inSingle {
			escaped = true
			continue
		}
		if ch == '\'' {
			inSingle = !inSingle
			continue
		}
		if inSingle {
			continue
		}

		// $( ... )
		if ch == '$' && i+1 < n && cmdLine[i+1] == '(' {
			depth := 1
			start := i + 2
			j := start
			subSingle, subEsc := false, false
			for j < n && depth > 0 {
				c := cmdLine[j]
				if subEsc {
					subEsc = false
					j++
					continue
				}
				if c == '\\' && !subSingle {
					subEsc = true
					j++
					continue
				}
				if c == '\'' {
					subSingle = !subSingle
					j++
					continue
				}
				if !subSingle {
					if c == '(' {
						depth++
					} else if c == ')' {
						depth--
						if depth == 0 {
							break
						}
					}
				}
				j++
			}
			if depth == 0 && j > start {
				subs = append(subs, cmdLine[start:j])
				i = j
				continue
			}
		}

		// ` ... `
		if ch == '`' {
			start := i + 1
			j := start
			subEsc := false
			for j < n {
				c := cmdLine[j]
				if subEsc {
					subEsc = false
					j++
					continue
				}
				if c == '\\' {
					subEsc = true
					j++
					continue
				}
				if c == '`' {
					break
				}
				j++
			}
			if j < n && j > start {
				subs = append(subs, cmdLine[start:j])
				i = j
				continue
			}
		}
	}
	return subs
}

func hasRulesFor(policy *Policy, currentMode string, actionType string) bool {
	if policy == nil {
		return false
	}
	for _, r := range policy.Rules {
		if (r.Mode == "" || strings.EqualFold(r.Mode, currentMode)) &&
			(r.Type == "" || strings.EqualFold(r.Type, actionType)) {
			return true
		}
	}
	return false
}

// isSafeSegment returns true if a tokenized command segment is a known safe read-only/build command
// with no output redirection.
func isSafeSegment(tokens []string) bool {
	// Any output redirection disqualifies a segment from being automatically safe.
	if hasOutputRedirection(tokens) {
		return false
	}

	cmdIdx := findCommandIndex(tokens)
	if cmdIdx < 0 {
		return false
	}

	cmd := path.Base(tokens[cmdIdx])
	args := tokens[cmdIdx+1:]

	switch cmd {
	case "git":
		if len(args) == 0 {
			return false
		}
		switch args[0] {
		case "status", "branch", "commit":
			return true
		case "log":
			// git log is safe unless it prints diffs/patches.
			for _, a := range args[1:] {
				if a == "-p" || a == "--patch" || a == "-u" || strings.HasPrefix(a, "--patch=") {
					return false
				}
			}
			return true
		}
	case "go":
		if len(args) > 0 && (args[0] == "build" || args[0] == "vet") {
			return true
		}
	}

	return false
}

func inspectReadSegment(policy *Policy, currentMode string, rawSeg string, tokens []string) (string, string) {
	cmdIdx := findCommandIndex(tokens)
	if cmdIdx < 0 {
		return "", ""
	}

	cmd := path.Base(tokens[cmdIdx])
	args := tokens[cmdIdx+1:]

	// Pattern A: File dumpers / viewers (cat, head, tail, more, less, strings, etc.)
	if fileViewers[cmd] {
		for _, arg := range extractPathArgs(args) {
			if ruleName, reason := evaluatePathRules(policy, currentMode, "read", arg); reason != "" {
				return ruleName, reason
			}
		}
	}

	// Input redirection (< target) on any command
	for _, target := range extractInputRedirectionTargets(tokens) {
		if ruleName, reason := evaluatePathRules(policy, currentMode, "read", target); reason != "" {
			return ruleName, reason
		}
	}

	// Pattern B: Git inspectors (git show, git diff, git checkout, git log -p)
	if cmd == "git" && len(args) >= 1 {
		subCmd := args[0]
		isGitInspector := subCmd == "show" || subCmd == "diff" || subCmd == "checkout"
		if subCmd == "log" {
			for _, a := range args[1:] {
				if a == "-p" || a == "--patch" || a == "-u" {
					isGitInspector = true
					break
				}
			}
		}
		if isGitInspector {
			for _, arg := range args[1:] {
				if strings.HasPrefix(arg, "-") {
					continue
				}
				if ruleName, reason := evaluatePathRules(policy, currentMode, "read", arg); reason != "" {
					return ruleName, reason
				}
				// Handle rev:path syntax such as HEAD:pkg/foo_test.go
				if idx := strings.Index(arg, ":"); idx >= 0 && idx+1 < len(arg) {
					revPath := arg[idx+1:]
					if ruleName, reason := evaluatePathRules(policy, currentMode, "read", revPath); reason != "" {
						return ruleName, reason
					}
				}
			}
		}
	}

	// Pattern C: Inline scripting / subshells (python -c, node -e, bash -c, etc.)
	if ruleName, reason := inspectInlineScript(policy, currentMode, "read", cmd, args, rawSeg); reason != "" {
		return ruleName, reason
	}

	return "", ""
}

func inspectWriteSegment(policy *Policy, currentMode string, rawSeg string, tokens []string) (string, string) {
	// 1. Output redirection (>, >>)
	for _, target := range extractOutputRedirectionTargets(tokens) {
		if ruleName, reason := evaluatePathRules(policy, currentMode, "write", target); reason != "" {
			return ruleName, reason
		}
	}

	cmdIdx := findCommandIndex(tokens)
	if cmdIdx < 0 {
		return "", ""
	}

	cmd := path.Base(tokens[cmdIdx])
	args := tokens[cmdIdx+1:]

	// 2. tee
	if cmd == "tee" {
		for _, arg := range extractPathArgs(args) {
			if ruleName, reason := evaluatePathRules(policy, currentMode, "write", arg); reason != "" {
				return ruleName, reason
			}
		}
	}

	// 3. sed -i / sed --in-place
	if cmd == "sed" {
		inPlace := false
		for _, a := range args {
			if a == "-i" || strings.HasPrefix(a, "-i") || a == "--in-place" || strings.HasPrefix(a, "--in-place=") {
				inPlace = true
				break
			}
		}
		if inPlace {
			for _, arg := range extractPathArgs(args) {
				if ruleName, reason := evaluatePathRules(policy, currentMode, "write", arg); reason != "" {
					return ruleName, reason
				}
			}
		}
	}

	// 4. Inline scripting writing to protected paths (e.g. python -c "open('src/internal/a.go','w')...")
	if ruleName, reason := inspectInlineScript(policy, currentMode, "write", cmd, args, rawSeg); reason != "" {
		return ruleName, reason
	}

	return "", ""
}

func inspectInlineScript(policy *Policy, currentMode string, actionType string, cmd string, args []string, rawSeg string) (string, string) {
	if !inlineScriptRunners[cmd] {
		return "", ""
	}
	hasInlineFlag := false
	for _, a := range args {
		if a == "-c" || a == "-e" || a == "--eval" {
			hasInlineFlag = true
			break
		}
	}
	if !hasInlineFlag {
		return "", ""
	}
	for _, candidate := range extractScriptCandidates(rawSeg) {
		if ruleName, reason := evaluatePathRules(policy, currentMode, actionType, candidate); reason != "" {
			return ruleName, reason
		}
	}
	return "", ""
}

// splitCommandSegments splits a shell command line by control operators (|, ||, &&, ;, \n)
// while respecting single and double quotes and backslash escapes.
func splitCommandSegments(cmdLine string) []string {
	var segments []string
	var cur strings.Builder

	inSingle := false
	inDouble := false
	escaped := false

	n := len(cmdLine)
	for i := 0; i < n; i++ {
		ch := cmdLine[i]

		if escaped {
			cur.WriteByte(ch)
			escaped = false
			continue
		}

		if ch == '\\' && !inSingle {
			cur.WriteByte(ch)
			escaped = true
			continue
		}

		if ch == '\'' && !inDouble {
			inSingle = !inSingle
			cur.WriteByte(ch)
			continue
		}

		if ch == '"' && !inSingle {
			inDouble = !inDouble
			cur.WriteByte(ch)
			continue
		}

		if !inSingle && !inDouble {
			if ch == ';' || ch == '\n' {
				if s := strings.TrimSpace(cur.String()); s != "" {
					segments = append(segments, s)
				}
				cur.Reset()
				continue
			}
			if ch == '|' {
				if i+1 < n && cmdLine[i+1] == '|' {
					i++ // consume '||'
				}
				if s := strings.TrimSpace(cur.String()); s != "" {
					segments = append(segments, s)
				}
				cur.Reset()
				continue
			}
			if ch == '&' && i+1 < n && cmdLine[i+1] == '&' {
				i++ // consume '&&'
				if s := strings.TrimSpace(cur.String()); s != "" {
					segments = append(segments, s)
				}
				cur.Reset()
				continue
			}
		}

		cur.WriteByte(ch)
	}

	if s := strings.TrimSpace(cur.String()); s != "" {
		segments = append(segments, s)
	}
	return segments
}

// tokenizeSegment splits a single command segment into shell tokens, separating redirection operators (> >> <)
// and stripping outer quotes.
func tokenizeSegment(seg string) []string {
	var tokens []string
	var cur strings.Builder

	inSingle := false
	inDouble := false
	escaped := false

	flush := func() {
		if cur.Len() > 0 {
			tokens = append(tokens, cur.String())
			cur.Reset()
		}
	}

	n := len(seg)
	for i := 0; i < n; i++ {
		ch := seg[i]

		if escaped {
			cur.WriteByte(ch)
			escaped = false
			continue
		}

		if ch == '\\' && !inSingle {
			escaped = true
			continue
		}

		if ch == '\'' && !inDouble {
			inSingle = !inSingle
			continue
		}

		if ch == '"' && !inSingle {
			inDouble = !inDouble
			continue
		}

		if !inSingle && !inDouble {
			if ch == ' ' || ch == '\t' || ch == '\r' {
				flush()
				continue
			}
			if ch == '>' || ch == '<' {
				// Keep fd prefix like "1>" or "2>>" attached to operator, otherwise flush.
				curStr := cur.String()
				if curStr != "1" && curStr != "2" && curStr != "&" {
					flush()
				}
				cur.WriteByte(ch)
				if ch == '>' && i+1 < n && seg[i+1] == '>' {
					i++
					cur.WriteByte('>')
				}
				flush()
				continue
			}
		}

		cur.WriteByte(ch)
	}

	flush()
	return tokens
}

// findCommandIndex returns the index of the primary executable token, skipping leading ENV=val assignments and sudo/env.
func findCommandIndex(tokens []string) int {
	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		if tok == "" {
			continue
		}
		if isRedirectionOp(tok) {
			i++ // skip redirection target
			continue
		}
		if strings.Contains(tok, "=") && !strings.HasPrefix(tok, "=") && !strings.Contains(tok, "/") {
			continue
		}
		base := path.Base(tok)
		if base == "sudo" || base == "env" || base == "command" {
			continue
		}
		return i
	}
	return -1
}

func isRedirectionOp(tok string) bool {
	return tok == ">" || tok == ">>" || tok == "1>" || tok == "1>>" || tok == "2>" || tok == "2>>" || tok == "&>" || tok == "&>>" || tok == "<"
}

func isOutputRedirectionOp(tok string) bool {
	return tok == ">" || tok == ">>" || tok == "1>" || tok == "1>>" || tok == "2>" || tok == "2>>" || tok == "&>" || tok == "&>>"
}

// isBenignOutputTarget reports whether a redirection target does not write to a filesystem path
// (e.g. /dev/null or file descriptor duplication/close like &1, &2, &-).
func isBenignOutputTarget(target string) bool {
	if target == "/dev/null" {
		return true
	}
	if !strings.HasPrefix(target, "&") || len(target) < 2 {
		return false
	}
	rest := target[1:]
	if rest == "-" {
		return true
	}
	for i := 0; i < len(rest); i++ {
		if rest[i] < '0' || rest[i] > '9' {
			return false
		}
	}
	return true
}

func hasOutputRedirection(tokens []string) bool {
	for i := 0; i < len(tokens); i++ {
		if isOutputRedirectionOp(tokens[i]) {
			if i+1 >= len(tokens) || !isBenignOutputTarget(tokens[i+1]) {
				return true
			}
			i++
		}
	}
	return false
}

func extractOutputRedirectionTargets(tokens []string) []string {
	var targets []string
	for i := 0; i < len(tokens); i++ {
		if isOutputRedirectionOp(tokens[i]) && i+1 < len(tokens) {
			if !isBenignOutputTarget(tokens[i+1]) {
				targets = append(targets, tokens[i+1])
			}
			i++
		}
	}
	return targets
}

func extractInputRedirectionTargets(tokens []string) []string {
	var targets []string
	for i := 0; i < len(tokens); i++ {
		if tokens[i] == "<" && i+1 < len(tokens) {
			targets = append(targets, tokens[i+1])
			i++
		}
	}
	return targets
}

// extractPathArgs extracts non-flag arguments while skipping redirection operators and their targets.
func extractPathArgs(args []string) []string {
	var res []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if isRedirectionOp(arg) {
			i++ // skip redirection target
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		res = append(res, arg)
	}
	return res
}

// extractScriptCandidates splits an inline script string by common delimiters to extract candidate file paths.
func extractScriptCandidates(script string) []string {
	f := func(r rune) bool {
		switch r {
		case ' ', '\t', '\n', '\r', '\'', '"', '`', '(', ')', '[', ']', '{', '}', ',', ';', '=', '+':
			return true
		default:
			return false
		}
	}
	return strings.FieldsFunc(script, f)
}
