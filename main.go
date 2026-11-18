package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// HookInput represents Antigravity PreToolUse payload
type HookInput struct {
	SessionID string `json:"sessionId"`
	ToolCall  struct {
		Name string                 `json:"name"`
		Args map[string]interface{} `json:"args"`
	} `json:"toolCall"`
	ToolName    string                 `json:"tool_name"`
	ToolArgs    map[string]interface{} `json:"tool_args"`
	Path        string                 `json:"Path"`
	CommandLine string                 `json:"CommandLine"`
}

// Rule defines a guard rule from guard-policy.yaml
type Rule struct {
	Name            string   `yaml:"name"`
	Mode            string   `yaml:"mode"` // DEV, PLAN, TEST, or empty for ALL
	Type            string   `yaml:"type"` // read, write
	Patterns        []string `yaml:"patterns"`
	ExcludePatterns []string `yaml:"exclude_patterns"`
	Message         string   `yaml:"message"`
}

// Policy defines the root structure of guard-policy.yaml
type Policy struct {
	Version string `yaml:"version"`
	Rules   []Rule `yaml:"rules"`
}

func main() {
	workspaceRoot := findWorkspaceRoot()

	// If arguments provided, handle CLI subcommands (dev, test, plan, all, status, run, help)
	if len(os.Args) > 1 {
		handleSubcommands(workspaceRoot, os.Args[1:])
		return
	}

	// Read payload from stdin (Hook mode)
	inputBytes, err := io.ReadAll(os.Stdin)
	if err != nil || len(inputBytes) == 0 {
		outputAllow()
		return
	}

	var input HookInput
	_ = json.Unmarshal(inputBytes, &input)

	toolName := extractToolName(&input)
	targetPath := extractTargetPath(&input)
	cmdLine := extractCommandLine(&input)

	mode := determineMode(workspaceRoot, input.SessionID)

	// Mode ALL or QA: bypass all checks
	if mode == "ALL" || mode == "QA" {
		outputAllow()
		return
	}

	policy := loadPolicy(workspaceRoot)

	// Check 1: Tool-level inspection (read_file, write_file, edit_file)
	if toolName == "read_file" {
		if reason := evaluatePathRules(policy, mode, "read", targetPath); reason != "" {
			outputDeny(reason)
			return
		}
	} else if toolName == "write_file" || toolName == "edit_file" {
		if reason := evaluatePathRules(policy, mode, "write", targetPath); reason != "" {
			outputDeny(reason)
			return
		}
	}

	// Check 2: Command-line inspection (run_command)
	if cmdLine != "" {
		if reason := evaluateCommandRules(policy, mode, cmdLine, workspaceRoot); reason != "" {
			outputDeny(reason)
			return
		}
	}

	outputAllow()
}

func extractToolName(input *HookInput) string {
	if input.ToolCall.Name != "" {
		return input.ToolCall.Name
	}
	if input.ToolName != "" {
		return input.ToolName
	}
	return ""
}

func extractTargetPath(input *HookInput) string {
	if p, ok := input.ToolCall.Args["Path"].(string); ok && p != "" {
		return p
	}
	if p, ok := input.ToolArgs["Path"].(string); ok && p != "" {
		return p
	}
	return input.Path
}

func extractCommandLine(input *HookInput) string {
	if c, ok := input.ToolCall.Args["CommandLine"].(string); ok && c != "" {
		return c
	}
	if c, ok := input.ToolArgs["CommandLine"].(string); ok && c != "" {
		return c
	}
	return input.CommandLine
}

func findWorkspaceRoot() string {
	if ws := os.Getenv("WORKSPACE_ROOT"); ws != "" {
		return ws
	}
	// Try walking up from cwd
	cwd, err := os.Getwd()
	if err == nil {
		dir := cwd
		for {
			if _, err := os.Stat(filepath.Join(dir, ".agents")); err == nil {
				return dir
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	return "."
}

func determineMode(workspaceRoot string, sessionID string) string {
	// 1. Session temporary token in /tmp (최우선: 세션 내 실시간 전환값)
	if sessionID != "" {
		sessionFile := filepath.Join("/tmp", fmt.Sprintf("aicc-session-%s.mode", sessionID))
		if data, err := os.ReadFile(sessionFile); err == nil {
			if m := strings.TrimSpace(string(data)); m != "" {
				return strings.ToUpper(m)
			}
		}
		roleFile := filepath.Join("/tmp", fmt.Sprintf("aicc-session-%s.role", sessionID))
		if data, err := os.ReadFile(roleFile); err == nil {
			if r := strings.TrimSpace(string(data)); r != "" {
				return strings.ToUpper(r)
			}
		}
	}

	// 2. Env vars (차상위: 터미널 창별 시작 기본값)
	if m := os.Getenv("AGENT_MODE"); m != "" {
		return strings.ToUpper(strings.TrimSpace(m))
	}
	if r := os.Getenv("AGENT_ROLE"); r != "" {
		return strings.ToUpper(strings.TrimSpace(r))
	}

	// 3. Workspace file (.agents/current_mode) (기본 전역 설정)
	modeFile := filepath.Join(workspaceRoot, ".agents", "current_mode")
	if data, err := os.ReadFile(modeFile); err == nil {
		if m := strings.TrimSpace(string(data)); m != "" {
			return strings.ToUpper(m)
		}
	}

	// Default mode is DEV (Cleanroom)
	return "DEV"
}

func loadPolicy(workspaceRoot string) *Policy {
	policyPath := filepath.Join(workspaceRoot, ".agents", "guard-policy.yaml")
	data, err := os.ReadFile(policyPath)
	if err == nil {
		var p Policy
		if err := yaml.Unmarshal(data, &p); err == nil && len(p.Rules) > 0 {
			return &p
		}
	}

	// Fallback built-in rules
	return &Policy{
		Version: "1.0",
		Rules: []Rule{
			{
				Name:     "default-test-files-read",
				Mode:     "DEV",
				Type:     "read",
				Patterns: []string{"*_test.go", "*.test.ts", "*.test.js", "*.spec.ts", "**/testutil/**", "**/tests/**", "**/000/**"},
				Message:  "[POLICY VIOLATION] 당신은 테스트 파일에 접근할 권한이 없습니다. 우회 시도(Python 스크립트 작성 등)를 즉시 중단하고, 요구사항 명세서와 'go test' 실행 결과(stdout)만을 참조하여 개발을 완료하십시오.",
			},
			{
				Name:     "default-source-code-write",
				Mode:     "PLAN",
				Type:     "write",
				Patterns: []string{"src/internal/**", "src/cmd/**", "src/web/**", "src/docker/**"},
				Message:  "[POLICY VIOLATION] 설계(PLAN) 모드에서는 소스 코드 파일(src/internal, src/cmd, src/web)을 수정할 수 없습니다. 작업 계획서 및 문서 파일만 작성하십시오.",
			},
		},
	}
}

func matchPattern(path string, pattern string) bool {
	// Normalize paths
	cleanPath := filepath.ToSlash(path)
	cleanPattern := filepath.ToSlash(pattern)

	// Direct suffix match (e.g. *_test.go)
	if strings.HasPrefix(cleanPattern, "*") && strings.HasSuffix(cleanPath, cleanPattern[1:]) {
		return true
	}

	// Substring / Glob match
	if matched, _ := filepath.Match(cleanPattern, cleanPath); matched {
		return true
	}
	if matched, _ := filepath.Match(cleanPattern, filepath.Base(cleanPath)); matched {
		return true
	}

	// Double star glob (e.g. **/testutil/**)
	if strings.Contains(cleanPattern, "**") {
		regexPattern := "^" + strings.ReplaceAll(regexp.QuoteMeta(cleanPattern), "\\*\\*", ".*") + "$"
		regexPattern = strings.ReplaceAll(regexPattern, "\\*", "[^/]*")
		if matched, _ := regexp.MatchString(regexPattern, cleanPath); matched {
			return true
		}
	}

	// Directory prefix match
	if strings.HasSuffix(cleanPattern, "/**") {
		prefix := strings.TrimSuffix(cleanPattern, "/**")
		if strings.HasPrefix(cleanPath, prefix) || strings.Contains(cleanPath, "/"+prefix+"/") {
			return true
		}
	}

	return false
}

func evaluatePathRules(policy *Policy, currentMode string, actionType string, path string) string {
	if path == "" {
		return ""
	}
	for _, rule := range policy.Rules {
		if rule.Mode != "" && rule.Mode != currentMode {
			continue
		}
		if rule.Type != "" && rule.Type != actionType {
			continue
		}
		for _, pattern := range rule.Patterns {
			if matchPattern(path, pattern) {
				// Check if excluded by exclude_patterns
				excluded := false
				for _, ex := range rule.ExcludePatterns {
					if matchPattern(path, ex) {
						excluded = true
						break
					}
				}
				if !excluded {
					return rule.Message
				}
			}
		}
	}
	return ""
}

func evaluateCommandRules(policy *Policy, currentMode string, cmd string, workspaceRoot string) string {
	trimmedCmd := strings.TrimSpace(cmd)

	// Safe read-only commands that should not be blocked:
	// 'git status', 'git log' without diff/patch, 'go build'
	if isSafeCommand(trimmedCmd) {
		return ""
	}

	// 1. Check DEV mode file-dump / inline inspection
	if currentMode == "DEV" {
		// Pattern A: File dumpers (cat, head, tail, more, less)
		dumpers := []string{"cat", "head", "tail", "more", "less", "strings"}
		for _, dumper := range dumpers {
			if isCommandCalling(trimmedCmd, dumper) {
				// Check arguments
				args := extractArgsAfter(trimmedCmd, dumper)
				for _, arg := range args {
					// Check wildcard glob expansion
					if strings.ContainsAny(arg, "*?") {
						matches, err := filepath.Glob(filepath.Join(workspaceRoot, arg))
						if err == nil {
							for _, m := range matches {
								if evaluatePathRules(policy, currentMode, "read", m) != "" {
									return getRuleMessage(policy, "guard-test-files-read", "[POLICY VIOLATION] 와일드카드 전개 결과 테스트 파일(*_test.go 등)이 포함되어 있습니다. 접근이 금지됩니다.")
								}
							}
						}
					}
					// Direct arg match
					if evaluatePathRules(policy, currentMode, "read", arg) != "" {
						return getRuleMessage(policy, "guard-test-files-read", "[POLICY VIOLATION] 테스트 파일(*_test.go 등) 열람이 금지되어 있습니다.")
					}
				}
			}
		}

		// Pattern B: Git inspectors (git show, git diff, git log -p)
		if strings.HasPrefix(trimmedCmd, "git") {
			gitArgs := strings.Fields(trimmedCmd)
			if len(gitArgs) >= 2 {
				subCmd := gitArgs[1]
				if subCmd == "show" || subCmd == "diff" || subCmd == "checkout" {
					for _, arg := range gitArgs[2:] {
						if evaluatePathRules(policy, currentMode, "read", arg) != "" || strings.Contains(arg, "_test.go") {
							return getRuleMessage(policy, "guard-test-files-read", "[POLICY VIOLATION] Git을 통한 테스트 파일 열람이 금지되어 있습니다.")
						}
					}
				}
			}
		}

		// Pattern C: Inline scripting (python -c, node -e) containing test files
		if strings.Contains(trimmedCmd, "python") || strings.Contains(trimmedCmd, "node") {
			if strings.Contains(trimmedCmd, "-c") || strings.Contains(trimmedCmd, "-e") {
				if strings.Contains(trimmedCmd, "_test.go") || strings.Contains(trimmedCmd, "testutil") {
					return getRuleMessage(policy, "guard-test-files-read", "[POLICY VIOLATION] 인라인 스크립트를 통한 테스트 파일 접근이 금지되어 있습니다.")
				}
			}
		}
	}

	// 2. Check PLAN mode writes
	if currentMode == "PLAN" {
		// Output redirection (>, >>) or overwriting
		if strings.Contains(trimmedCmd, ">") || strings.Contains(trimmedCmd, "tee") || strings.Contains(trimmedCmd, "sed -i") {
			if strings.Contains(trimmedCmd, "src/internal") || strings.Contains(trimmedCmd, "src/cmd") || strings.Contains(trimmedCmd, "src/web") {
				return getRuleMessage(policy, "guard-source-code-write", "[POLICY VIOLATION] 설계(PLAN) 모드에서는 소스 코드 파일(src/internal, src/cmd, src/web)을 수정할 수 없습니다.")
			}
		}
	}

	return ""
}

func isSafeCommand(cmd string) bool {
	// git status, git log (summary), git branch, go build, npm run check are always safe
	if strings.HasPrefix(cmd, "git status") || strings.HasPrefix(cmd, "git branch") || strings.HasPrefix(cmd, "git commit") {
		return true
	}
	if strings.HasPrefix(cmd, "go build") || strings.HasPrefix(cmd, "go vet") {
		return true
	}
	return false
}

func isCommandCalling(cmdLine string, targetCmd string) bool {
	parts := strings.Split(cmdLine, "|")
	for _, part := range parts {
		fields := strings.Fields(strings.TrimSpace(part))
		if len(fields) > 0 {
			cmd := filepath.Base(fields[0])
			if cmd == targetCmd {
				return true
			}
		}
	}
	return false
}

func extractArgsAfter(cmdLine string, targetCmd string) []string {
	var args []string
	parts := strings.Split(cmdLine, "|")
	for _, part := range parts {
		fields := strings.Fields(strings.TrimSpace(part))
		if len(fields) > 0 && filepath.Base(fields[0]) == targetCmd {
			for _, f := range fields[1:] {
				if !strings.HasPrefix(f, "-") {
					args = append(args, f)
				}
			}
		}
	}
	return args
}

func getRuleMessage(policy *Policy, ruleName string, fallback string) string {
	for _, r := range policy.Rules {
		if r.Name == ruleName && r.Message != "" {
			return r.Message
		}
	}
	return fallback
}

func outputAllow() {
	fmt.Println(`{"decision": "allow"}`)
	os.Exit(0)
}

func outputDeny(reason string) {
	resp := map[string]string{
		"decision": "deny",
		"reason":   reason,
	}
	bytes, _ := json.Marshal(resp)
	fmt.Println(string(bytes))
	fmt.Fprintln(os.Stderr, reason)
	os.Exit(0)
}

func handleSubcommands(workspaceRoot string, args []string) {
	cmd := strings.ToLower(args[0])

	switch cmd {
	case "dev", "test", "plan", "all":
		mode := strings.ToUpper(cmd)
		modeFile := filepath.Join(workspaceRoot, ".agents", "current_mode")
		_ = os.MkdirAll(filepath.Dir(modeFile), 0755)
		if err := os.WriteFile(modeFile, []byte(mode+"\n"), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing mode file: %v\n", err)
			os.Exit(1)
		}

		// If running inside an active agent session, also write session-specific override
		sessID := os.Getenv("SESSION_ID")
		if sessID == "" {
			sessID = os.Getenv("CONVERSATION_ID")
		}
		if sessID != "" {
			_ = os.WriteFile(filepath.Join("/tmp", fmt.Sprintf("aicc-session-%s.mode", sessID)), []byte(mode+"\n"), 0644)
		}
		fmt.Printf("✅ [guard] 모드가 성공적으로 변경되었습니다: %s\n", mode)
		switch mode {
		case "DEV":
			fmt.Println("   • 개발(Cleanroom) 모드: 테스트 파일(*_test.go) 열람이 물리 차단됩니다.")
		case "TEST":
			fmt.Println("   • 테스트(TDD) 모드: 프로덕션 코드 열람이 차단되며, 테스트 코드만 열람/수정 가능합니다.")
		case "PLAN":
			fmt.Println("   • 기획/설계 모드: 프로덕션 코드 수정이 차단되며, 문서 파일만 작성 가능합니다.")
		case "ALL":
			fmt.Println("   • 전체 허용 모드: 모든 파일 열람 및 수정 제한이 해제됩니다.")
		}

	case "status":
		mode := determineMode(workspaceRoot, "")
		fmt.Printf("🛡️  [guard] 현재 활성 모드: %s\n", mode)
		fmt.Printf("   • 워크스페이스: %s\n", workspaceRoot)
		policyFile := filepath.Join(workspaceRoot, ".agents", "guard-policy.yaml")
		if _, err := os.Stat(policyFile); err == nil {
			fmt.Printf("   • 정책 파일: %s (로드 완료)\n", policyFile)
		} else {
			fmt.Printf("   • 정책 파일: 기본 내장 규칙 사용 중\n")
		}

	case "run":
		if len(args) < 3 {
			fmt.Println("사용법: guard run <dev|test|plan|all> <command...>")
			os.Exit(1)
		}
		subMode := strings.ToUpper(args[1])
		targetCmd := args[2]
		cmdArgs := args[3:]

		c := exec.Command(targetCmd, cmdArgs...)
		c.Env = append(os.Environ(), "AGENT_MODE="+subMode)
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		if err := c.Run(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				os.Exit(exitErr.ExitCode())
			}
			os.Exit(1)
		}

	case "help", "-h", "--help":
		printHelp()

	default:
		fmt.Fprintf(os.Stderr, "알 수 없는 서브커맨드: %s\n\n", cmd)
		printHelp()
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println("🛡️  Guard - AI Agent R/W Guardrail CLI")
	fmt.Println("사용법:")
	fmt.Println("  guard dev               # DEV 모드로 전환 (테스트 파일 열람 차단)")
	fmt.Println("  guard test              # TEST 모드로 전환 (프로덕션 코드 열람 차단)")
	fmt.Println("  guard plan              # PLAN 모드로 전환 (프로덕션 코드 수정 차단)")
	fmt.Println("  guard all               # ALL 모드로 전환 (모든 제한 해제)")
	fmt.Println("  guard status            # 현재 모드 및 정책 상태 확인")
	fmt.Println("  guard run <mode> <cmd>  # 특정 모드 환경변수를 주입하여 명령어 실행")
	fmt.Println("  guard (stdin)           # Antigravity Hook 모드 (자동 실행)")
}
