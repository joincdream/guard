package main

import "guard/internal/i18n"

// PolicyDoc returns the localized AI policy specification and Few-Shot template
// according to the active i18n language ("ko" or "en").
func PolicyDoc() string {
	if i18n.Lang() == "ko" {
		return policyDocSpecKO
	}
	return policyDocSpecEN
}

const policyDocSpecEN = `# Guard Policy Specification & Few-Shot Sample (guard-policy.yaml)

## 1. Policy Lookup Priority
guard loads the first valid guard-policy.yaml found in the following order:
  1. $GUARD_POLICY_PATH
  2. ./guard-policy.yaml (current working directory)
  3. <workspace>/.agents/guard-policy.yaml
  4. ~/.config/guard/guard-policy.yaml
  5. Built-in fallback rules

## 2. Rule Schema & Pattern Syntax
- name            : Unique rule identifier (string)
- mode            : "DEV" | "PLAN" | "TEST" | "" (empty matches all modes except ALL/QA)
- type            : "read" (blocks read_file & shell readers) | "write" (blocks write_file, edit_file, >, >>, tee, sed -i) | "command" (blocks matching shell commands & subshell wrappers, alias: "exec")
- patterns        : List of file path or command patterns to block
    * Suffix match       : "*_test.go", "*.spec.ts", "*.env"
    * Standard glob      : "*.ts", "config/*.json"
    * Multi-dir wildcard : "**/tests/**", "**/testutil/**", "src/internal/**"
    * Command match      : "go test*", "make test*", "pytest" (for type: "command"; plain prefix or '*'/'?' wildcard)
- exclude_patterns: (Optional) Patterns exempted even when matched by patterns (e.g. "go test -list*")
- message         : Violation message returned to the agent on deny

## 3. Sample guard-policy.yaml (Few-Shot Template)
` + "```yaml" + `
version: "1.0"
rules:
  # 1. DEV (Cleanroom): Block reading test files to prevent test cheating
  - name: "guard-test-files-read"
    mode: "DEV"
    type: "read"
    patterns:
      - "*_test.go"
      - "*.test.ts"
      - "*.spec.ts"
      - "**/tests/**"
      - "**/testutil/**"
    message: "[POLICY VIOLATION] Reading test files is forbidden in DEV mode. Refer only to specifications and compile checks (go build, go vet)."

  # 1-2. DEV (Cleanroom): Block running test commands to prevent output-driven overfitting
  - name: "guard-test-command-exec"
    mode: "DEV"
    type: "command"
    patterns:
      - "go test*"
      - "make test*"
    message: "[POLICY VIOLATION] Running test commands (go test, make test) is forbidden in DEV mode. Run compile checks (go build, go vet) and hand off verification to TEST mode."

  # 2. PLAN (Design): Block modifying production source code during planning
  - name: "guard-source-code-write"
    mode: "PLAN"
    type: "write"
    patterns:
      - "src/internal/**"
      - "src/cmd/**"
      - "src/web/**"
    message: "[POLICY VIOLATION] Modifying production source code is forbidden in PLAN mode. Write documentation files only."

  # 3. TEST (TDD): Block reading production code except tests and contracts
  - name: "guard-production-code-read"
    mode: "TEST"
    type: "read"
    patterns:
      - "src/internal/**"
      - "src/cmd/**"
    exclude_patterns:
      - "*_test.go"
      - "**/testutil/**"
      - "**/contract/**"
    message: "[POLICY VIOLATION] Reading production implementation code is forbidden in TEST mode. Only contracts and test files are allowed."

  # 4. Global Secret Guard (applies to DEV/PLAN/TEST when mode is empty)
  - name: "guard-secrets-read"
    mode: ""
    type: "read"
    patterns:
      - "*.env"
      - "**/.env*"
      - "**/secrets/**"
    message: "[POLICY VIOLATION] Accessing sensitive configuration (.env, secrets) files is forbidden."
` + "```" + `

## 4. Quick Verification After Generating guard-policy.yaml
  # Check which policy file is loaded:
  guard status

  # Simulate PreToolUse hook checks (returns {"decision":"deny",...} or {"decision":"allow"}):
  echo '{"sessionId":"verify","tool_name":"read_file","tool_args":{"Path":"pkg/foo_test.go"}}' | GUARD_SESSION_ID="" AGENT_MODE=DEV guard hook
  echo '{"sessionId":"verify","tool_name":"run_command","tool_args":{"CommandLine":"go test ./..."}}' | GUARD_SESSION_ID="" AGENT_MODE=DEV guard hook
  echo '{"sessionId":"verify","tool_name":"write_file","tool_args":{"Path":"src/internal/app.go"}}' | GUARD_SESSION_ID="" AGENT_MODE=PLAN guard hook
`

const policyDocSpecKO = `# Guard 정책 명세 및 Few-Shot 샘플 (guard-policy.yaml)

## 1. 정책 파일 탐색 우선순위 (Policy Lookup Priority)
guard는 다음 순서로 가장 먼저 발견된 유효한 guard-policy.yaml을 로드합니다:
  1. $GUARD_POLICY_PATH 환경변수
  2. ./guard-policy.yaml (현재 작업 디렉터리)
  3. <workspace>/.agents/guard-policy.yaml
  4. ~/.config/guard/guard-policy.yaml
  5. 내장 기본 정책 (Built-in fallback)

## 2. 규칙 스키마 및 패턴 문법 (Rule Schema & Pattern Syntax)
- name            : 규칙 고유 식별자 (문자열)
- mode            : "DEV" | "PLAN" | "TEST" | "" (비워두면 ALL/QA를 제외한 모든 모드에 적용)
- type            : "read" (read_file 및 셸 열람 차단) | "write" (write_file, edit_file, >, >>, tee, sed -i 차단) | "command" (일치하는 셸 명령어 및 서브셸 래퍼 실행 차단, 별칭: "exec")
- patterns        : 차단할 파일 경로 또는 명령어 패턴 목록
    * 접미사 매칭        : "*_test.go", "*.spec.ts", "*.env"
    * 기본 Glob          : "*.ts", "config/*.json"
    * 다중 디렉터리 매칭 : "**/tests/**", "**/testutil/**", "src/internal/**"
    * 명령어 매칭        : "go test*", "make test*", "pytest" (type: "command" 전용; 접두 명령어 또는 '*'/'?' 와일드카드)
- exclude_patterns: (선택) patterns에 매칭되더라도 예외로 허용할 패턴 (예: "go test -list*")
- message         : 차단(deny) 시 에이전트에게 반환할 위반 안내 메시지

## 3. guard-policy.yaml 완성형 샘플 (Few-Shot Template)
` + "```yaml" + `
version: "1.0"
rules:
  # 1. DEV (Cleanroom): 테스트 치팅 방지를 위한 테스트 파일 열람 차단
  - name: "guard-test-files-read"
    mode: "DEV"
    type: "read"
    patterns:
      - "*_test.go"
      - "*.test.ts"
      - "*.spec.ts"
      - "**/tests/**"
      - "**/testutil/**"
    message: "[POLICY VIOLATION] DEV 모드에서는 테스트 파일 열람이 금지됩니다. 명세서와 컴파일 검증(go build, go vet)만 참조하세요."

  # 1-2. DEV (Cleanroom): 테스트 출력 기반 땜빵 코드 방지를 위한 테스트 명령어 실행 차단
  - name: "guard-test-command-exec"
    mode: "DEV"
    type: "command"
    patterns:
      - "go test*"
      - "make test*"
    message: "[POLICY VIOLATION] DEV 모드에서는 테스트 명령어(go test, make test 등)를 직접 실행할 수 없습니다. 컴파일 검증(go build, go vet)만 수행하고 검증은 TEST 모드로 핸드오프하세요."

  # 2. PLAN (Design): 설계 단계에서 프로덕션 소스 코드 임의 수정 차단
  - name: "guard-source-code-write"
    mode: "PLAN"
    type: "write"
    patterns:
      - "src/internal/**"
      - "src/cmd/**"
      - "src/web/**"
    message: "[POLICY VIOLATION] PLAN 모드에서는 프로덕션 소스 코드를 수정할 수 없습니다. 문서 파일만 작성하세요."

  # 3. TEST (TDD): 계약 및 테스트 파일 외 프로덕션 구현 코드 열람 차단
  - name: "guard-production-code-read"
    mode: "TEST"
    type: "read"
    patterns:
      - "src/internal/**"
      - "src/cmd/**"
    exclude_patterns:
      - "*_test.go"
      - "**/testutil/**"
      - "**/contract/**"
    message: "[POLICY VIOLATION] TEST 모드에서는 프로덕션 구현 코드 열람이 금지됩니다. 계약 및 테스트 파일만 허용됩니다."

  # 4. 전역 민감 파일 보호 (mode가 비어 있으면 DEV/PLAN/TEST 공통 적용)
  - name: "guard-secrets-read"
    mode: ""
    type: "read"
    patterns:
      - "*.env"
      - "**/.env*"
      - "**/secrets/**"
    message: "[POLICY VIOLATION] 민감 설정(.env, secrets) 파일 접근이 금지됩니다."
` + "```" + `

## 4. 정책 작성 후 즉시 검증 방법 (Quick Verification)
  # 현재 로드된 정책 파일 및 규칙 개수 확인:
  guard status

  # PreToolUse 훅 판정 시뮬레이션 ({"decision":"deny",...} 또는 {"decision":"allow"} 반환 확인):
  echo '{"sessionId":"verify","tool_name":"read_file","tool_args":{"Path":"pkg/foo_test.go"}}' | GUARD_SESSION_ID="" AGENT_MODE=DEV guard hook
  echo '{"sessionId":"verify","tool_name":"run_command","tool_args":{"CommandLine":"go test ./..."}}' | GUARD_SESSION_ID="" AGENT_MODE=DEV guard hook
  echo '{"sessionId":"verify","tool_name":"write_file","tool_args":{"Path":"src/internal/app.go"}}' | GUARD_SESSION_ID="" AGENT_MODE=PLAN guard hook
`
