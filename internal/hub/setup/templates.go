package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const WorkflowManagedMarker = "<!-- guard-managed: v1 -->"

const workflowRuleTemplate = `---
trigger: always_on
---

<!-- guard-managed: v1 -->

# Guard Handoff Workflow Rule

이 워크스페이스는 Guard 범용 핸드오프 프로토콜(` + "`guard handoff`" + `)을 사용합니다.

## 규칙
1. **수신 핸드오프 확인**:
   - 환경변수 ` + "`GUARD_INBOX_PATH`" + `가 가리키는 인박스 파일 또는 ` + "`guard handoff inbox --json`" + `으로 전달된 ` + "`GUARD_HANDOFF_ENVELOPE`" + ` 메시지를 확인합니다.
   - 활성 세션 상태는 ` + "`guard status`" + `로 확인합니다.
   - 참조 문서(` + "`References`" + `)를 근거로 작업하며, 수정 허용 대상(` + "`Targets`" + `)이 지정된 경우 해당 범위 안에서만 수정합니다.
2. **작업 완료 처리 (` + "`ack`" + `)**:
   - 수신한 핸드오프 티켓의 작업을 완료하면 반드시 ` + "`guard handoff ack <id>`" + `를 실행해 완료 상태로 갱신합니다.
3. **다른 세션으로 작업 위임 (` + "`send`" + `)**:
   - 다른 모드(` + "`plan`" + `, ` + "`dev`" + `, ` + "`test`" + `)로 작업을 넘겨야 할 때 다음 명령으로 티켓을 발행합니다:
     ` + "`guard handoff send --to <plan|dev|test|all|sessionId> --type <task|verify|fix|review> [--ref <file>] [--targets <glob>] [--parent <id>] -m \"<summary>\" [--payload \"<text>\"]`" + `
   - 이전 티켓에 대한 회신·재검증 요청 시 ` + "`--parent <이전_id>`" + `를 함께 전달합니다.
4. **승인 권한 경계**:
   - ` + "`guard handoff approve`" + `는 사용자 전용 명령이므로 에이전트가 직접 실행하지 않습니다.
`

// WorkflowRuleContent returns the built-in Always-On guard-workflow.md template.
func WorkflowRuleContent() string {
	return workflowRuleTemplate
}

// HasAlwaysOnFrontmatter reports whether content begins with a YAML frontmatter block
// containing `trigger: always_on`.
func HasAlwaysOnFrontmatter(content string) bool {
	lines, endIdx, ok := parseFrontmatterBlock(content)
	if !ok {
		return false
	}
	for _, line := range lines[1:endIdx] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		k, v, found := strings.Cut(trimmed, ":")
		if !found {
			continue
		}
		if strings.TrimSpace(k) == "trigger" {
			val := strings.TrimSpace(v)
			// Strip optional inline comment and quotes.
			if idx := strings.Index(val, "#"); idx >= 0 {
				val = strings.TrimSpace(val[:idx])
			}
			val = strings.Trim(val, `"'`)
			if val == "always_on" {
				return true
			}
		}
	}
	return false
}

// InjectAlwaysOnFrontmatter ensures `trigger: always_on` is present in the leading YAML
// frontmatter block while preserving all existing frontmatter keys and document body.
func InjectAlwaysOnFrontmatter(content string) string {
	if HasAlwaysOnFrontmatter(content) {
		return content
	}

	lineEnding := "\n"
	if strings.Contains(content, "\r\n") {
		lineEnding = "\r\n"
	}

	lines, endIdx, ok := parseFrontmatterBlock(content)
	if ok {
		out := make([]string, 0, len(lines)+1)
		out = append(out, lines[:endIdx]...)
		out = append(out, "trigger: always_on")
		out = append(out, lines[endIdx:]...)
		return strings.Join(out, lineEnding)
	}

	trimmed := strings.TrimPrefix(content, "\uFEFF")
	header := "---" + lineEnding + "trigger: always_on" + lineEnding + "---" + lineEnding + lineEnding
	return header + trimmed
}

func parseFrontmatterBlock(content string) (lines []string, endIdx int, ok bool) {
	cleaned := strings.TrimPrefix(content, "\uFEFF")
	normalized := strings.ReplaceAll(cleaned, "\r\n", "\n")
	lines = strings.Split(normalized, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return nil, -1, false
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return lines, i, true
		}
	}
	return nil, -1, false
}

// EnsureWorkflowRule creates <workspaceRoot>/.agents/rules/guard-workflow.md if missing.
// Existing files are preserved untouched.
func EnsureWorkflowRule(workspaceRoot string) (rulePath string, created bool, err error) {
	if strings.TrimSpace(workspaceRoot) == "" {
		return "", false, nil
	}
	rulePath = filepath.Join(workspaceRoot, ".agents", "rules", "guard-workflow.md")
	if _, statErr := os.Stat(rulePath); statErr == nil {
		return rulePath, false, nil
	}
	if err := writeFileAtomic(rulePath, []byte(WorkflowRuleContent()), 0o644); err != nil {
		return rulePath, false, err
	}
	return rulePath, true, nil
}

// RenderPolicyYAML returns a valid guard-policy.yaml template for the given preset.
func RenderPolicyYAML(preset string) (string, error) {
	p := strings.ToLower(strings.TrimSpace(preset))
	switch p {
	case PresetGo:
		return goPolicyTemplate, nil
	case PresetNode:
		return nodePolicyTemplate, nil
	case PresetPython:
		return pythonPolicyTemplate, nil
	case PresetGeneric:
		return genericPolicyTemplate, nil
	default:
		return "", fmt.Errorf("%w: %q (supported: go, node, python, generic)", ErrInvalidPreset, preset)
	}
}

const goPolicyTemplate = `version: "1.0"
rules:
  - name: "guard-test-files-read"
    mode: "DEV"
    type: "read"
    patterns:
      - "*_test.go"
      - "**/testutil/**"
      - "**/tests/**"
    message: "[POLICY VIOLATION] DEV 모드에서는 테스트 파일 열람이 금지됩니다. 명세서와 'go test' 출력만 참조하세요."

  - name: "guard-source-code-write"
    mode: "PLAN"
    type: "write"
    patterns:
      - "cmd/**"
      - "internal/**"
      - "pkg/**"
    message: "[POLICY VIOLATION] PLAN 모드에서는 Go 소스 코드를 수정할 수 없습니다. 문서 파일만 작성하세요."

  - name: "guard-production-code-read"
    mode: "TEST"
    type: "read"
    patterns:
      - "cmd/**"
      - "internal/**"
      - "pkg/**"
    exclude_patterns:
      - "*_test.go"
      - "**/testutil/**"
      - "**/contract/**"
    message: "[POLICY VIOLATION] TEST 모드에서는 프로덕션 구현 코드 열람이 금지됩니다. 계약 및 테스트 파일만 허용됩니다."

  - name: "guard-secrets-read"
    mode: ""
    type: "read"
    patterns:
      - "*.env"
      - "**/.env*"
      - "**/secrets/**"
    message: "[POLICY VIOLATION] 민감 설정(.env, secrets) 파일 접근이 금지됩니다."
`

const nodePolicyTemplate = `version: "1.0"
rules:
  - name: "guard-test-files-read"
    mode: "DEV"
    type: "read"
    patterns:
      - "*.test.ts"
      - "*.spec.ts"
      - "*.test.js"
      - "*.spec.js"
      - "**/__tests__/**"
      - "**/tests/**"
    message: "[POLICY VIOLATION] DEV 모드에서는 테스트 파일 열람이 금지됩니다. 명세서와 테스트 실행 출력만 참조하세요."

  - name: "guard-source-code-write"
    mode: "PLAN"
    type: "write"
    patterns:
      - "src/**"
      - "app/**"
      - "lib/**"
    message: "[POLICY VIOLATION] PLAN 모드에서는 소스 코드를 수정할 수 없습니다. 문서 파일만 작성하세요."

  - name: "guard-production-code-read"
    mode: "TEST"
    type: "read"
    patterns:
      - "src/**"
      - "app/**"
      - "lib/**"
    exclude_patterns:
      - "*.test.ts"
      - "*.spec.ts"
      - "*.test.js"
      - "*.spec.js"
      - "**/__tests__/**"
      - "**/contract/**"
    message: "[POLICY VIOLATION] TEST 모드에서는 프로덕션 구현 코드 열람이 금지됩니다. 계약 및 테스트 파일만 허용됩니다."

  - name: "guard-secrets-read"
    mode: ""
    type: "read"
    patterns:
      - "*.env"
      - "**/.env*"
      - "**/secrets/**"
    message: "[POLICY VIOLATION] 민감 설정(.env, secrets) 파일 접근이 금지됩니다."
`

const pythonPolicyTemplate = `version: "1.0"
rules:
  - name: "guard-test-files-read"
    mode: "DEV"
    type: "read"
    patterns:
      - "test_*.py"
      - "*_test.py"
      - "**/tests/**"
    message: "[POLICY VIOLATION] DEV 모드에서는 테스트 파일 열람이 금지됩니다. 명세서와 pytest 출력만 참조하세요."

  - name: "guard-source-code-write"
    mode: "PLAN"
    type: "write"
    patterns:
      - "src/**"
      - "app/**"
    message: "[POLICY VIOLATION] PLAN 모드에서는 Python 소스 코드를 수정할 수 없습니다. 문서 파일만 작성하세요."

  - name: "guard-production-code-read"
    mode: "TEST"
    type: "read"
    patterns:
      - "src/**"
      - "app/**"
    exclude_patterns:
      - "test_*.py"
      - "*_test.py"
      - "**/tests/**"
      - "**/contract/**"
    message: "[POLICY VIOLATION] TEST 모드에서는 프로덕션 구현 코드 열람이 금지됩니다. 계약 및 테스트 파일만 허용됩니다."

  - name: "guard-secrets-read"
    mode: ""
    type: "read"
    patterns:
      - "*.env"
      - "**/.env*"
      - "**/secrets/**"
    message: "[POLICY VIOLATION] 민감 설정(.env, secrets) 파일 접근이 금지됩니다."
`

const genericPolicyTemplate = `version: "1.0"
rules:
  - name: "guard-test-files-read"
    mode: "DEV"
    type: "read"
    patterns:
      - "*_test.*"
      - "*.test.*"
      - "*.spec.*"
      - "**/tests/**"
    message: "[POLICY VIOLATION] DEV 모드에서는 테스트 파일 열람이 금지됩니다. 명세서와 테스트 실행 출력만 참조하세요."

  - name: "guard-source-code-write"
    mode: "PLAN"
    type: "write"
    patterns:
      - "src/**"
      - "cmd/**"
      - "internal/**"
    message: "[POLICY VIOLATION] PLAN 모드에서는 소스 코드를 수정할 수 없습니다. 문서 파일만 작성하세요."

  - name: "guard-production-code-read"
    mode: "TEST"
    type: "read"
    patterns:
      - "src/**"
      - "cmd/**"
      - "internal/**"
    exclude_patterns:
      - "*_test.*"
      - "*.test.*"
      - "*.spec.*"
      - "**/tests/**"
      - "**/contract/**"
    message: "[POLICY VIOLATION] TEST 모드에서는 프로덕션 구현 코드 열람이 금지됩니다. 계약 및 테스트 파일만 허용됩니다."

  - name: "guard-secrets-read"
    mode: ""
    type: "read"
    patterns:
      - "*.env"
      - "**/.env*"
      - "**/secrets/**"
    message: "[POLICY VIOLATION] 민감 설정(.env, secrets) 파일 접근이 금지됩니다."
`
