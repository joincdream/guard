# 커스텀 모드 확장 개발 계획 (Dynamic Mode Support)

## 1. 배경 및 목표

현재 `guard`의 코어 판정 엔진(`internal/core`)과 워크스페이스 모드 해석(`internal/hub/policy`), 런처 배너(`internal/adapters/launcher`), TUI 배지(`internal/adapters/tui`)는 임의의 모드 문자열을 처리할 수 있도록 구현되어 있다.
반면 CLI 라우팅(`cmd/guard/main.go`), 핸드오프 검증(`internal/hub/handoff/model.go`), TUI 단축키(`internal/adapters/tui/update.go`)에는 기본 모드(`DEV`, `TEST`, `PLAN`, `ALL`)가 하드코딩되어 있다.

본 계획의 목표는 다음과 같다:
* `DEV`, `TEST`, `PLAN`, `ALL`을 코드상 특수 분기가 아닌 **정책(`guard-policy.yaml` 및 내장 기본 정책)에서 로드되는 기본 모드**로 통합한다.
* 사용자가 `guard-policy.yaml`의 `rules[].mode`에 새 모드를 추가하면 별도의 코드 수정이나 재빌드 없이 **CLI 실행(`guard <mode>`), 모드 전환, 핸드오프(`guard handoff`), 가드레일 판정, TUI 전환**이 모두 동작하도록 한다.

---

## 2. 단계별 구현 계획

### Phase 1. 코어 모드 추출 헬퍼 추가 (`internal/core`)
* **대상 파일**: `internal/core/evaluator.go`
* **작업 내용**:
  1. `ExtractModes(p *Policy) []string` 및 `HasMode(p *Policy, mode string) bool` 함수를 추가한다.
     * `p.Rules`의 각 `rule.Mode`를 `NormalizeMode`로 정규화·중복 제거하여 수집한다.
     * 규칙이 존재하지 않는 바이패스 모드(`ModeAll`)는 기본으로 포함한다.
  2. 기존 `ModeDev`, `ModeTest`, `ModePlan`, `ModeAll`, `ModeQA` 상수는 내장 정책(`DefaultPolicy`) 및 `IsBypassMode` 호환용으로만 유지한다.
* **단위 테스트**:
  * 커스텀 모드(`REVIEW`, `SECURITY` 등)가 포함된 `Policy`에서 `ExtractModes`, `HasMode`, `EvaluateRequest` 동작 검증.

---

### Phase 2. 핸드오프 동적 모드 지원 (`internal/hub/handoff`, `cmd/guard`)
* **대상 파일**:
  * `internal/hub/handoff/model.go`
  * `cmd/guard/handoff.go`
* **설계 주의사항**:
  * `ParseTarget`은 대상이 **모드명**이 아니면 **세션 ID**로 간주하므로, 모든 문자열을 모드로 허용하면 세션 ID 라우팅이 깨진다.
* **작업 내용**:
  1. `ParseTargetWithModes(raw string, knownModes map[string]bool)`(또는 유사 시그니처)를 도입해 `knownModes`에 포함된 이름만 `HandoffNode{Mode: ...}`로, 나머지는 `HandoffNode{SessionID: ...}`로 파싱한다.
  2. `ValidateAsTarget` / `ValidateAsSender`에서 하드코딩된 `ValidAgentMode` 체크 대신 안전한 식별자(`ValidSafeID`) 또는 `knownModes` 기반 검증을 사용하도록 확장한다.
  3. `cmd/guard/handoff.go`의 `runHandoffSend` 및 `resolveSenderNode`에서 `policy.LoadDetailed(a.workspaceRoot)`로 추출한 모드 목록을 전달하도록 연결한다.
* **단위 테스트**:
  * 커스텀 모드(`--to review`)와 세션 ID(`--to sess-abc123`)가 충돌 없이 각각 모드와 세션으로 파싱되는지 검증.

---

### Phase 3. CLI 동적 라우팅 (`cmd/guard/main.go`)
* **대상 파일**: `cmd/guard/main.go`
* **작업 내용**:
  1. 현재 고정된 `case "dev", "test", "plan", "all":` 분기 대신 `default:` 분기에서 `policy.LoadDetailed(a.workspaceRoot)`로 정책을 로드해 `core.HasMode(polRes.Policy, cmd)` 여부를 확인한다.
     * 등록된 모드이면 `len(args) > 1`일 때 `l.Run(mode, ...)`, 인자가 없으면 `a.switchMode(mode)`를 실행한다.
     * 등록된 모드가 아니면 기존처럼 `exec.LookPath` → `unknown_subcommand` 순서로 처리하여 오타 명령어를 계속 차단한다.
  2. `switchMode`에 `default:` 분기를 추가해 커스텀 모드 전환 시에도 일반 안내 메시지를 출력한다.
  3. *(선택)* 실행 파일 이름과 겹치거나 정책 작성 전 임시 모드를 사용할 경우를 대비해 `guard mode <name> [cmd...]` 명시적 서브커맨드 추가를 검토한다.

---

### Phase 4. TUI 모드 전환 보완 (`internal/adapters/tui`)
* **대상 파일**: `internal/adapters/tui/update.go`
* **작업 내용**:
  1. `internal/adapters/tui/styles.go`의 `renderModeBadge`는 이미 `default:` 분기로 커스텀 모드를 `[NAME]` 형태로 표시하므로 수정을 생략한다.
  2. `internal/adapters/tui/update.go`의 `D/T/P/A` 단축키는 유지하되, 정책의 `ExtractModes` 목록을 순환 전환하는 키(예: `m`/`M`)를 추가한다.

---

## 3. 검증 계획

1. **기존 회귀 테스트**: `go test ./...` 전체 통과 확인.
2. **커스텀 모드 E2E 시나리오**:
   * `guard-policy.yaml`에 `mode: "REVIEW"` 규칙 추가
   * `guard review` (모드 전환) 및 `guard review <cmd>` (런처 실행 및 정책 차단) 동작 확인
   * `guard handoff send --to review -m "..."` → `approve` → `inbox --mode review` 전달 확인
   * 오타 명령(`guard invalidcmd`) 에러 반환 및 세션 ID 대상(`guard handoff send --to <sessionId>`) 정상 파싱 확인
