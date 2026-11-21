# Handoff Protocol 블랙박스 QA 테스트 통합 보고서 (Stage 1 ~ Stage 6)

- **작성일**: 2026-11-21
- **검증 대상**: `guard handoff` 프로토콜 (Phase 1 ~ Phase 5 구현분)
- **기준 문서**:
  - [handoff-qa-test-brief.md](./handoff-qa-test-brief.md)
  - [05_handoff_protocol_contract.md](../spec/05_handoff_protocol_contract.md)
  - [02_session_hub_contract.md](../spec/02_session_hub_contract.md)
  - [03_adapters_cli_contract.md](../spec/03_adapters_cli_contract.md)
  - [01_core_policy_contract.md](../spec/01_core_policy_contract.md)

---

## 1. 전체 실행 요약

- **총계**: **28 / 32 PASS, 4 FAIL** (신규 발견 결함 **3건**, 기존 `TC-SCOPE-05`는 명세 정정 후 재검증 **PASS**)
- **데이터 레이스**: 전 단계 `go test -race` 실행 시 레이스 미검출

| Stage | 영역 | 테스트 파일 | 실행 결과 | 비고 |
| :--- | :--- | :--- | :---: | :--- |
| **Stage 1** | `TC-STATE` (상태 머신 · 입력 검증 · 동시성 · Compaction) | `test/e2e/handoff_state_test.go` | **6 / 6 PASS** | `-race` 통과 (`3.975s`) |
| **Stage 2** | `TC-GATE` (Core 신뢰 경계 · 자체 승인 우회 공격) | `test/e2e/handoff_gate_test.go` | **4 / 4 PASS** | `-race` 통과 (`1.021s`) |
| **Stage 3** | `TC-SCOPE` (조건부 Scope Lock 매트릭스 · Envelope 계약) | `test/e2e/handoff_scope_test.go` | **8 / 8 PASS** | `TC-SCOPE-05` 명세 정정 후 재검증 통과 (`1.027s`) |
| **Stage 4** | `TC-ROUTE` (세션 라우팅 · 오프라인 버퍼링 · 런처 생명주기) | `test/e2e/handoff_route_test.go` | **5 / 6 PASS, 1 FAIL** | `TC-ROUTE-06` 결함 발견 |
| **Stage 5** | `TC-CLI-E2E` (실제 빌드 바이너리 CLI 통합 시나리오) | `test/e2e/handoff_cli_test.go` | **1 / 4 PASS, 3 FAIL** | 결함 2건 발견 (`reject` 플래그, `hook` 페이로드) |
| **Stage 6** | `TC-TUI-I18N` (관제탑 TUI 오버레이 · 대용량 방어 · 다국어) | `test/e2e/handoff_tui_i18n_test.go` | **4 / 4 PASS** | `-race` 통과 (`1.059s`) |

---

## 2. 미해결 결함 상세 (개발자 조치 필요 — 총 3건)

### [DEFECT-01] `TC-ROUTE-06`: `launcher.Run` 실행 시 `.agents/rules/guard-workflow.md` 미생성

- **심각도**: Medium
- **실패 테스트**: `TestHandoffRoute06_LauncherAutoConsumeAndCleanup` (`test/e2e/handoff_route_test.go:433`)
- **명세 기준**: `doc/test/handoff-qa-test-brief.md` L388-393 (`launcher.Run("dev", ws, []string{"sh", "-c", ...})` 실행 시 `<ws>/.agents/rules/guard-workflow.md` 자동 생성)
- **실제 동작**:
  - `exitCode == 0`, 환경변수(`AGENT_MODE=DEV`, `GUARD_SESSION_ID`, `GUARD_INBOX_PATH`) 주입, 오프라인 티켓 자동 소비 및 인박스 내용 전달, 종료 후 `.json`/`.inbox` 정리, 비TTY OSC 미출력은 모두 정상 통과
  - 그러나 `<ws>/.agents/rules/guard-workflow.md` 파일이 생성되지 않음 (`stat .../.agents/rules/guard-workflow.md: no such file or directory`)
- **재현 명령**:
  ```bash
  go test -race -v ./test/e2e/... -run "TestHandoffRoute06"
  ```
- **확인 요청**: `launcher.Run`이 에이전트 명령(`agy`)일 때만 `EnsureWorkflowRule`을 호출하도록 분기되어 있는지, 아니면 모든 `launcher.Run`에서 호출해야 하는지 확인 및 조치 부탁드립니다.

---

### [DEFECT-02] `TC-CLI-02`, `TC-CLI-03`: `guard handoff reject <id> -m "<reason>"` 위치 인자 뒤 플래그 파싱 실패

- **심각도**: High
- **실패 테스트**:
  - `TestHandoffCLI02_InboxFilteringAndJSON` (`test/e2e/handoff_cli_test.go:216`)
  - `TestHandoffCLI03_StatusTransitionsAndExitCodes` (`test/e2e/handoff_cli_test.go:287`)
- **계약/명세 기준**:
  - `doc/spec/05_handoff_protocol_contract.md` L132: `guard handoff reject <id> [-m "<reason>"]`
  - `doc/test/handoff-qa-test-brief.md` L425: `guard handoff reject <B> -m "스펙 변경"`
- **실제 동작**:
  - `guard handoff reject <id> -m "스펙 변경"` 실행 시 종료 코드 `1` 반환
  - `stderr`: `usage: guard handoff reject <id> [-m "reason"]`
  - 출력하는 usage 문법(`<id>` 뒤에 `-m`) 그대로 호출해도 Go `flag.FlagSet`이 위치 인자 뒤 플래그를 파싱하지 못해 실패하는 것으로 추정
- **재현 명령**:
  ```bash
  go test -race -v ./test/e2e/... -run "TestHandoffCLI0[23]"
  ```

---

### [DEFECT-03] `TC-CLI-04`: `guard hook` 바이너리에서 `tool_input.command` 포맷 자체 승인 차단 우회

- **심각도**: **Critical (보안 신뢰 경계 우회)**
- **실패 테스트**: `TestHandoffCLI04_HookBinarySelfApproveBlock` (`test/e2e/handoff_cli_test.go:337`)
- **명세 기준**: `doc/test/handoff-qa-test-brief.md` L93-95, L431-435
  ```bash
  echo '{"tool_name":"run_command","tool_input":{"command":"guard handoff approve hnd-1"}}' \
    | HOME="$TEST_HOME" AGENT_MODE=ALL guard hook
  # 기대 출력: {"decision":"deny","reason":"..."} + stderr 차단 사유
  ```
- **실제 동작**:
  - 종료 코드 `0`
  - `stdout`: `{"decision":"allow"}\n`
  - `stderr`: `""` (비어있음)
- **원인 추정**: `doc/spec/03_adapters_cli_contract.md` L12-28의 `hook.Input` 구조체에 `tool_input` 필드가 정의되어 있지 않아 `tool_input.command`에서 명령어를 추출하지 못하고 Fail-open(`allow`) 처리됨
- **재현 명령**:
  ```bash
  go test -race -v ./test/e2e/... -run "TestHandoffCLI04"
  ```

---

## 3. 단계별 상세 검증 결과

### Stage 1. `TC-STATE` (`go test -race -v ./test/e2e/... -run "TestHandoffState"`)

| TC ID | 테스트 함수 | 검증 내용 | 결과 |
| :--- | :--- | :--- | :---: |
| TC-STATE-01 | `TestHandoffState01_LifecycleAndIDTimestamp` | `pending → approved → acked` 전이, ID 정규식(`^hnd-\d{8}-[0-9a-f]{8}$`), `CreatedAt`/`UpdatedAt` 영속화 | **PASS** |
| TC-STATE-02 | `TestHandoffState02_RejectAndNoInboxDelivery` | `pending → rejected` 반려 사유 저장 및 활성 DEV 세션 `.inbox` 미생성 | **PASS** |
| TC-STATE-03 | `TestHandoffState03_InvalidTransitions` | 금지 상태 전이 13종 전수 `handoff.ErrInvalidTransition` 반환 및 상태/사유 불변 | **PASS** |
| TC-STATE-04 | `TestHandoffState04_ValidationAndPathTraversal` | `ParseTarget`/`Enqueue` 입력 검증(`ErrInvalidItem`), `InboxPath` 경로 탈출(`ErrInvalidSessionID`), 중복 ID 차단 및 기존 티켓 무손상 | **PASS** |
| TC-STATE-05 | `TestHandoffState05_ConcurrencyFlock` | 10고루틴×5건(50건) 동시 `Enqueue` JSON 무결성 및 10고루틴 동시 승인 경합(1건 성공, 9건 `ErrInvalidTransition`) | **PASS** |
| TC-STATE-06 | `TestHandoffState06_QueueCompaction` | 완료 180건 + 활성 50건(총 230건) Compaction 시 활성 50건 100% 보존, 전체 `<= 200`, 최오래된 완료 30건 제거 | **PASS** |

### Stage 2. `TC-GATE` (`go test -race -v ./test/e2e/... -run "TestHandoffGate"`)

| TC ID | 테스트 함수 | 검증 내용 | 결과 |
| :--- | :--- | :--- | :---: |
| TC-GATE-01 | `TestHandoffGate01_SelfApproveBlockAllModesAndPolicies` | 모드 4종(`PLAN`/`DEV`/`TEST`/`ALL`) × 정책 2종(`nil`, 전체 허용) 8개 조합 모두 `Allowed=false`, `RuleName="handoff-self-approve-block"` | **PASS** |
| TC-GATE-02 | `TestHandoffGate02_BypassAttacks` | 셸 체이닝(`&&`, `;`, `\|\|`), `bash -c`, `sh -c`, `python3 -c`, 절대/상대 경로, `AGENT_MODE=ALL` 접두사 9종 우회 공격 모두 `core` 및 `hook` 차단 | **PASS** |
| TC-GATE-03 | `TestHandoffGate03_NoFalsePositive` | 정상 명령 4종(`send`, `inbox`, `ack`, `reject`) 오탐 없이 `Allowed=true` | **PASS** |
| TC-GATE-04 | `TestHandoffGate04_InboxIsolationBeforeApprove` | 승인 전 활성 `DEV` 세션 `.inbox` 파일 미생성 및 `AllowedWritePaths` 미변경 | **PASS** |

### Stage 3. `TC-SCOPE` (`go test -race -v ./test/e2e/... -run "TestHandoffScope"`)

| TC ID | 테스트 케이스 | 기대 결과 | 결과 |
| :--- | :--- | :--- | :---: |
| TC-SCOPE-01 | `fix` + `["pkg/a.go"]` + `DEV` | `ScopeLocked=true`, `AllowedWritePaths=["pkg/a.go"]`, `수정 허용 대상 (Scope Locked)` | **PASS** |
| TC-SCOPE-02 | `task` + `["pkg/b.go","pkg/c.go"]` + `ALL` | `ScopeLocked=true`, `AllowedWritePaths=["pkg/b.go","pkg/c.go"]`, `수정 허용 대상 (Scope Locked)` | **PASS** |
| TC-SCOPE-03 | `verify` + `["pkg/a.go"]` + `TEST` | `ScopeLocked=false`, 기존 스코프 불변, `작업 대상` (`Scope Locked` 미포함) | **PASS** |
| TC-SCOPE-04 | `review` + `["doc/a.md"]` + `PLAN` | `ScopeLocked=false`, 기존 스코프 불변, `작업 대상` (`Scope Locked` 미포함) | **PASS** |
| TC-SCOPE-05 | `fix` + `[]` + `DEV` | `ScopeLocked=false`, 기존 스코프 불변, `작업 대상` 섹션 생략 (`Scope Locked` 미포함) | **PASS** (명세 정정 후 재검증) |
| TC-SCOPE-06 | `fix` + `["pkg/a.go"]` + `TEST` | `ScopeLocked=false`, 기존 스코프 불변, `작업 대상` (`Scope Locked` 미포함) | **PASS** |
| TC-SCOPE-07 | `task` + `["pkg/a.go"]` + `PLAN` | `ScopeLocked=false`, 기존 스코프 불변, `작업 대상` (`Scope Locked` 미포함) | **PASS** |
| TC-SCOPE-08 | `FormatEnvelope` 필수 필드(`ID`, `Type`, `Summary`) 누락 | `handoff.ErrInvalidItem` 반환 | **PASS** |

### Stage 4. `TC-ROUTE` (`go test -race -v ./test/e2e/... -run "TestHandoffRoute"`)

| TC ID | 테스트 함수 | 검증 내용 | 결과 |
| :--- | :--- | :--- | :---: |
| TC-ROUTE-01 | `TestHandoffRoute01_ActiveSessionImmediateDelivery` | 활성 `DEV` 세션 즉시 전달, Envelope 마커·ID·요약 검증, `To.SessionID` 기록 | **PASS** |
| TC-ROUTE-02 | `TestHandoffRoute02_OfflineBufferingExcludesDeadPID` | 사망 PID(`999999`) 배제, 오프라인 `Buffered=true` 보관 및 Stale `.inbox` 미생성 | **PASS** |
| TC-ROUTE-03 | `TestHandoffRoute03_MissingTargetSessionRollback` | 미존재 `SessionID` 승인 실패 시 `pending` 상태 원자적 롤백 | **PASS** |
| TC-ROUTE-04 | `TestHandoffRoute04_ConcurrentConsumeNoDuplicate` | 5개 세션 동시 `ConsumePendingForMode` 경합 시 정확히 1개 세션만 소비 및 1개 `.inbox` 생성 | **PASS** |
| TC-ROUTE-05 | `TestHandoffRoute05_AlreadyDeliveredNotReconsumed` | 기전달 티켓(`To.SessionID != ""`) 신규 세션 재소비 차단 | **PASS** |
| TC-ROUTE-06 | `TestHandoffRoute06_LauncherAutoConsumeAndCleanup` | 오프라인 소비·환경 주입·종료 정리·비TTY OSC 미출력 통과, `.agents/rules/guard-workflow.md` 미생성 | **FAIL** (`DEFECT-01`) |

### Stage 5. `TC-CLI-E2E` (`go test -race -v ./test/e2e/... -run "TestHandoffCLI"`)

| TC ID | 테스트 함수 | 검증 내용 | 결과 |
| :--- | :--- | :--- | :---: |
| TC-CLI-01 | `TestHandoffCLI01_SendFlagsAndValidation` | `handoff send` 반복 `--ref`, 쉼표 `--targets`, 필수값 누락/경로 탈출(`../bad`) 종료 코드 `1` | **PASS** |
| TC-CLI-02 | `TestHandoffCLI02_InboxFilteringAndJSON` | `handoff inbox --json` / `--all` / `--mode dev` 필터링 (사전 셋업 `reject <id> -m` 실패) | **FAIL** (`DEFECT-02`) |
| TC-CLI-03 | `TestHandoffCLI03_StatusTransitionsAndExitCodes` | `approve`/`reject`/`ack` 상태 전이 및 오류 종료 코드 (`reject <id> -m` 실패) | **FAIL** (`DEFECT-02`) |
| TC-CLI-04 | `TestHandoffCLI04_HookBinarySelfApproveBlock` | `guard hook` 바이너리 `tool_input.command` 자체 승인 차단 (`allow` 반환) | **FAIL** (`DEFECT-03`) |

### Stage 6. `TC-TUI-I18N` (`go test -race -v ./test/e2e/... -run "TestHandoffTUI"`)

| TC ID | 테스트 함수 | 검증 내용 | 결과 |
| :--- | :--- | :--- | :---: |
| TC-TUI-01 | `TestHandoffTUI01_ModalAndPanelToggle` | `[H]` 하단 패널 토글, 모달 `[Enter]` 승인(`approved`), `[Esc]` 반려(`rejected`) | **PASS** |
| TC-TUI-02 | `TestHandoffTUI02_OverlayPriorityNoSteal` | `?` 도움말 및 `Space` 이벤트 상세 모달 활성 중 신규 `pending` 티켓 도착 시 포커스 가로채기 방지 | **PASS** |
| TC-TUI-03 | `TestHandoffTUI03_LargeRefDocViewerCap` | 300KB 참조 문서 `[V]` 뷰어 256KB 상한 방어 및 `tui.handoff.ref_truncated` 안내 출력 | **PASS** |
| TC-TUI-04 | `TestHandoffTUI04_I18nCatalogAndLocale` | `ko.json`/`en.json` 전체 키 집합·포맷 동사 100% 일치, 로케일 우선순위(`GUARD_LANG > LC_ALL > LC_MESSAGES > LANG > en`) 및 `C`/`POSIX` 폴백 | **PASS** |

---

## 4. 조치 완료 이력

### [RESOLVED] `TC-SCOPE-05` (2026-11-21)
- **판정**: 코드 결함이 아닌 명세서 기술 누락 (`Targets`/`References`/`Payload` 비어 있을 때 섹션 생략이 의도된 동작)
- **개발자 조치**: `doc/spec/05_handoff_protocol_contract.md` (L97) 및 `doc/test/handoff-qa-test-brief.md` (L332) 정정 완료
- **QA 재검증**: `test/e2e/handoff_scope_test.go` 기대값을 `작업 대상` 및 `Scope Locked` 모두 미포함으로 갱신 후 `go test -race -v ./test/e2e/... -run "TestHandoffScope"` **8 / 8 PASS** 확인

---

## 5. 개발자 검토 및 조치 결과 — `DEFECT-01` ~ `DEFECT-03` (2026-11-21)

### [FIXED / SPECIFIED] `DEFECT-01` (`TC-ROUTE-06`): `agy` 전용 워크스페이스 부트스트랩 명세 명확화
- **판정**: 의도된 동작 (`EnsureWorkspaceHook`, `EnsureWorkflowRule` 모두 일반 셸 명령 실행 시 워크스페이스 `.agents/` 오염을 방지하기 위해 `filepath.Base(cmdArgs[0]) == "agy"`일 때만 자동 실행 — 참조: `internal/adapters/launcher/launcher.go:86-97`)
- **조치 내역**:
  1. `doc/spec/03_adapters_cli_contract.md` (L76-77): `launcher.Run` 계약에 `agy` 실행 시 한정 자동 부트스트랩 조건 명시
  2. `doc/test/handoff-qa-test-brief.md` (L393): `TC-ROUTE-06`의 `.agents/rules/guard-workflow.md` 생성 검증을 `agy` 한정 동작으로 정정 (`EnsureWorkflowRule` 직접 호출 또는 `agy` 실행 파일 기준 검증)

### [FIXED] `DEFECT-02` (`TC-CLI-02`, `TC-CLI-03`): `guard handoff reject <id> -m "<reason>"` 위치 인자 뒤 플래그 파싱 수정
- **원인**: Go 표준 `flag.FlagSet.Parse`가 위치 인자(`<id>`) 이후의 `-m`/`--reason` 플래그를 파싱하지 못함
- **조치 내역**:
  - `cmd/guard/handoff.go` (L253-297): `normalizeRejectArgs` 추가하여 `guard handoff reject <id> -m "<reason>"` 및 `guard handoff reject -m "<reason>" <id>` 모두 정상 파싱되도록 수정

### [FIXED] `DEFECT-03` (`TC-CLI-04`): `guard hook` `tool_input.command` 포맷 지원 추가 (보안 신뢰 경계 보완)
- **원인**: `hook.Input`에 `tool_input` 필드가 누락되어 `tool_input.command` 추출 실패 및 Fail-open(`allow`) 발생
- **조치 내역**:
  1. `internal/adapters/hook/payload.go` (L16-80): `Input.ToolInput` (`json:"tool_input"`) 추가 및 `ExtractCommandLine()`(`CommandLine`, `command_line`, `command`, `cmd`), `ExtractTargetPath()`(`Path`, `path`, `file_path`)에서 `tool_input` 추출 지원
  2. `doc/spec/03_adapters_cli_contract.md` (L22-26): `hook.Input` DTO 계약에 `ToolInput` 필드 반영

### QA 재검증 요청
- `DEFECT-02`, `DEFECT-03` 수정 코드 기준 `TC-CLI-02`, `TC-CLI-03`, `TC-CLI-04` 재검증 및 `DEFECT-01` 정정 명세 반영 후 `TC-ROUTE-06` 재검증 부탁드립니다.
