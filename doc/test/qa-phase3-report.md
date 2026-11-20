# QA Phase 3 블랙박스 테스트 결과 및 수정 요청 보고서

- **작성일**: 2026-11-20
- **작성자**: QA
- **기준 시나리오**: [test-scenario-plan.md](./test-scenario-plan.md) (`Phase 3`)
- **실행 명령**: `go test -v -race -count=1 ./internal/adapters/hook/... ./internal/adapters/launcher/...`
- **테스트 환경**: Linux / Go Test (`-race` 포함) / guard `TEST` 모드

---

## 1. 실행 요약

| 단계 | 패키지 | 결과 | 소요 시간 | 비고 |
| :--- | :--- | :---: | :---: | :--- |
| **Phase 3** | `guard/internal/adapters/hook` | **FAIL** | 0.029s | 기존 2건 + `TC-P3-001`~`005` + fallback 통과 / **1건 실패 (`DEF-004`)** |
| **Phase 3** | `guard/internal/adapters/launcher` | **PASS** | 1.029s | 기존 3건 + `TC-P3-007`~`013` 전체 통과 (레이스 0건) |

---

## 2. 발견 이슈 상세 (`DEF-004`)

### DEF-004: `GUARD_DEBUG=1` 설정 후 정상 `Handle()` 실행 시 `stderr`에 `[guard:debug]` 로그가 출력되지 않음

- **심각도 / 우선순위**: Low / P3 (진단 로그 계약 불일치 또는 `io.Writer` 미사용)
- **대상 패키지**: `guard/internal/adapters/hook`
- **관련 테스트**: `internal/adapters/hook/blackbox_phase3_test.go` (`L288-L305`, `TestPhase3_Hook_DebugLogToggle/GUARD_DEBUG_1_writes_debug_prefix_to_stderr` / `TC-P3-006`)
- **단독 재현 명령**:
  ```bash
  go test -v -race -count=1 ./internal/adapters/hook -run TestPhase3_Hook_DebugLogToggle
  ```

#### 재현 절차 및 입력 데이터
1. 환경변수 설정: `AGENT_MODE=ALL`, `GUARD_DEBUG=1` (기타 guard 환경변수 빈 값 격리)
2. `var stdout, stderr bytes.Buffer`를 주입해 `hook.NewHandler(mgr, t.TempDir(), &stdout, &stderr)` 생성
3. 정상 JSON 페이로드로 `h.Handle(...)` 호출:
   ```json
   {"tool_name":"read_file","tool_args":{"Path":"pkg/main.go"}}
   ```

#### 기대 결과 (Expected Result)
- 계약서(`doc/spec/03_adapters_cli_contract.md` §1.2):
  > **진단 로그**: `GUARD_DEBUG=1` 설정 시에만 `stderr`에 `[guard:debug]` 로그 출력 (미설정 시 무출력).
- `stdout`에 `{"decision":"allow"}` 출력
- 주입된 `stderr` 버퍼에 `[guard:debug]` 접두사를 포함한 진단 로그 출력

#### 실제 결과 (Actual Result)
- `stdout`은 정상적으로 `{"decision":"allow"}` 출력
- 주입된 `stderr` 버퍼는 완전히 비어 있음 (`""`)
- 에러 로그:
  ```text
  blackbox_phase3_test.go:303: expected [guard:debug] in stderr when GUARD_DEBUG=1, got ""
  ```

#### 원인 추정 및 개발자 확인 요청 사항
- **판정**: **프로덕션 구현 또는 스펙 조건 명확화 필요**
- **확인 포인트**:
  - [ ] `GUARD_DEBUG=1` 로그가 주입된 `h.stderr`(`io.Writer`)가 아닌 `os.Stderr`로 직접 출력되고 있는지 확인
  - [ ] `[guard:debug]` 로그가 모든 `Handle()` 호출 시 출력되는 스펙인지, 아니면 **예외/경고/실패 상황(예: 깨진 JSON, 세션 저장 실패, 정책 파싱 경고)**에만 출력되는 스펙인지 확인
  - [ ] 예외/경고 시에만 출력되는 사양이라면 `doc/spec/03_adapters_cli_contract.md` §1.2에 정확한 트리거 조건을 명시해 주시면 QA 테스트(`TC-P3-006`)를 해당 조건에 맞게 갱신하겠습니다.

---

## 3. 참고 — 정상 검증 완료된 주요 항목 (Phase 3)
- **`internal/adapters/hook`**:
  - Fail-open 계약 (빈 입력/공백/깨진 JSON/`null` → `{"decision":"allow"}`)
  - 6종 JSON 입력 스키마 호환성 (`toolCall`, `tool_name`, 최상위 `Path`/`path`/`CommandLine`/`command_line`, `sessionId`/`session_id`/`conversationId`)
  - 외부 미존재 `GUARD_SESSION_ID` + `AGENT_MODE=TEST` 환경에서 페이로드 `sessionId`(`DEV`) 우선 매칭 (`DEF-001` 회귀 방지)
  - 미등록 세션 폴백 평가 시 `PID: 0` 세션 파일 비생성 계약
  - `deny` 시 `stdout` JSON + `stderr` 차단 사유 1줄 출력, `.agents/current_mode` 폴백
- **`internal/adapters/launcher`**:
  - 빈 `cmdArgs` 방어(종료 코드 `1`), 자식 프로세스 `AGENT_MODE`/`GUARD_SESSION_ID` 주입 및 실제 자식 PID(`>0`) 세션 등록 + 종료 시 `defer Delete` 정리
  - 비정상 종료 코드(`42`) 전파 및 세션 정리
  - 비-TTY(`bytes.Buffer`) `stdout`에 ANSI OSC 타이틀 시퀀스(`\033]0;`) 미혼입
  - 배너 `⚠️  [Policy Warning]` 출력, `EnsureAgentSkill` 기존 파일 보존
  - `EnsureWorkspaceHook` 기존 키(`obsidian-chatlog`) 100% 보존 병합 및 손상 JSON 덮어쓰기 방어

---

## 4. 개발자 검토 및 수정 회신 (2026-11-20)

### `DEF-004` — 수정 완료 (`internal/adapters/hook/handler.go`)
- **원인 확인**: `h.debugf`는 주입된 `h.stderr`를 정상 사용하고 있었으나, 예외/경고 경로(깨진 JSON, 정책 파싱 경고, 이벤트 저장 실패)에서만 호출되고 정상 평가 경로에서는 호출되지 않고 있었습니다.
- **수정 내용**: 운영 진단성 향상을 위해 `Handle()`의 `core.EvaluateRequest` 직후에도 `h.debugf("evaluated mode=%s source=%s tool=%s allowed=%v", ...)`를 출력하도록 추가했습니다. (`GUARD_DEBUG=1` 미설정 시에는 기존과 동일하게 무출력 유지)
- **QA 확인 요청**: `TestPhase3_Hook_DebugLogToggle` (`TC-P3-006`) 및 `internal/adapters/hook` 전체 `-race` 재검증을 부탁드립니다.

---

## 5. QA 재검증 및 종료 보고 (2026-11-20)

- **재검증 명령**: `go test -v -race -count=1 ./internal/adapters/hook/...`
- **재검증 결과**: **PASS** (`ok guard/internal/adapters/hook 1.029s`, 종료 코드 `0`, 데이터 레이스 0건)
  - `TestPhase3_Hook_DebugLogToggle` (`TC-P3-006`) **PASS** (테스트 코드 수정 없이 통과)
  - 기존 2건 + Phase 3 블랙박스 7건 전체 9건 **PASS**
- **결함 최종 상태**:
  - **DEF-004**: **Closed** (프로덕션 수정 검증 완료)
- **Phase 3 최종 상태**: `adapters/hook`, `adapters/launcher` 전수 통과 완료
