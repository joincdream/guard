package main

const policyDocSpec = `# Guard Policy Specification & Few-Shot Sample (guard-policy.yaml)

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
- type            : "read" (blocks read_file & shell readers) | "write" (blocks write_file, edit_file, >, >>, tee, sed -i)
- patterns        : List of path patterns to block
    * Suffix match       : "*_test.go", "*.spec.ts", "*.env"
    * Standard glob      : "*.ts", "config/*.json"
    * Multi-dir wildcard : "**/tests/**", "**/testutil/**", "src/internal/**"
- exclude_patterns: (Optional) Patterns exempted even when matched by patterns
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
    message: "[POLICY VIOLATION] DEV 모드에서는 테스트 파일 열람이 금지됩니다. 명세서와 테스트 실행 결과(stdout)만 참조하세요."

  # 2. PLAN (Design): Block modifying production source code during planning
  - name: "guard-source-code-write"
    mode: "PLAN"
    type: "write"
    patterns:
      - "src/internal/**"
      - "src/cmd/**"
      - "src/web/**"
    message: "[POLICY VIOLATION] PLAN 모드에서는 프로덕션 소스 코드를 수정할 수 없습니다. 문서 파일만 작성하세요."

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
    message: "[POLICY VIOLATION] TEST 모드에서는 프로덕션 구현 코드 열람이 금지됩니다. 계약 및 테스트 파일만 허용됩니다."

  # 4. Global Secret Guard (applies to DEV/PLAN/TEST when mode is empty)
  - name: "guard-secrets-read"
    mode: ""
    type: "read"
    patterns:
      - "*.env"
      - "**/.env*"
      - "**/secrets/**"
    message: "[POLICY VIOLATION] 민감 설정(.env, secrets) 파일 접근이 금지됩니다."
` + "```" + `

## 4. Quick Verification After Generating guard-policy.yaml
  # Check which policy file is loaded:
  guard status

  # Simulate a PreToolUse hook check (should return {"decision":"deny",...} or {"decision":"allow"}):
  echo '{"sessionId":"verify","tool_name":"read_file","tool_args":{"Path":"pkg/foo_test.go"}}' | AGENT_MODE=DEV guard
  echo '{"sessionId":"verify","tool_name":"write_file","tool_args":{"Path":"src/internal/app.go"}}' | AGENT_MODE=PLAN guard
`
