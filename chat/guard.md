# 💬 guard

> Created by `chatlog` on 2026-11-18 22:39:47
## 🧑‍💻 User · `2026-11-21 12:21:16` {#step-10}

단계별 상세 테스트 시나리오 작성하겠습니다. 테스트 시나리오 작성 원칙 설명해주세요.

---

## 🤖 Antigravity · `2026-11-21 12:21:16` {#step-11}

단계별 상세 테스트 시나리오를 작성할 때 적용할 **블랙박스 테스트 시나리오 작성 원칙**은 다음과 같습니다.

---

### 1. 계약 기반 검증 (Contract-First, Black-box)
* 내부 코드 구조나 구현 방식에 의존하지 않고, 계약서([handoff-qa-test-brief.md](file:///mnt/data/mytask/kms/guard/doc/test/handoff-qa-test-brief.md) 및 스펙 문서)에 정의된 **외부 관측 가능한 동작**만 검증합니다.
* 검증 대상: CLI 종료 코드(Exit Code), 표준 출력/에러(JSON/텍스트), 파일 시스템 상태(`queue.json`, `.inbox`), 훅 응답(`decision: deny`), 세션 메타데이터(`AllowedWritePaths`).

### 2. Given – When – Then 구조화
모든 시나리오는 모호함을 없애고 누구나 동일하게 실행·판정할 수 있도록 3단계로 기술합니다.
* **Given (사전 조건)**: 초기 큐 상태, 활성/오프라인 세션 유무, 환경 변수(`TEST_HOME`, `AGENT_MODE` 등)
* **When (실행 단계)**: 구체적인 명령어, 입력 페이로드, TUI 키 입력 또는 API 호출
* **Then (기대 결과)**: "정상 동작한다" 같은 추상적 표현 대신 **구체적이고 검증 가능한 Assertion** 명시 (예: `exit code == 0`, `status == "approved"`, `.inbox` 파일 생성 확인)

### 3. 완전한 격리성과 독립성 (Isolation & Determinism)
* **실제 환경 보호**: 모든 시나리오는 실제 `~/.guard`를 건드리지 않고 격리된 임시 디렉터리(`t.TempDir()`, `mktemp -d`)에서 실행합니다.
* **순서 무관성**: 각 시나리오는 다른 시나리오의 실행 결과에 의존하지 않고 단독 실행 시에도 동일한 결과를 보장해야 합니다.

### 4. 정상 경로(Positive) + 예외/경계 경로(Negative & Boundary) 균형
* **정상 흐름(Happy Path)**: 표준 워크플로우가 계약대로 완료되는지 확인합니다.
* **부정 테스트(Negative Path)**: 허용되지 않은 상태 전이(`pending → acked`), 에이전트 자체 승인 시도(`guard handoff approve`), 필수 인자 누락 등이 **기대한 에러/차단으로 실패하는지** 검증합니다.
* **경계값(Boundary)**: 빈 `Targets` 목록, 256KB 상한 초과 참조 문서, 동시 다발적 큐 접근(`flock`) 등 극단 조건을 포함합니다.

### 5. 요구사항 추적성 (Traceability)
* 각 상세 시나리오는 테스트 계획서의 **TC ID**(`TC-STATE-01`, `TC-GATE-01` 등)와 1:1로 매핑하여, 수용 기준 대비 누락된 검증 항목이 없도록 관리합니다.

---

이 원칙을 기준으로 특정 영역(예: `TC-STATE`부터 순차적으로, 또는 전체 영역 일괄)의 상세 시나리오 작성을 원하시면 말씀해 주세요.

---

## 🧑‍💻 User · `2026-11-21 12:23:02` {#step-12}

당신은 테스트를 성공시키는게 목적이 아닙니다. 최대한 FAIL 사례를 찾아내는게 목적입니다. 테스트를 성공시킬 목적으로 에러를 무시하거나, 하드코딩, 빈값 비교등은 엄격히 금지합니다.

---

## 🤖 Antigravity · `2026-11-21 12:23:02` {#step-14}

명확히 숙지했습니다. **통과(PASS)를 만드는 것이 아니라, 계약 위반과 잠재 결함(FAIL)을 최대한 찾아내는 것**을 QA의 최우선 목적으로 삼겠습니다.

앞으로의 테스트 시나리오 작성 및 실행 시 다음 원칙을 엄격히 준수하겠습니다.

### 엄격 적용할 QA 원칙 (Defect-Hunting)
1. **거짓 양성(False Positive) 원천 금지**
   * 에러 무시(`_ = err`, 실패 시 스킵/완화 처리) 금지
   * 테스트 통과를 위한 결과값 하드코딩·모킹 남용 금지
   * 빈 값/널 비교, 단순 `err == nil` 또는 `len > 0` 수준의 느슨한 검증 금지 → **계약 명세 기준의 정확한 값·타입·에러 타입·상태 전이·부수효과(파일/스코프)까지 엄격하게 단언(Assert)**
2. **적대적(Adversarial) 케이스 우선 설계**
   * 경계값 위반, 잘못된 입력, 우회 시도(예: 훅 우회/변형 명령어로 `guard handoff approve` 시도, 경로 트래버설, 잘못된 상태 전이, 동시성 경합 등)를 적극 공격하여 실제 FAIL을 유도
3. **실패 발견 시 있는 그대로 보고**
   * 계약서와 구현이 조금이라도 어긋나면 테스트를 수정해 우회하지 않고, 즉시 **FAIL(결함)**로 기록하고 재현 절차·기대 결과·실제 결과를 보고

어느 단계(`TC-STATE`부터 순차 진행 또는 전체 상세 시나리오 작성)부터 시작할지 지시해 주시면 바로 착수하겠습니다.

---

## 🧑‍💻 User · `2026-11-21 12:23:22` {#step-15}

시나리오 작성해주세요.

---

## 🤖 Antigravity · `2026-11-21 12:26:02` {#step-22}

계약 명세 3종([05_handoff_protocol_contract.md](file:///mnt/data/mytask/kms/guard/doc/spec/05_handoff_protocol_contract.md), [02_session_hub_contract.md](file:///mnt/data/mytask/kms/guard/doc/spec/02_session_hub_contract.md), [03_adapters_cli_contract.md](file:///mnt/data/mytask/kms/guard/doc/spec/03_adapters_cli_contract.md))을 기준으로 결함 발견(Defect-Hunting) 중심의 **`7. 단계별 상세 블랙박스 테스트 시나리오`**를 [handoff-qa-test-brief.md](file:///mnt/data/mytask/kms/guard/doc/test/handoff-qa-test-brief.md) 하단에 append 완료했습니다.

### 추가된 상세 시나리오 구성 (총 6개 Stage, 26개 세부 시나리오)
* **Stage 1 (`TC-STATE-01` ~ `06`)**: 정상 전이, 금지된 상태 전이 전수 차단(`ErrInvalidTransition`), 경로 탈출/빈 요소 입력 공격, `flock` 동시성 승인 경합, 큐 Compaction(`DefaultMaxQueueItems=200`) 활성 티켓 보존
* **Stage 2 (`TC-GATE-01` ~ `04`)**: 모드/정책 무관 자체 승인 차단, 셸 체이닝·서브셸·인라인 스크립트·경로 변형 우회 공격 9종, 정상 명령 오탐 방지, 승인 전 인박스 물리 격리
* **Stage 3 (`TC-SCOPE-01` ~ `08`)**: 조건부 Scope Lock 7개 조합 매트릭스 및 Envelope 헤더(`수정 허용 대상 (Scope Locked)` vs `작업 대상`) 검증, `FormatEnvelope` 필수 필드 누락 방어
* **Stage 4 (`TC-ROUTE-01` ~ `06`)**: 활성 세션 즉시 전달, 사망 PID 배제 오프라인 버퍼링, 미존재 SessionID 승인 시 원자적 롤백, 5개 고루틴 동시 소비 중복 방지, 기전달 티켓 재소비 차단, `launcher.Run` E2E 생명주기 및 종료 정리
* **Stage 5 (`TC-CLI-01` ~ `04`)**: 실제 빌드 바이너리 기준 `send`/`inbox`/`approve`/`reject`/`ack`/`hook` 플래그 파싱, 필터링, 종료 코드 검증
* **Stage 6 (`TC-TUI-01` ~ `04`)**: 3줄 요약 모달·`[H]` 패널 전환, 오버레이 포커스 가로채기 방지, 256KB 초과 참조 문서 방어(`tui.handoff.ref_truncated`), `ko.json`/`en.json` 키·포맷 동사 100% 정합성 검증

시나리오 검토 후 블랙박스 테스트 코드 작성 및 실행을 진행할지 말씀해 주세요.

---

## 🧑‍💻 User · `2026-11-21 12:27:22` {#step-23}

7. 단계별 상세 블랙박스 테스트 시나리오 (Defect-Hunting) 에서 각 단계별 테스트 범위와 목적 간단히 설명해주세요. 그래야 어떤 목표를 가지고 테스트를 작성했는지를 이해할 수 있을 테니까요.

---

## 🤖 Antigravity · `2026-11-21 12:27:22` {#step-24}

[handoff-qa-test-brief.md](file:///mnt/data/mytask/kms/guard/doc/test/handoff-qa-test-brief.md)의 **Stage 1 ~ Stage 6** 각 단계별 테스트 범위와 결함 탐지 목적은 다음과 같습니다.

| 단계 | 테스트 범위 (대상 계층) | 테스트 목적 (탐지하려는 핵심 결함) |
| :--- | :--- | :--- |
| **Stage 1**<br>`TC-STATE` | `internal/hub/handoff`<br>(큐 저장소, 상태 머신, 모델 검증) | **데이터 무결성 및 상태 오염 탐지**<br>허용되지 않은 상태 전이 누락, 경로 탈출(`../`) 입력 통과, 동시 접근(`flock`) 시 큐 파일 손상·중복 승인, 큐 상한(200건) 초과 시 활성 티켓 유실 결함을 찾아냅니다. |
| **Stage 2**<br>`TC-GATE` | `internal/core`, `internal/adapters/hook`<br>(PreToolUse 보안 게이트) | **보안 우회(Bypass) 및 신뢰 경계 붕괴 탐지**<br>에이전트가 셸 체이닝(`&&`, `;`), 서브셸(`bash -c`), 인라인 스크립트(`python3 -c`), 경로 변형 등으로 `guard handoff approve` 자체 승인 차단을 뚫을 수 있는지, 또는 승인 전 인박스에 티켓이 조기 유출되는지 검증합니다. |
| **Stage 3**<br>`TC-SCOPE` | `internal/hub/handoff`, `internal/hub/session`<br>(조건부 Scope Lock & Envelope) | **권한 과잉 부여 및 스코프 오염 탐지**<br>`verify`/`review` 요청이나 읽기 전용 모드(`PLAN`/`TEST`)에 쓰기 권한(`AllowedWritePaths`)이 잘못 적용되거나, 기존 세션 스코프가 의도치 않게 덮어써지는 결함을 찾아냅니다. |
| **Stage 4**<br>`TC-ROUTE` | `internal/hub/handoff`, `internal/adapters/launcher`<br>(라우터, 오프라인 큐, 런처 생명주기) | **메시지 유실·중복 전달·좀비 자원 누수 탐지**<br>존재하지 않는 세션 승인 시 상태만 `approved`로 반쪽 변경되는 원자성 깨짐, 다중 런처 기동 시 오프라인 티켓 중복 소비(Race), 사망 PID 세션으로의 오배송, 세션 종료 후 `.inbox`/세션 파일 잔존 결함을 검증합니다. |
| **Stage 5**<br>`TC-CLI-E2E` | `cmd/guard` (실제 빌드 바이너리)<br>(CLI 서브커맨드 & 파이프라인) | **실제 사용자/에이전트 접점의 통합 결함 탐지**<br>단위 테스트를 통과하더라도 실제 바이너리에서 플래그 파싱(`--ref`, `--targets` 쉼표/반복), `--json` 스키마, 오류 시 비정상 종료 코드(`0` 반환 등), `guard hook` 표준입출력 파이프라인이 깨지는 문제를 잡아냅니다. |
| **Stage 6**<br>`TC-TUI-I18N` | `internal/adapters/tui`, `internal/i18n`<br>(관제탑 UI 및 다국어 리소스) | **운영자 UX 깨짐·크래시·로컬라이징 누락 탐지**<br>도움말/상세 모달 열람 중 포커스 강제 탈취, 256KB 초과 대용량 참조 문서 열람 시 프리즈/크래시, `ko.json`/`en.json` 간 키 누락 또는 포맷 동사 불일치로 인한 런타임 출력 깨짐을 찾아냅니다. |

---

## 🧑‍💻 User · `2026-11-21 15:17:38` {#step-0}

@[doc/test/handoff-qa-test-brief.md] 에 따라서 븍랙박스 테스트를 진행하겠습니다. 1단계 부터 진행해주세요.

---

## 🧑‍💻 User · `2026-11-21 15:18:05` {#step-6}

먼저 테스트 원칙을 말해주세요.

---

## 🤖 Antigravity · `2026-11-21 15:18:05` {#step-7}

[handoff-qa-test-brief.md](file:///mnt/data/mytask/kms/guard/doc/test/handoff-qa-test-brief.md)와 [05_handoff_protocol_contract.md](file:///mnt/data/mytask/kms/guard/doc/spec/05_handoff_protocol_contract.md)에 정의된 이번 블랙박스 QA 테스트 원칙은 다음과 같습니다.

1. **결함 탐지 중심 (Defect-Hunting)**
   - 테스트 통과(PASS)가 아니라 **계약 위반 및 결함(FAIL) 발견**을 목적으로 합니다.
   - 에러 무시(`_ = err`), 통과용 하드코딩, 느슨한 검증(`err == nil`만 확인, 빈 값·단순 길이 비교)을 금지합니다.

2. **계약 기반 엄격한 단언 (Contract-Driven Strict Assertions)**
   - [05_handoff_protocol_contract.md](file:///mnt/data/mytask/kms/guard/doc/spec/05_handoff_protocol_contract.md) 등 계약서에 명시된 정확한 값, 센티넬 에러 타입(`errors.Is`), 상태 전이, 파일/디스크 부수효과를 외부(블랙박스) 관점에서 검증합니다.

3. **완전한 환경 격리 (Environment Isolation)**
   - 실제 `~/.guard` 및 작업 공간을 오염시키지 않도록 모든 Go 테스트는 `t.TempDir()`로 디렉터리를 주입하고, CLI 테스트는 임시 `TEST_HOME`/`TEST_WS`에서 독립 실행 후 정리합니다.

4. **적대적·동시성 검증 (Adversarial & Race Testing)**
   - 금지된 상태 전이 전수 시도, 경로 탈출(`../`), 우회 실행 명령, 다중 고루틴 경합 등을 포함하며 `-race` 플래그로 데이터 레이스를 검증합니다.

5. **TEST 모드 권한 경계 준수**
   - 프로덕션 코드는 수정하지 않고 테스트 파일(`*_test.go`)과 `doc/` 문서 범위 내에서만 작성·검증합니다.

이 원칙대로 **Stage 1 (`TC-STATE`)**부터 바로 진행할까요?

---

## 🧑‍💻 User · `2026-11-21 15:18:33` {#step-8}

승인

---

## 🤖 Antigravity · `2026-11-21 15:20:47` {#step-22}

**Stage 1 (`TC-STATE`)** 블랙박스 테스트 작성 및 `-race` 실행을 완료했습니다.

- **테스트 파일**: [handoff_state_test.go](file:///mnt/data/mytask/kms/guard/test/e2e/handoff_state_test.go)
- **실행 명령**: `go test -race -v ./test/e2e/... -run "TestHandoffState"`
- **결과**: **6 / 6 PASS** (`3.975s`, 데이터 레이스 없음)

| TC ID | 검증 항목 | 결과 |
| :--- | :--- | :---: |
| **TC-STATE-01** | `pending → approved → acked` 전이, ID 정규식(`^hnd-\d{8}-[0-9a-f]{8}$`), `CreatedAt`/`UpdatedAt` 영속화 | **PASS** |
| **TC-STATE-02** | `pending → rejected` 반려 사유 저장 및 활성 DEV 세션 `.inbox` 미생성 | **PASS** |
| **TC-STATE-03** | 금지 상태 전이 13종 전수 `ErrInvalidTransition` 반환 및 상태/사유 불변 | **PASS** |
| **TC-STATE-04** | `ParseTarget`/`Enqueue` 입력 검증(`ErrInvalidItem`), `InboxPath` 경로 탈출(`ErrInvalidSessionID`), 중복 ID 차단 및 기존 티켓 무손상 | **PASS** |
| **TC-STATE-05** | 10고루틴×5건(50건) 동시 `Enqueue` 무결성·JSON 유효성 및 10고루틴 동시 승인 경합(1건 성공, 9건 `ErrInvalidTransition`) | **PASS** |
| **TC-STATE-06** | 완료 180건 + 활성 50건(총 230건) Compaction 시 활성 50건 100% 보존, 전체 `<= 200`, 최오래된 완료 30건 제거 | **PASS** |

다음으로 **Stage 2 (`TC-GATE` — Core 신뢰 경계 및 자체 승인 우회 공격)**를 진행할까요?

---

## 🧑‍💻 User · `2026-11-21 18:06:41` {#step-23}

진행해주세요.

---

## 🤖 Antigravity · `2026-11-21 21:57:14` {#step-32}

**Stage 2 (`TC-GATE`)** 블랙박스 테스트 작성 및 `-race` 실행을 완료했습니다.

- **테스트 파일**: [handoff_gate_test.go](file:///mnt/data/mytask/kms/guard/test/e2e/handoff_gate_test.go)
- **실행 명령**: `go test -race -v ./test/e2e/... -run "TestHandoffGate"`
- **결과**: **4 / 4 PASS** (`1.021s`, 데이터 레이스 없음)

| TC ID | 검증 항목 | 결과 |
| :--- | :--- | :---: |
| **TC-GATE-01** | 모드 4종(`PLAN`/`DEV`/`TEST`/`ALL`) × 정책 2종(`nil`, 전체 허용) 8개 조합 모두 `Allowed=false`, `RuleName="handoff-self-approve-block"`, 비어있지 않은 `Reason` 반환 | **PASS** |
| **TC-GATE-02** | 셸 체이닝(`&&`, `;`, `\|\|`), `bash -c`, `sh -c`, `python3 -c`, 절대/상대 경로, `AGENT_MODE=ALL` 접두사 등 9종 우회 공격 모두 `core` 및 `hook`(`stdout` JSON `deny` + `stderr` 1줄) 차단 | **PASS** |
| **TC-GATE-03** | 정상 명령 4종(`send`, `inbox`, `ack`, `reject`) 오탐 없이 `Allowed=true` | **PASS** |
| **TC-GATE-04** | 승인 전 활성 `DEV` 세션 `.inbox` 파일 미생성 및 `AllowedWritePaths` 미변경 | **PASS** |

다음으로 **Stage 3 (`TC-SCOPE` — 조건부 Scope Lock 매트릭스 및 Envelope 계약)**를 진행할까요?

---

## 🧑‍💻 User · `2026-11-21 21:57:30` {#step-33}

승인

---

## 🤖 Antigravity · `2026-11-21 21:58:42` {#step-39}

**Stage 3 (`TC-SCOPE`)** 블랙박스 테스트 작성 및 `-race` 실행을 완료했습니다.

- **테스트 파일**: [handoff_scope_test.go](file:///mnt/data/mytask/kms/guard/test/e2e/handoff_scope_test.go)
- **실행 명령**: `go test -race -v ./test/e2e/... -run "TestHandoffScope"`
- **결과**: **7 / 8 PASS, 1 FAIL** (`TC-SCOPE-05`에서 명세 대비 불일치 발견)

| TC ID | 검증 항목 | 결과 |
| :--- | :--- | :---: |
| **TC-SCOPE-01** | `fix` + `["pkg/a.go"]` + `DEV` → `ScopeLocked=true`, `AllowedWritePaths=["pkg/a.go"]`, `수정 허용 대상 (Scope Locked)` | **PASS** |
| **TC-SCOPE-02** | `task` + `["pkg/b.go","pkg/c.go"]` + `ALL` → `ScopeLocked=true`, `AllowedWritePaths` 반영, `수정 허용 대상 (Scope Locked)` | **PASS** |
| **TC-SCOPE-03** | `verify` + `["pkg/a.go"]` + `TEST` → `ScopeLocked=false`, 기존 스코프 불변, `작업 대상` (`Scope Locked` 미포함) | **PASS** |
| **TC-SCOPE-04** | `review` + `["doc/a.md"]` + `PLAN` → `ScopeLocked=false`, 기존 스코프 불변, `작업 대상` (`Scope Locked` 미포함) | **PASS** |
| **TC-SCOPE-05** | `fix` + `[]` + `DEV` → `ScopeLocked=false` 및 기존 스코프 불변은 일치하나, **Envelope에 `작업 대상` 헤더 미출력** | **FAIL** |
| **TC-SCOPE-06** | `fix` + `["pkg/a.go"]` + `TEST` → `ScopeLocked=false`, 기존 스코프 불변, `작업 대상` (`Scope Locked` 미포함) | **PASS** |
| **TC-SCOPE-07** | `task` + `["pkg/a.go"]` + `PLAN` → `ScopeLocked=false`, 기존 스코프 불변, `작업 대상` (`Scope Locked` 미포함) | **PASS** |
| **TC-SCOPE-08** | `FormatEnvelope` 필수 필드(`ID`, `Type`, `Summary`) 누락 시 `ErrInvalidItem` 반환 | **PASS** |

### 발견된 불일치 (`TC-SCOPE-05`)
- **테스트 명세** ([handoff-qa-test-brief.md](file:///mnt/data/mytask/kms/guard/doc/test/handoff-qa-test-brief.md#L332)): `Targets: []`일 때 기대 Envelope 헤더에 `작업 대상` (`Scope Locked` 미포함) 명시
- **실제 출력**: `Targets`가 비어 있으면 대상 섹션 자체를 생략함
  ```markdown
  <!-- GUARD_HANDOFF_ENVELOPE: START (ID: hnd-20261121-ed88e6d4, Type: fix) -->
  ### 🔔 [Handoff: FIX] from PLAN

  **요약**: scope lock matrix test: TC-SCOPE-05/fix_empty_targets_DEV

  > 작업 완료 후 `guard handoff ack hnd-20261121-ed88e6d4`을 실행하세요.
  <!-- GUARD_HANDOFF_ENVELOPE: END -->
  ```

`TC-SCOPE-05`를 **결함(FAIL)으로 유지**하고 넘어갈까요, 아니면 `Targets`가 비어 있을 때 섹션 생략을 정상 동작으로 간주해 명세/테스트를 조정할까요? 이후 **Stage 4 (`TC-ROUTE`)** 진행 여부도 함께 알려주세요.

---

## 🧑‍💻 User · `2026-11-21 22:00:01` {#step-40}

개발자다에게 전달할 테스트 보고서를 @[test] 디렉토리에 만들어주세요.

---

## 🤖 Antigravity · `2026-11-21 22:00:21` {#step-43}

현재 세션의 `TEST` 모드 가드레일 정책으로 인해 [test](file:///mnt/data/mytask/kms/guard/test) 디렉토리 내 `.md` 파일 생성이 차단되었습니다.

> `[POLICY VIOLATION] TEST 모드에서는 doc/ 디렉토리와 테스트 파일(*_test.go)만 수정할 수 있습니다.`

허용된 경로인 `doc/test/handoff-qa-report.md`에 보고서를 작성해 드릴까요? (또는 모드 전환 후 `test/` 디렉토리에 작성하시겠습니까?)

---

## 🧑‍💻 User · `2026-11-21 22:03:56` {#step-44}

@doc/test/ 에 저장해주세요.

---

## 🤖 Antigravity · `2026-11-21 22:05:16` {#step-47}

개발자 전달용 블랙박스 QA 테스트 보고서를 저장했습니다.

- **보고서 파일**: [handoff-qa-report.md](file:///mnt/data/mytask/kms/guard/doc/test/handoff-qa-report.md)
- **포함 내용**: Stage 1~3 실행 요약, `TC-SCOPE-05` 불일치 상세(기대/실제 Envelope 출력, 재현 명령, 개발자 확인 요청), 단계별 전체 TC 결과표

이어서 **Stage 4 (`TC-ROUTE`)**를 진행할까요?

---

## 🧑‍💻 User · `2026-11-21 22:06:21` {#step-0}

QA 로 부터 테스트 문서 @[doc/test/handoff-qa-report.md] 가 전달됐습니다. 하나의 FAIL이 발견됐다고 합니다. 리뷰/검토 해주세요.

---

## 🤖 Antigravity · `2026-11-21 22:06:40` {#step-10}

[handoff-qa-report.md](file:///home/yundream/mytask/kms/guard/doc/test/handoff-qa-report.md) 검토 결과, 발견된 1건의 FAIL(`TC-SCOPE-05`)은 **기능/보안 결함이 아니라 명세서 기술 누락으로 인한 기대값 불일치**입니다.

---

### 1. 원인 분석

- **구현 동작** ([envelope.go](file:///home/yundream/mytask/kms/guard/internal/hub/handoff/envelope.go#L39-L69)):
  - `FormatEnvelope`은 선택 필드인 `References`, `Targets`, `Payload`가 비어 있을 경우(`len == 0` / `""`) 해당 마크다운 섹션 전체를 생략하도록 일관되게 구현되어 있습니다.
  - 따라서 `Targets`가 빈 슬라이스(`[]`)일 때 `**작업 대상**:` 헤더를 출력하지 않는 것은 의도된 정상 동작입니다.
- **명세서 누락** ([05_handoff_protocol_contract.md](file:///home/yundream/mytask/kms/guard/doc/spec/05_handoff_protocol_contract.md#L97)):
  - `FormatEnvelope` 계약 설명에 `len(Targets) > 0` 조건이 빠진 채 *"`scopeLocked == true`이면 `수정 허용 대상 (Scope Locked)`, `false`이면 `작업 대상` 헤더 출력"*으로만 기술되어 있습니다.
- **테스트 명세 반영** ([handoff-qa-test-brief.md](file:///home/yundream/mytask/kms/guard/doc/test/handoff-qa-test-brief.md#L332)):
  - 위 계약 문구를 그대로 `TC-SCOPE-05` (`Targets: []`) 행에 적용하면서, 빈 타겟에도 `작업 대상` 헤더가 포함될 것을 기대값으로 설정해 FAIL이 발생했습니다.

---

### 2. 핵심 검증 상태 평가

- **Stage 1 (`TC-STATE`)** 6/6 PASS, **Stage 2 (`TC-GATE`)** 4/4 PASS로 상태 머신·동시성·자체 승인 차단 신뢰 경계는 모두 정상 통과했습니다.
- **Stage 3 (`TC-SCOPE-05`)**에서도 핵심 불변식(`ShouldApplyScopeLock == false`, `ScopeLocked == false`, `AllowedWritePaths` 불변 유지, `Scope Locked` 문구 미포함)은 모두 정상 통과했습니다.

---

### 3. 권장 조치

항목 없는 빈 `**작업 대상**:` 헤더를 출력하는 것보다 **현행 구현(빈 `Targets` 시 섹션 생략)을 유지하고 명세를 명확화**하는 것을 권장합니다.

1. [05_handoff_protocol_contract.md](file:///home/yundream/mytask/kms/guard/doc/spec/05_handoff_protocol_contract.md#L97): `Targets`가 비어있지 않을 때만 헤더를 출력하고, 비어 있으면 섹션을 생략함을 명시
2. [handoff-qa-test-brief.md](file:///home/yundream/mytask/kms/guard/doc/test/handoff-qa-test-brief.md#L332): `TC-SCOPE-05` 기대 Envelope 헤더를 `작업 대상 섹션 생략 (Scope Locked 미포함)`으로 정정
3. QA에 현행 동작이 의도된 스펙임을 회신하고 `TC-SCOPE-05` 기대값 수정 후 Stage 4~6 진행 요청

위 방향대로 계약서와 테스트 브리프 문서 정정을 진행할까요?

---

## 🧑‍💻 User · `2026-11-21 22:07:50` {#step-11}

승인

---

## 🤖 Antigravity · `2026-11-21 22:08:32` {#step-15}

두 문서의 명세 정정을 완료했습니다.

### 수정 내역
- [05_handoff_protocol_contract.md](file:///home/yundream/mytask/kms/guard/doc/spec/05_handoff_protocol_contract.md#L97): `FormatEnvelope` 계약에 `len(Targets) > 0` 조건 및 빈 `Targets`/`References`/`Payload` 섹션 생략 규칙 명시
- [handoff-qa-test-brief.md](file:///home/yundream/mytask/kms/guard/doc/test/handoff-qa-test-brief.md#L332): `TC-SCOPE-05` 기대 Envelope 헤더를 `` `작업 대상` 섹션 생략 (`Scope Locked` 미포함) ``으로 정정

이제 QA 측에 현행 동작이 의도된 스펙임을 회신하고 `TC-SCOPE-05` 기댓값 갱신 및 Stage 4~6 진행을 요청하시면 됩니다.

---

## 🧑‍💻 User · `2026-11-21 22:09:50` {#step-16}

조치 내용을 @[doc/test/handoff-qa-report.md] 에 Update(Append) 해주세요.

---

## 🤖 Antigravity · `2026-11-21 22:10:02` {#step-20}

[handoff-qa-report.md](file:///home/yundream/mytask/kms/guard/doc/test/handoff-qa-report.md) 하단에 `4. 개발자 검토 및 조치 결과 (2026-11-21)` 섹션을 추가(Append)했습니다.

### 추가된 내용 요약
- **판정**: `TC-SCOPE-05`는 코드 결함이 아닌 명세서 누락으로 인한 기대값 불일치 (`Targets`/`References`/`Payload` 빈 값 시 섹션 생략이 의도된 동작)
- **조치 내역**: [05_handoff_protocol_contract.md](file:///home/yundream/mytask/kms/guard/doc/spec/05_handoff_protocol_contract.md#L97) 및 [handoff-qa-test-brief.md](file:///home/yundream/mytask/kms/guard/doc/test/handoff-qa-test-brief.md#L332) 정정 내용 기록
- **QA 요청 사항**: `TC-SCOPE-05` 테스트 기대값 갱신 후 재검증 및 Stage 4~6 진행 요청

---

## 🧑‍💻 User · `2026-11-21 22:10:45` {#step-48}

개발자가 조치 내용을 @[doc/test/handoff-qa-report.md] 에 업데이트 했습니다. 검토해주세요.

---

## 🤖 Antigravity · `2026-11-21 22:10:54` {#step-55}

개발자 조치 내용을 검토했으며, 관련 명세 문서 2건에 모두 일관되게 반영된 것을 확인했습니다.

### 검토 결과: **타당 (수용 가능)**
- [05_handoff_protocol_contract.md](file:///mnt/data/mytask/kms/guard/doc/spec/05_handoff_protocol_contract.md#L97): `len(Targets) > 0`일 때만 헤더를 출력하고, `Targets`/`References`/`Payload`가 비어 있으면 해당 섹션을 생략하는 계약으로 명확화됨.
- [handoff-qa-test-brief.md](file:///mnt/data/mytask/kms/guard/doc/test/handoff-qa-test-brief.md#L332): `TC-SCOPE-05` 기대 Envelope 헤더가 `` `작업 대상` 섹션 생략 (`Scope Locked` 미포함) ``으로 정정됨.
- [handoff-qa-report.md](file:///mnt/data/mytask/kms/guard/doc/test/handoff-qa-report.md#L99-L107): 판정 사유 및 QA 요청 사항이 누락 없이 정리됨.

### 다음 단계 제안
1. [handoff_scope_test.go](file:///mnt/data/mytask/kms/guard/test/e2e/handoff_scope_test.go)의 `TC-SCOPE-05` 기대값을 `작업 대상` 및 `Scope Locked` **둘 다 미포함(생략)**으로 엄격하게 갱신하고 Stage 3 재검증
2. **Stage 4 (`TC-ROUTE`)** 진행

진행할까요?

---

## 🧑‍💻 User · `2026-11-21 22:11:48` {#step-56}

TC-SCOPE-05 조정하고 재검증 해주세요.

---

## 🤖 Antigravity · `2026-11-21 22:48:23` {#step-62}

`TC-SCOPE-05` 기대값을 정정된 명세(`작업 대상` 및 `Scope Locked` 모두 미포함)에 맞게 갱신하고 재검증을 완료했습니다.

- **수정 파일**: [handoff_scope_test.go](file:///mnt/data/mytask/kms/guard/test/e2e/handoff_scope_test.go)
- **실행 명령**: `go test -race -v ./test/e2e/... -run "TestHandoffScope"`
- **결과**: **8 / 8 PASS** (`1.027s`, 데이터 레이스 없음)

다음으로 **Stage 4 (`TC-ROUTE` — 세션 라우팅 · 오프라인 버퍼링 · 중복 소비 방지 · 런처 생명주기)**를 진행할까요?

---

## 🧑‍💻 User · `2026-11-21 22:49:01` {#step-63}

테스트 코드 작성 원칙 알려주세요.

---

## 🤖 Antigravity · `2026-11-21 22:49:01` {#step-64}

이번 블랙박스 QA 테스트 코드 작성 시 준수하는 원칙은 다음과 같습니다.

1. **결함 탐지 중심 (Defect-Hunting)**
   - 테스트를 통과(PASS)시키는 것이 아니라 **계약 위반·결함(FAIL) 발견**을 목표로 작성합니다.
   - 구현 코드 내부를 추정해 기대값을 맞추거나 통과용 하드코딩을 넣지 않습니다.

2. **느슨한 검증 및 에러 무시 금지**
   - `_ = err`로 에러를 무시하지 않습니다.
   - 단순 `err != nil` 또는 `len > 0` 확인에 그치지 않고, 계약서에 정의된 **정확한 센티넬 에러(`errors.Is`)**, **정규식·문자열·구조체 필드**, **디스크 영속화 결과(`Get` 재조회, 파일 존재/부재)**까지 엄격하게 단언합니다.

3. **외부 계약 기반 블랙박스 작성**
   - `package e2e_test`와 같이 외부 패키지 관점에서 공개 API·계약서([05_handoff_protocol_contract.md](file:///mnt/data/mytask/kms/guard/doc/spec/05_handoff_protocol_contract.md), [02_session_hub_contract.md](file:///mnt/data/mytask/kms/guard/doc/spec/02_session_hub_contract.md), [03_adapters_cli_contract.md](file:///mnt/data/mytask/kms/guard/doc/spec/03_adapters_cli_contract.md), [01_core_policy_contract.md](file:///mnt/data/mytask/kms/guard/doc/spec/01_core_policy_contract.md))만 기준으로 작성합니다.

4. **완전한 테스트 격리**
   - 실제 `~/.guard` 및 워크스페이스를 오염시키지 않도록 모든 테스트 케이스가 `t.TempDir()`, `t.Setenv()` 또는 임시 `TEST_HOME`/`TEST_WS`에서 독립적으로 실행·정리되도록 구성합니다.

5. **적대적 입력 및 동시성·레이스 검증**
   - 경로 탈출(`../`), 금지 상태 전이 전수 시도, 우회 명령어 등 Adversarial 케이스를 포함합니다.
   - 동시성 테스트는 start barrier 채널 등으로 실제 경합을 유도하고 `-race` 플래그로 실행합니다.

6. **명세 불일치 시 임의 완화 금지**
   - 실패가 발생하면 테스트 기대값을 임의로 낮추지 않고 실제 출력과 명세 차이를 보고서로 전달한 뒤, 명세/구현 확정 후에만 테스트를 갱신합니다.

이어서 **Stage 4 (`TC-ROUTE`)**를 진행할까요?

---

## 🧑‍💻 User · `2026-11-21 22:49:24` {#step-65}

승인

---

## 🤖 Antigravity · `2026-11-21 23:12:30` {#step-72}

**Stage 4 (`TC-ROUTE`)** 블랙박스 테스트 작성 및 `-race` 실행을 완료했습니다.

- **테스트 파일**: [handoff_route_test.go](file:///mnt/data/mytask/kms/guard/test/e2e/handoff_route_test.go)
- **실행 명령**: `go test -race -v ./test/e2e/... -run "TestHandoffRoute"`
- **결과**: **5 / 6 PASS, 1 FAIL** (`TC-ROUTE-06`에서 결함/명세 불일치 발견)

| TC ID | 검증 항목 | 결과 |
| :--- | :--- | :---: |
| **TC-ROUTE-01** | 활성 `DEV` 세션 즉시 전달, Envelope 시작/종료 마커·ID·요약 검증, `To.SessionID` 기록 | **PASS** |
| **TC-ROUTE-02** | 사망 PID(`999999`) Stale 세션 배제, 오프라인 `Buffered=true` 보관 및 Stale `.inbox` 미생성 | **PASS** |
| **TC-ROUTE-03** | 미존재 특정 `SessionID`(`sess-ghost-999`) 승인 실패 시 `pending` 상태 원자적 롤백 | **PASS** |
| **TC-ROUTE-04** | 5개 세션 동시 `ConsumePendingForMode` 경합 시 정확히 1개 세션만 소비 및 1개 `.inbox` 생성 | **PASS** |
| **TC-ROUTE-05** | 기전달 티켓(`To.SessionID != ""`) 신규 세션 재소비 차단 | **PASS** |
| **TC-ROUTE-06** | `launcher.Run` 오프라인 자동 소비·환경변수(`AGENT_MODE`, `GUARD_SESSION_ID`, `GUARD_INBOX_PATH`) 주입·종료 후 `.json`/`.inbox` 정리·비TTY OSC 미출력은 통과했으나, **`<ws>/.agents/rules/guard-workflow.md` 미생성** | **FAIL** |

### 발견된 결함 (`TC-ROUTE-06`)
- **테스트 명세** ([handoff-qa-test-brief.md](file:///mnt/data/mytask/kms/guard/doc/test/handoff-qa-test-brief.md#L388-L393)): `launcher.Run("dev", ws, []string{"sh", "-c", ...})` 실행 시 `<ws>/.agents/rules/guard-workflow.md` 파일 자동 생성 확인
- **실제 결과**: `stat <ws>/.agents/rules/guard-workflow.md: no such file or directory`
- **재현 명령**: `go test -race -v ./test/e2e/... -run "TestHandoffRoute06"`

이 내용을 [handoff-qa-report.md](file:///mnt/data/mytask/kms/guard/doc/test/handoff-qa-report.md)에 업데이트할까요, 아니면 **Stage 5 (`TC-CLI-E2E`)**로 계속 진행할까요?

---

## 🧑‍💻 User · `2026-11-21 23:13:28` {#step-73}

State 5 진행해주세요.

---

## 🤖 Antigravity · `2026-11-21 23:14:36` {#step-79}

**Stage 5 (`TC-CLI-E2E`)** 블랙박스 테스트 작성 및 `-race` 실행을 완료했습니다.

- **테스트 파일**: [handoff_cli_test.go](file:///mnt/data/mytask/kms/guard/test/e2e/handoff_cli_test.go)
- **실행 명령**: `go test -race -v ./test/e2e/... -run "TestHandoffCLI"`
- **결과**: **1 / 4 PASS, 3 FAIL** (실제 결함 2건 발견)

| TC ID | 검증 항목 | 결과 |
| :--- | :--- | :---: |
| **TC-CLI-01** | `guard handoff send` 정상 플래그 파싱(`--ref` 반복, `--targets` 쉼표 구분, JSON 검증) 및 필수값 누락/경로 탈출(`../bad`) 종료 코드 `1` 거부 | **PASS** |
| **TC-CLI-02** | `guard handoff inbox` 필터링(`--json`, `--all`, `--mode dev`) — 사전 셋업의 `reject <id> -m "반려"` 실패로 중단 | **FAIL** |
| **TC-CLI-03** | `approve`/`reject`/`ack` 상태 전이 및 오류 종료 코드 — `reject <id> -m "스펙 변경"` 실패 | **FAIL** |
| **TC-CLI-04** | `guard hook` 바이너리 파이프라인 자체 승인 차단 (`tool_input.command` 페이로드) — `deny` 대신 `allow` 반환 | **FAIL** |

### 발견된 결함 상세

1. **[DEFECT-CLI-01] `guard handoff reject <id> -m "<reason>"` 위치 인자 뒤 플래그 파싱 실패 (`TC-CLI-02`, `TC-CLI-03`)**
   - **계약/명세**: `guard handoff reject <id> [-m "<reason>"]` ([05_handoff_protocol_contract.md](file:///mnt/data/mytask/kms/guard/doc/spec/05_handoff_protocol_contract.md#L132), [handoff-qa-test-brief.md](file:///mnt/data/mytask/kms/guard/doc/test/handoff-qa-test-brief.md#L425))
   - **실제 동작**: `guard handoff reject <id> -m "스펙 변경"` 실행 시 종료 코드 `1` 및 `stderr="usage: guard handoff reject <id> [-m \"reason\"]\n"` 출력 (자기 usage 문법대로 호출했는데도 거부됨)

2. **[DEFECT-CLI-02] `guard hook`에서 `tool_input.command` JSON 페이로드 자체 승인 차단 우회 (`TC-CLI-04`)**
   - **명세**: `echo '{"tool_name":"run_command","tool_input":{"command":"guard handoff approve hnd-1"}}' | AGENT_MODE=ALL guard hook` → `{"decision":"deny","reason":"..."}` ([handoff-qa-test-brief.md](file:///mnt/data/mytask/kms/guard/doc/test/handoff-qa-test-brief.md#L93-L95), [L431-L435](file:///mnt/data/mytask/kms/guard/doc/test/handoff-qa-test-brief.md#L431-L435))
   - **실제 동작**: 종료 코드 `0`, `stdout="{\"decision\":\"allow\"}\n"`, `stderr=""` (계약서 [03_adapters_cli_contract.md](file:///mnt/data/mytask/kms/guard/doc/spec/03_adapters_cli_contract.md#L12-L28)의 `hook.Input`에 `tool_input` 필드가 없어 명령 추출 실패 후 `allow` 처리됨)

마지막 **Stage 6 (`TC-TUI-I18N`)**까지 마저 실행한 뒤 전체 결과를 [handoff-qa-report.md](file:///mnt/data/mytask/kms/guard/doc/test/handoff-qa-report.md)에 통합 보고할까요?

---

## 🧑‍💻 User · `2026-11-21 23:16:15` {#step-80}

Stage 6 실행해주세요. 그 후 전체 결과를 통합 보고 하겠습니다.

---

## 🤖 Antigravity · `2026-11-21 23:40:35` {#step-88}

**Stage 6 (`TC-TUI-I18N`)** 블랙박스 테스트 실행과 **Stage 1 ~ Stage 6 통합 보고서** 작성을 완료했습니다.

### Stage 6 실행 결과
- **테스트 파일**: [handoff_tui_i18n_test.go](file:///mnt/data/mytask/kms/guard/test/e2e/handoff_tui_i18n_test.go)
- **실행 명령**: `go test -race -v ./test/e2e/... -run "TestHandoffTUI"`
- **결과**: **4 / 4 PASS** (`1.059s`, 데이터 레이스 없음)
  - `TC-TUI-01`: `[H]` 패널 토글, 모달 `[Enter]` 승인 / `[Esc]` 반려 **PASS**
  - `TC-TUI-02`: `?` 도움말 및 `Space` 이벤트 상세 모달 활성 시 포커스 가로채기 방지 **PASS**
  - `TC-TUI-03`: 300KB 참조 문서 `[V]` 뷰어 256KB 상한 및 `tui.handoff.ref_truncated` 출력 **PASS**
  - `TC-TUI-04`: `ko.json`/`en.json` 전체 키·포맷 동사 100% 일치 및 로케일 우선순위/`C`·`POSIX` 폴백 **PASS**

### Stage 1 ~ Stage 6 통합 요약
- **통합 보고서**: [handoff-qa-report.md](file:///mnt/data/mytask/kms/guard/doc/test/handoff-qa-report.md)
- **전체 결과**: **28 / 32 PASS, 4 FAIL** (개발자 조치 필요 결함 **3건**)
  1. **DEFECT-01 (`TC-ROUTE-06`, Medium)**: `launcher.Run` 실행 시 `<ws>/.agents/rules/guard-workflow.md` 미생성
  2. **DEFECT-02 (`TC-CLI-02`, `TC-CLI-03`, High)**: `guard handoff reject <id> -m "<reason>"` 위치 인자 뒤 플래그 파싱 실패
  3. **DEFECT-03 (`TC-CLI-04`, Critical)**: `guard hook`에서 `tool_input.command` 포맷 페이로드 자체 승인 차단 우회 (`allow` 반환)

---

## 🧑‍💻 User · `2026-11-21 23:42:22` {#step-21}

@[doc/test/handoff-qa-report.md] 에 QA 테스트 보고서가 추가 되었습니다. 4개의 FAIL이 발견됐다고 합니다. 검토해주세요.

---

## 🤖 Antigravity · `2026-11-21 23:42:56` {#step-38}

[handoff-qa-report.md](file:///home/yundream/mytask/kms/guard/doc/test/handoff-qa-report.md)의 신규 결함 3건(4 FAIL)을 실제 구현 코드와 대조하여 검토한 결과입니다.

---

### 검토 요약

| 결함 ID | 심각도 | 판정 | 요약 |
| :--- | :---: | :---: | :--- |
| **DEFECT-01** (`TC-ROUTE-06`) | Medium | **설계 판단 / 명세 불일치** | `EnsureWorkflowRule`이 `agy` 실행 시에만 호출되도록 의도적으로 분기되어 있음 |
| **DEFECT-02** (`TC-CLI-02`, `03`) | **High** | **실제 코드 버그** | Go `flag.FlagSet`이 위치 인자(`<id>`) 뒤의 `-m` 플래그를 파싱하지 못함 |
| **DEFECT-03** (`TC-CLI-04`) | **Critical** | **실제 코드 결함 (보안)** | `hook.Input`에 `tool_input` 필드가 누락되어 `tool_input.command` 추출 실패 → Fail-open(`allow`) |

---

### 상세 분석 및 권장 조치

#### 1. `[DEFECT-01]` `launcher.Run` 실행 시 `.agents/rules/guard-workflow.md` 미생성
- **근거**: [launcher.go](file:///home/yundream/mytask/kms/guard/internal/adapters/launcher/launcher.go#L86-L97)
  - `EnsureWorkspaceHook(workspace)`와 `EnsureWorkflowRule(workspace)` 모두 `if filepath.Base(cmdArgs[0]) == "agy"` 블록 안에서만 실행됩니다.
  - 반면 `TC-ROUTE-06`은 `sh -c ...` 명령으로 `launcher.Run`을 호출하면서 `.agents/rules/guard-workflow.md` 생성을 검증합니다.
- **권장 조치 (선택 필요)**:
  - **안 A (권장 — 구현 유지 + 명세 명확화)**: `guard dev sh -c ...` 같은 일반 셸 명령 실행 시 워크스페이스에 `.agents/` 파일을 생성(오염)하지 않도록 `agy` 전용 부트스트랩을 유지하고, [03_adapters_cli_contract.md](file:///home/yundream/mytask/kms/guard/doc/spec/03_adapters_cli_contract.md#L75)와 [handoff-qa-test-brief.md](file:///home/yundream/mytask/kms/guard/doc/test/handoff-qa-test-brief.md#L388-L393)에 `agy` 실행 시 생성 조건을 명시.
  - **안 B (구현 변경)**: `EnsureWorkflowRule(workspace)`를 `agy` 분기 밖으로 이동해 모든 `launcher.Run`에서 항상 생성.

#### 2. `[DEFECT-02]` `guard handoff reject <id> -m "<reason>"` 파싱 실패
- **근거**: [handoff.go](file:///home/yundream/mytask/kms/guard/cmd/guard/handoff.go#L245-L259)
  - Go 표준 `flag.FlagSet.Parse(args)`는 첫 번째 위치 인자(`<id>`)를 만나는 즉시 플래그 파싱을 중단합니다.
  - 계약서([05_handoff_protocol_contract.md](file:///home/yundream/mytask/kms/guard/doc/spec/05_handoff_protocol_contract.md#L132)) 및 자체 `usage` 문구(`guard handoff reject <id> [-m "reason"]`)대로 호출해도 `fs.NArg()`가 3이 되어 실패합니다.
- **권장 조치 (코드 수정)**:
  - `runHandoffReject`에서 `<id>`와 `-m`/`--reason` 플래그의 순서에 무관하게 파싱되도록 수정 (`reject <id> -m "reason"` 및 `reject -m "reason" <id>` 모두 지원).

#### 3. `[DEFECT-03]` `guard hook`의 `tool_input.command` 자체 승인 차단 우회
- **근거**: [payload.go](file:///home/yundream/mytask/kms/guard/internal/adapters/hook/payload.go#L6-L22), [payload.go](file:///home/yundream/mytask/kms/guard/internal/adapters/hook/payload.go#L49-L74)
  - `hook.Input`이 `toolCall.args`, `tool_args`, 최상위 `CommandLine`/`command_line`만 파싱하고 `tool_input` 필드를 받지 않아, `{"tool_name":"run_command","tool_input":{"command":"guard handoff approve hnd-1"}}` 입력 시 `ExtractCommandLine()`이 `""`을 반환하며 `allow`로 통과합니다.
- **권장 조치 (코드 수정 + 계약서 반영)**:
  - [payload.go](file:///home/yundream/mytask/kms/guard/internal/adapters/hook/payload.go#L6-L74)의 `Input`에 `ToolInput map[string]interface{} \`json:"tool_input"\``을 추가하고, `ExtractCommandLine()`(`CommandLine`, `command_line`, `command`, `cmd`) 및 `ExtractTargetPath()`(`Path`, `path`, `file_path`)에서 `ToolInput`을 함께 추출하도록 보완.
  - [03_adapters_cli_contract.md](file:///home/yundream/mytask/kms/guard/doc/spec/03_adapters_cli_contract.md#L12-L28)의 `Input` DTO에도 `ToolInput` 필드 추가.

---

`DEFECT-02`, `DEFECT-03` 코드 수정과 함께 `DEFECT-01`은 **안 A(명세 명확화)** 또는 **안 B(항상 생성)** 중 어느 쪽으로 진행할까요?

---

## 🧑‍💻 User · `2026-11-21 23:44:40` {#step-39}

수정방향이 명확한 DEFECT-02 와 DEFECT-03 부터 작업진행해주세요.

---

## 🧑‍💻 User · `2026-11-21 23:45:41` {#step-50}

테스트는 당신의 역할이 아닙니다.

---

## 🤖 Antigravity · `2026-11-21 23:45:41` {#step-51}

알겠습니다. 테스트 실행 없이 `DEFECT-02`, `DEFECT-03` 수정 내역만 보고드립니다.

### 수정 완료 내역

1. **`DEFECT-02` (`guard handoff reject <id> -m "<reason>"` 플래그 파싱)**
   - [handoff.go](file:///home/yundream/mytask/kms/guard/cmd/guard/handoff.go#L253-L297): `normalizeRejectArgs` 헬퍼를 추가하여 위치 인자(`<id>`) 뒤에 `-m`/`--reason` 플래그가 오는 경우와 앞에 오는 경우 모두 정상 파싱되도록 수정했습니다.
2. **`DEFECT-03` (`guard hook`의 `tool_input.command` 자체 승인 차단 우회)**
   - [payload.go](file:///home/yundream/mytask/kms/guard/internal/adapters/hook/payload.go#L16-L80): `hook.Input`에 `ToolInput` (`json:"tool_input"`) 필드를 추가하고, `ExtractCommandLine()` 및 `ExtractTargetPath()`가 `tool_input` 내부의 `command`/`cmd`/`command_line`/`CommandLine` 및 `path`/`Path`/`file_path`를 추출하도록 보완했습니다.
   - [03_adapters_cli_contract.md](file:///home/yundream/mytask/kms/guard/doc/spec/03_adapters_cli_contract.md#L22-L26): `hook.Input` DTO 계약에 `ToolInput` 필드를 반영했습니다.

`DEFECT-01`에 대한 방향(안 A: `agy` 전용 유지 및 명세 명확화 vs 안 B: 모든 `launcher.Run`에서 생성)도 결정해 주시면 바로 반영하겠습니다.

---

## 🧑‍💻 User · `2026-11-21 23:46:05` {#step-52}

A 안으로 해주세요.

---

## 🤖 Antigravity · `2026-11-21 23:46:18` {#step-56}

`DEFECT-01`에 대해 **안 A(`agy` 전용 부트스트랩 유지 및 명세 명확화)** 반영을 완료했습니다.

### 수정 내역
- [03_adapters_cli_contract.md](file:///home/yundream/mytask/kms/guard/doc/spec/03_adapters_cli_contract.md#L75-L77): `launcher.Run` 계약에 `EnsureWorkspaceHook` 및 `EnsureWorkflowRule`은 일반 셸 명령 실행 시 워크스페이스 오염을 방지하기 위해 `filepath.Base(cmdArgs[0]) == "agy"`일 때만 자동 실행됨을 명시
- [handoff-qa-test-brief.md](file:///home/yundream/mytask/kms/guard/doc/test/handoff-qa-test-brief.md#L393): `TC-ROUTE-06`의 `.agents/rules/guard-workflow.md` 생성 조건을 `agy` 실행 시 한정으로 정정 (`EnsureWorkflowRule` 또는 `agy` 이름의 실행 파일로 별도 검증)

필요하시면 `DEFECT-01` ~ `DEFECT-03` 조치 결과를 [handoff-qa-report.md](file:///home/yundream/mytask/kms/guard/doc/test/handoff-qa-report.md)에 Append 해드릴까요?

---

## 🧑‍💻 User · `2026-11-21 23:46:45` {#step-57}

승인

---

## 🤖 Antigravity · `2026-11-21 23:46:57` {#step-60}

[handoff-qa-report.md](file:///home/yundream/mytask/kms/guard/doc/test/handoff-qa-report.md) 하단에 `5. 개발자 검토 및 조치 결과 — DEFECT-01 ~ DEFECT-03 (2026-11-21)` 섹션을 추가(Append)했습니다.

- **`DEFECT-01` (`TC-ROUTE-06`)**: `agy` 전용 부트스트랩 명세 명확화 ([03_adapters_cli_contract.md](file:///home/yundream/mytask/kms/guard/doc/spec/03_adapters_cli_contract.md#L76-L77), [handoff-qa-test-brief.md](file:///home/yundream/mytask/kms/guard/doc/test/handoff-qa-test-brief.md#L393))
- **`DEFECT-02` (`TC-CLI-02`, `03`)**: `reject` 위치 인자 뒤 플래그 파싱 수정 ([handoff.go](file:///home/yundream/mytask/kms/guard/cmd/guard/handoff.go#L253-L297))
- **`DEFECT-03` (`TC-CLI-04`)**: `hook.Input` `tool_input` 필드 및 추출 지원 추가 ([payload.go](file:///home/yundream/mytask/kms/guard/internal/adapters/hook/payload.go#L16-L80), [03_adapters_cli_contract.md](file:///home/yundream/mytask/kms/guard/doc/spec/03_adapters_cli_contract.md#L22-L26))

---

## 🧑‍💻 User · `2026-11-21 23:47:05` {#step-89}

개발자가 수정 조치했다고 합니다. 레포트 문서 읽어서 테스트 해주세요.

---

## 🤖 Antigravity · `2026-11-22 00:16:29` {#step-100}

개발자 수정 조치 내역([handoff-qa-report.md](file:///mnt/data/mytask/kms/guard/doc/test/handoff-qa-report.md#L165-L185))을 확인하고, `TC-ROUTE-06`을 정정된 명세에 맞게 갱신한 뒤 Stage 1 ~ Stage 6 전체 스위트를 재검증했습니다.

### 최종 재검증 결과: **32 / 32 ALL PASS**
- **수정 파일**: [handoff_route_test.go](file:///mnt/data/mytask/kms/guard/test/e2e/handoff_route_test.go) (`sh` 실행 시 미생성 + `agy` 실행 시 `.agents/rules/guard-workflow.md` 자동 생성 검증 추가)
- **실행 명령**: `go test -count=1 -race -v ./test/e2e/... -run "TestHandoff"`
- **소요 시간**: `4.442s` (데이터 레이스 없음)

| 결함 ID | 대상 TC | 수정 및 검증 내용 | 재검증 결과 |
| :--- | :--- | :--- | :---: |
| **DEFECT-01** | `TC-ROUTE-06` | `sh` 실행 시 `.agents/rules/guard-workflow.md` 미생성, `agy` 실행 시 자동 생성 확인 | **PASS** |
| **DEFECT-02** | `TC-CLI-02`, `TC-CLI-03` | `guard handoff reject <id> -m "<reason>"` 위치 인자 뒤 플래그 파싱 및 `inbox`/`ack`/`reject` 시나리오 통과 | **PASS** |
| **DEFECT-03** | `TC-CLI-04` | `guard hook` 바이너리 `tool_input.command` 페이로드 자체 승인 차단(`decision: deny`, `stderr` 사유 출력) 확인 | **PASS** |

[handoff-qa-report.md](file:///mnt/data/mytask/kms/guard/doc/test/handoff-qa-report.md)와 [handoff-qa-test-brief.md](file:///mnt/data/mytask/kms/guard/doc/test/handoff-qa-test-brief.md#L191-L198) 체크리스트를 최종 **ALL PASS** 상태로 마감 업데이트할까요?

---

## 🧑‍💻 User · `2026-11-22 00:17:04` {#step-61}

commit & push 해주세요.

---

## 🤖 Antigravity · `2026-11-22 00:19:33` {#step-68}

커밋 및 푸시를 완료했습니다.

- **커밋**: `6ac2338` (`feat: implement handoff protocol and resolve QA defects (DEFECT-01~03)`)
- **변경 규모**: 40 files changed, 6087 insertions(+), 5156 deletions(-)
- **푸시**: `cb80277..6ac2338 main -> main` (`joinc-dream:joincdream/guard.git`)

---

## 🧑‍💻 User · `2026-11-22 00:21:00` {#step-69}

Handoff 기능에 대한 사용자 테스트를 하겠습니다. DEV, QA 모드를 기반으로 시나리오 만들어주세요. 이 애플리케이션을 모르는 사용자라도 테스트 할 수 있도록 만들어주세요. 문서는 @[test] 디렉토리에 만들어주세요.

---

## 🤖 Antigravity · `2026-11-22 00:21:55` {#step-72}

DEV ↔ QA 기반 Handoff 사용자 테스트 가이드를 [handoff-user-test-guide.md](file:///home/yundream/mytask/kms/guard/test/handoff-user-test-guide.md)에 작성했습니다.

### 문서 구성
1. **1분 개념 이해**: `guard`, `DEV`/`TEST(QA)` 모드, Handoff(승인 게이트·조건부 Scope Lock·오프라인 대기·자체 승인 차단) 개요 및 흐름도
2. **사전 준비**: 기존 `~/.guard`를 건드리지 않는 격리 환경(`mktemp`) 생성, 샘플 파일 준비, 빌드, 3개 터미널(`guard top` / `guard dev sh` / `guard test sh`) 배치 안내
3. **6단계 실습 시나리오** (복사·붙여넣기 명령어 및 기대 출력 포함):
   - **시나리오 1**: DEV → QA `verify` 요청, 승인 전 물리 격리, TUI `[V]` 문서 열람 및 `[Enter]` 승인, `ack`
   - **시나리오 2**: QA → DEV `fix` 요청(`--parent` 스레드 연결), 조건부 `Scope Locked` 및 세션 `allowed_write_paths` 확인
   - **시나리오 3**: TUI `[Esc]` 및 CLI `guard handoff reject <id> -m "..."` 반려 및 인박스 미전달 확인
   - **시나리오 4**: `guard hook`을 통한 에이전트 자체 승인(`approve`) 직접·우회 차단 및 정상 명령 허용 확인
   - **시나리오 5**: DEV 오프라인 상태 승인 보관 → 재시작 시 자동 배달 → 종료 시 `.inbox` 자동 정리 확인
   - **시나리오 6**: `guard top` 단축키(`[H]` 큐 패널 전환, `[?]` 도움말, `[q]` 종료)
4. **환경 정리 및 결과 체크리스트**

---

## 🧑‍💻 User · `2026-11-22 00:23:08` {#step-73}

사용자 테스트 입니다. 빌드 방법은 빼주세요.

---

## 🤖 Antigravity · `2026-11-22 00:23:25` {#step-76}

[handoff-user-test-guide.md](file:///home/yundream/mytask/kms/guard/test/handoff-user-test-guide.md#L42-L97)에서 `go build` 및 임시 `PATH` 설정 등 빌드 관련 단계를 제거하고, 설치된 `guard` 명령을 바로 사용하도록 수정했습니다.

---

## 🧑‍💻 User · `2026-11-22 00:25:27` {#step-77}

테스트 환경 초기화 ? 애플리케이션을 모르는 사용자에게 Step 0-1 을 요청한다고요 ? 이것은 둘 중 하나 입니다. 테스트 가이드가 잘못됐거나, 애플리케이션 온보딩 기능이 충분하지 않거나 어느쪽인가요.

---

## 🤖 Antigravity · `2026-11-22 00:25:27` {#step-78}

**일차적으로는 테스트 가이드가 잘못됐습니다.** (다만 애플리케이션 온보딩 관점의 한계도 일부 드러납니다.)

### 1. 테스트 가이드의 문제 (주원인)
- `guard`는 `~/.guard/sessions`, `~/.guard/handoff`를 최초 실행 시 자동 생성하므로, **사용자가 `mktemp`, `HOME` 덮어쓰기, 샘플 파일 생성 같은 초기화 스크립트를 실행할 필요가 없습니다.**
- 개발자/QA의 격리 테스트 습관을 일반 사용자 테스트 가이드에 그대로 옮겨 놓은 것이 잘못입니다.
- 사용자 테스트 가이드라면 그냥 작업 폴더에서 바로 아래처럼 시작했어야 합니다:
  - 터미널 1: `guard top`
  - 터미널 2: `guard dev agy` (또는 `guard dev sh`)
  - 터미널 3: `guard test agy` (또는 `guard test sh`)

### 2. 애플리케이션 온보딩 관점의 한계 (부수적 원인)
- `guard`의 Handoff는 원래 `agy` 에이전트가 `.agents/rules/guard-workflow.md`를 읽고 `send`/`ack` 및 `$GUARD_INBOX_PATH` 확인을 대신 수행하도록 설계되어 있습니다.
- 그래서 **에이전트 없이 사람이 직접 기능을 탐색**하려면 `guard handoff send`의 옵션이나 `$GUARD_INBOX_PATH` 경로를 알아야 하고, `guard top` 첫 화면에서도 "다음에 무엇을 실행하면 되는지" 안내하는 온보딩/빈 상태(Empty State) 가이드가 부족합니다.

---

원하시면 [handoff-user-test-guide.md](file:///home/yundream/mytask/kms/guard/test/handoff-user-test-guide.md)에서 불필요한 `Step 0-1` 초기화 스크립트를 모두 제거하고, 실제 사용자 흐름(`guard top` + `guard dev` + `guard test`) 중심으로 바로 시작하도록 다시 작성하겠습니다.

---

## 🧑‍💻 User · `2026-11-22 00:27:19` {#step-79}

다시 작성해주세요.

---

## 🤖 Antigravity · `2026-11-22 00:27:48` {#step-83}

[handoff-user-test-guide.md](file:///home/yundream/mytask/kms/guard/test/handoff-user-test-guide.md)를 초기화 스크립트와 빌드 단계 없이 실제 사용자 흐름 중심으로 다시 작성했습니다.

### 변경 사항
- **사전 초기화/빌드 제거**: `mktemp`, `HOME` 덮어쓰기, 샘플 파일 생성 스크립트를 모두 삭제하고 작업 폴더에서 터미널 3개(`guard top`, `guard dev agy`, `guard test agy`)로 바로 시작하도록 구성했습니다.
- **실제 사용 방식 중심**: `agy` 자연어 지시 흐름을 기본으로 두고, 에이전트 없이 체험할 수 있는 `sh` CLI 명령을 보조로 병기했습니다.
- **6개 핵심 시나리오 + 체크리스트**: DEV→QA 검증 요청, QA→DEV `fix` 및 Scope Lock, 요청 반려, 자체 승인 차단, 오프라인 대기·자동 전달, 관제탑 단축키 탐색.

---

## 🧑‍💻 User · `2026-11-22 00:30:37` {#step-84}

애플리케이션을 처음 사용하는 사용자를 기준으로 해주세요. 처음 사용하는 사용자가 guard handoff send --to test ... 이걸 입력한다고요 ?

---

## 🧑‍💻 User · `2026-11-22 00:31:25` {#step-87}

다시 한번 말합니다. 애플리케이션을 처음 사용해보는 사용자입니다. 만약 사용자가 충분히 쉽게 테스트를 못하게 되어 있다면 그것은 애플리케이션 문제입니다. 그럴 경우 보고해주세요.

---

## 🤖 Antigravity · `2026-11-22 00:31:31` {#step-91}

말씀이 정확합니다. **처음 사용하는 사용자가 `guard handoff send --to test ...` 같은 명령어를 직접 입력해야 한다면, 그것은 애플리케이션 UX/온보딩 문제입니다.**

현재 코드([launcher.go](file:///home/yundream/mytask/kms/guard/internal/adapters/launcher/launcher.go#L86-L97), [bootstrap.go](file:///home/yundream/mytask/kms/guard/internal/adapters/launcher/bootstrap.go#L110-L126))를 기준으로 확인된 **애플리케이션 측 문제점**을 보고드립니다.

---

### 확인된 애플리케이션 문제점 (처음 사용하는 사용자 관점)

1. **사람이 직접 체험/테스트할 수 있는 진입점 부재**
   - `guard handoff send --to ... --type ... --ref ... --targets ... -m "..."` 및 `guard handoff ack <id>`는 원래 사용자가 치는 명령이 아니라, `agy` 에이전트가 `.agents/rules/guard-workflow.md`를 읽고 대신 실행하도록 설계된 **에이전트용 CLI**입니다.
   - 사용자가 `agy` 대화만으로 테스트하지 않고 기능을 확인하려 하면, 복잡한 플래그와 내부 환경변수(`$GUARD_INBOX_PATH`), `guard hook` JSON 포맷까지 알아야 하는 구조입니다.

2. **승인 후 수신 세션 자동 인지 불가 (UX 단절)**
   - 사용자가 `guard top`에서 `[Enter]`로 승인하면 `~/.guard/sessions/<id>.inbox` 파일에 기록될 뿐, 실행 중인 수신 `agy` 세션은 새 작업이 도착했다는 사실을 자동으로 알지 못합니다.
   - 결국 사용자가 수신 터미널 창으로 직접 이동해 *"인박스 확인해서 작업해줘"*라고 다시 말해야만 다음 단계가 진행됩니다.

3. **초기 실행 온보딩 안내 부족 (`guard top` / 배너)**
   - 처음 `guard top`을 켰을 때 활성 세션이나 핸드오프가 없으면 사용자가 다음에 무엇을 실행해야 하는지, 승인 후 수신 세션에서 어떻게 이어가야 하는지 화면 내 안내가 부족합니다.

4. **부트스트랩이 `agy` 명령어 이름에만 고정**
   - [launcher.go](file:///home/yundream/mytask/kms/guard/internal/adapters/launcher/launcher.go#L86-L97)에서 `filepath.Base(cmdArgs[0]) == "agy"`일 때만 Hook과 `.agents/rules/guard-workflow.md`가 자동 설치되므로, 다른 실행 방식에서는 워크플로우가 자동 연결되지 않습니다.

---

### 제안 방향

1. **사용자 테스트 가이드([handoff-user-test-guide.md](file:///home/yundream/mytask/kms/guard/test/handoff-user-test-guide.md)) 수정**:
   - 사용자가 내부 CLI(`guard handoff send/ack/reject`, `$GUARD_INBOX_PATH`, `guard hook`)를 **단 한 줄도 직접 입력하지 않도록** 변경
   - 사용자가 하는 일은 오직 **① `guard top` / `guard dev agy` / `guard test agy` 실행 → ② 에이전트에게 자연어로 지시 → ③ `guard top`에서 `[V]`/`[Enter]`/`[Esc]` 조작**만으로 구성 (이 과정에서 막히는 부분이 나오면 그대로 앱 결함으로 기록)
2. **애플리케이션 UX 개선 (필요 시)**:
   - `guard top` 빈 상태/승인 직후 다음 행동 안내 추가 등 온보딩 보완

먼저 테스트 가이드를 **CLI 명령어 입력 없는 순수 자연어 + `guard top` 조작 시나리오**로 고쳐드릴까요, 아니면 애플리케이션 UX 개선을 먼저 논의할까요?

---

## 🧑‍💻 User · `2026-11-22 00:32:53` {#step-92}

1 먼저 진행해주세요.

---

## 🤖 Antigravity · `2026-11-22 00:33:15` {#step-96}

[handoff-user-test-guide.md](file:///home/yundream/mytask/kms/guard/test/handoff-user-test-guide.md)를 **처음 사용하는 사용자 기준**으로 전면 재작성했습니다.

### 변경 요약
- **내부 CLI 명령어 완전 제거**: `guard handoff send/ack/reject`, `$GUARD_INBOX_PATH`, `guard hook` 파이프 명령어를 모두 없앴습니다.
- **순수 사용자 행동만으로 구성**:
  1. 터미널 3개 실행 (`guard top`, `guard dev agy`, `guard test agy`)
  2. 에이전트에게 자연어로 지시
  3. `guard top`에서 키보드 조작 (`[V]`, `[Enter]`, `[Esc]`, `[H]`, `[?]`, `[q]`)
- **결함 기록 중심**: 자연어 지시나 화면 조작 중 막히거나 헷갈리는 지점을 그대로 앱 개선 포인트로 기록할 수 있도록 체크리스트를 구성했습니다.

---

## 🧑‍💻 User · `2026-11-22 00:33:39` {#step-97}

처음 애플리케이션 사용자라고 했습니다. 애플리케이션을 실행하는 것 부터 해야죠.

---

## 🤖 Antigravity · `2026-11-22 00:33:54` {#step-100}

[handoff-user-test-guide.md](file:///home/yundream/mytask/kms/guard/test/handoff-user-test-guide.md#L10-L51)의 `1. 애플리케이션 처음 실행하기` 섹션을 첫 실행 단계부터 순서대로 보강했습니다.

- **Step 1**: `guard` 최초 실행 및 도움말 화면 확인
- **Step 2**: 프로젝트 폴더 이동 후 `guard top` 관제탑 실행 및 초기 화면 확인
- **Step 3**: 새 터미널에서 `guard dev agy` 실행 → 배너 확인 및 관제탑 `DEV` 세션 등록 확인
- **Step 4**: 세 번째 터미널에서 `guard test agy` 실행 → 배너 확인 및 관제탑 `DEV`/`TEST` 2개 세션 등록 확인

---

## 🧑‍💻 User · `2026-11-22 10:10:25` {#step-0}

ALL 모드로 변경해줘.

---

## 🤖 Antigravity · `2026-11-22 10:11:24` {#step-13}

`./guard all`을 실행하여 워크스페이스 및 현재 활성 세션(`85428527`)의 모드를 **ALL**로 변경했습니다.

- **변경 전**: `TEST`
- **변경 후**: `ALL` (모든 파일 열람 및 수정 제한 해제)

---

## 🧑‍💻 User · `2026-11-22 10:11:35` {#step-14}

QA 결과 릴리즈전에 README.md 파일에 대한 영어 문서를 만들어야 할 것 같습니다. READEME.md 파일을 수정하도록 DEV 세션에 fix 핸드오프 보내줘.

---

## 🤖 Antigravity · `2026-11-22 10:12:06` {#step-20}

DEV 세션으로 `fix` 핸드오프를 전송(큐 등록)했습니다.

- **Handoff ID**: `hnd-20261122-046621e0`
- **유형**: `FIX` (`-> DEV`)
- **대상 파일 (`--targets`)**: [README.md](file:///mnt/data/mytask/kms/guard/README.md)
- **요약**: 릴리즈 전 README.md 영어 문서 작성 및 수정 요청

`guard top` 또는 `guard handoff approve hnd-20261122-046621e0`으로 승인하면 DEV 세션 인박스로 전달됩니다.

---

## 🧑‍💻 User · `2026-11-22 10:12:48` {#step-0}

도착한 핸드오프 확인해줘.

---

## 🤖 Antigravity · `2026-11-22 10:13:38` {#step-9}

도착한 핸드오프 1건이 확인되었습니다.

| 항목 | 내용 |
| :--- | :--- |
| **ID** | `hnd-20261122-046621e0` |
| **유형 (Type)** | `fix` |
| **상태 (Status)** | `approved` |
| **발신 (From)** | `all` (session: `85428527`) |
| **수신 (To)** | `dev` (session: `98b388bc`) |
| **대상 (Targets)** | [README.md](file:///home/yundream/mytask/kms/guard/README.md) |
| **생성 / 갱신 시각** | `2026-11-22 10:12:06` / `10:12:23` (KST) |

- **요약**: 릴리즈 전 README.md 영어 문서 작성 및 수정 요청
- **상세 내용**: QA 결과 릴리즈 전에 README.md 파일에 대한 영어 문서를 만들어야 합니다. README.md 파일을 수정해 주세요.

---

## 🧑‍💻 User · `2026-11-22 10:14:54` {#step-10}

한글 README.md 와 영문 README.md 가 모두 있어야 할 것 같은데요. 영문 README.md 를 메인으로 노출하고 한글 / 영문 링크를 넣으면 어떨까요.

---

## 🤖 Antigravity · `2026-11-22 10:14:54` {#step-11}

좋은 방향입니다. GitHub 등 오픈소스 프로젝트에서도 가장 표준적인 구성입니다.

다음 구조로 정리하면 깔끔할 것 같습니다.

- **`README.md`**: 영문 메인 문서 (상단에 `English | [한국어](README.ko.md)` 링크 배치)
- **`README.ko.md`**: 기존 한글 문서 유지 (상단에 `[English](README.md) | 한국어` 링크 배치)

이 방향으로 바로 작업 진행할까요? (기존 [README.md](file:///home/yundream/mytask/kms/guard/README.md)를 `README.ko.md`로 옮기고 영문 `README.md` 작성)

---

## 🧑‍💻 User · `2026-11-22 10:15:12` {#step-12}

승인

---

## 🤖 Antigravity · `2026-11-22 10:15:57` {#step-18}

작업을 완료했습니다.

- [README.md](file:///home/yundream/mytask/kms/guard/README.md): 영문 메인 문서로 교체하고 상단에 `**English** | [한국어](README.ko.md)` 전환 링크를 추가했습니다.
- [README.ko.md](file:///home/yundream/mytask/kms/guard/README.ko.md): 기존 한글 문서를 보존하고 상단에 `[English](README.md) | **한국어**` 전환 링크를 추가했습니다.
- 두 문서 하단의 설계 문서 링크도 절대 경로(`file:///...`)에서 상대 경로(`doc/...`)로 정리했습니다.

핸드오프(`hnd-20261122-046621e0`) 확인 처리(`ack`)나 완료 회신이 필요하시면 말씀해 주세요.

---

## 🧑‍💻 User · `2026-11-22 10:18:07` {#step-19}

작업을 끝냈으니 DEV 세션에 작엄 조치 보고서와 함께 핸드오프 해줘.

---

## 🤖 Antigravity · `2026-11-22 10:18:17` {#step-23}

전송 대상을 한 번만 확인하겠습니다. 현재 활성 세션은 다음과 같습니다.

- **`98b388bc` (DEV)**: 기존 핸드오프(`hnd-20261122-046621e0`)의 **수신 세션**
- **`85428527` (ALL)**: 기존 핸드오프(`hnd-20261122-046621e0`)의 **발신 세션**

작업 조치 보고서를 **발신 세션인 `ALL` (`85428527`)**로 회신할까요, 아니면 말씀하신 대로 **`DEV` (`98b388bc`)**로 전송할까요?

---

## 🧑‍💻 User · `2026-11-22 10:18:44` {#step-24}

ALL로 회신하세요.

---

## 🤖 Antigravity · `2026-11-22 10:18:59` {#step-27}

`ALL` 세션(`85428527`)으로 작업 조치 보고서 회신 핸드오프를 전송했습니다.

- **기존 핸드오프 완료 처리 (`ack`)**: `hnd-20261122-046621e0`
- **신규 핸드오프 ID**: `hnd-20261122-d65609ea`
- **유형 / 수신 대상**: `verify` → `85428527` (`ALL`)
- **참조 파일**: [README.md](file:///home/yundream/mytask/kms/guard/README.md), [README.ko.md](file:///home/yundream/mytask/kms/guard/README.ko.md)
- **요약**: `README.md 영문 문서 작성 및 한/영 문서 분리 조치 완료` (조치 내역 및 변경 파일 포함)

---

## 🧑‍💻 User · `2026-11-22 10:19:24` {#step-21}

DEV 세션으로 부터 작업 조치 문서가 도착했습니다. 검토해주세요.

---

## 🤖 Antigravity · `2026-11-22 10:19:45` {#step-29}

DEV 세션(`98b388bc`)에서 도착한 검증 핸드오프(`hnd-20261122-d65609ea`, Parent: `hnd-20261122-046621e0`)와 변경된 문서를 검토했습니다.

### 1. 조치 내역 확인
- **[README.md](file:///mnt/data/mytask/kms/guard/README.md)** (수정, 176줄): 영문 메인 문서로 전면 번역 및 상단 언어 전환 링크(`**English** | [한국어](README.ko.md)`) 추가
- **[README.ko.md](file:///mnt/data/mytask/kms/guard/README.ko.md)** (신규, 176줄): 기존 한글 문서 보존 및 상단 언어 전환 링크(`[English](README.md) | **한국어**`) 추가
- **하단 설계 문서 링크**: 두 문서 모두 상대 경로(`doc/01_requirements.md` ~ `doc/04_development_plan.md`)로 정리됨

### 2. 검토 결과
- **구조 및 번역 품질 (양호)**: [README.md](file:///mnt/data/mytask/kms/guard/README.md)와 [README.ko.md](file:///mnt/data/mytask/kms/guard/README.ko.md)가 176줄 1:1 구조로 완전히 대응되며, Mermaid 다이어그램, CLI/TUI 안내, `.agents/hooks.json`, `guard-policy.yaml` 예시 및 에러 메시지 번역 모두 자연스럽고 정확합니다.
- **링크 정합성 (양호)**: 한/영 상호 링크 및 `doc/01_requirements.md`~`04_development_plan.md` 4개 상대 경로 링크 모두 실제 파일과 일치합니다.
- **릴리즈 전 참고 사항 (선택)**:
  - 이번 조치는 기존 한글 README 기준의 1:1 영문 번역이므로, `guard handoff`, `guard reset`, `guard doc` 등 최신 서브커맨드 안내는 두 문서 모두 포함되어 있지 않습니다. 릴리즈 문서에 포함이 필요하면 추가 업데이트를 검토하시면 됩니다.
  - 현재 `git status`에 `.obsidian/workspace.json`, `chat/guard.md`, `.agents/rules/`, `test/handoff-user-test-guide.md` 등 다른 변경 파일도 존재하므로 커밋 시 `README.md`, `README.ko.md`만 선별하는 것이 안전합니다.

### 3. 다음 단계
- 검토 결과에 이상이 없으시면 `guard handoff ack hnd-20261122-d65609ea`로 완료 처리할까요?

---

## 🧑‍💻 User · `2026-11-22 10:20:48` {#step-30}

승인

---

## 🤖 Antigravity · `2026-11-22 10:21:29` {#step-34}

핸드오프 `hnd-20261122-d65609ea`를 완료(`ack`) 처리했습니다.

---

## 🧑‍💻 User · `2026-11-22 10:22:01` {#step-35}

TEST 모드로 전환해주세요.

---

## 🤖 Antigravity · `2026-11-22 10:23:18` {#step-38}

`./guard test`를 실행하여 워크스페이스 및 현재 활성 세션(`85428527`)의 모드를 **TEST**로 전환했습니다.

- **변경 전**: `ALL`
- **변경 후**: `TEST` (프로덕션 코드 열람 차단, 계약 및 테스트 파일만 허용)

---

## 🧑‍💻 User · `2026-11-22 10:23:16` {#step-28}

guard handoff 를 지시하면 AI agent 가 guard 바이너리 확인하고 help 확인하는 등에 쓸데 없는 시간을 쓰는 것 같습니다. SKILL 등록이 부족한 건가요 ? 뭐가 문제죠 ? 처음 한 번 정도는 guard를 실행한 적이 없으니 확인한다고 해도, 그 뒤에도 계속 반복해서 확이하는 느낌입니다.

---

## 🤖 Antigravity · `2026-11-22 10:23:16` {#step-29}

실제 이번 대화에서도 같은 현상이 발생했는데, 현재 컨텍스트 기준으로 확인되는 원인은 크게 **두 가지**입니다.

---

### 1. `guard` 스킬의 트리거(Description)에 `handoff`가 빠져 있음
현재 등록된 `guard` 스킬의 설명을 보면 다음과 같이 **정책 파일(`guard-policy.yaml`) 전용**으로 한정되어 있습니다.

> `guard` (`/home/yundream/.gemini/antigravity-cli/skills/guard/SKILL.md`): Antigravity CLI 가드레일(guard) 정책 파일(`guard-policy.yaml`)을 프로젝트 구조에 맞게 자동 생성·수정·검증할 때 사용하는 스킬. 사용자가 'guard 정책 만들어줘', '가드레일 규칙 추가', 'guard-policy.yaml 수정' 등을 요청할 때 활성화하십시오.

따라서 사용자가 *"도착한 핸드오프 확인해줘"* 또는 *"핸드오프 보내줘"*라고 요청해도 에이전트는 `guard` 스킬을 활성화하지 않습니다.

### 2. 세션 간 CLI 문맥이 유지되지 않음 (매번 처음부터 탐색)
`guard handoff` 서브커맨드 스펙(`inbox`, `send`, `ack`, `status` 등)과 바이너리 위치(`guard` vs `./guard`)가 **시스템 프롬프트(Rules)**에 주입되어 있지 않습니다.
- 에이전트는 새 대화가 시작될 때마다 `handoff`가 파일인지 CLI인지, 바이너리가 어디 있는지, 서브커맨드가 `list`인지 `inbox`인지 모르는 상태가 됩니다.
- 그 결과 매 세션마다 `ls/find` → `guard handoff --help` → 실제 실행 순으로 2~3턴을 낭비하게 됩니다.

---

### 해결 방법

| 방식 | 특징 | 추천 용도 |
| :--- | :--- | :--- |
| **1. Rules에 핵심 커맨드 요약 등록** (글로벌 또는 `.agents/rules/`) | 매 세션 시작 시 **자동 주입**되므로 `SKILL.md` 읽기나 `--help` 호출 없이 **첫 턴에 즉시 실행**됨 | 자주 쓰는 `guard status`, `guard handoff inbox/send/ack` 문법과 바이너리 호출 규칙(`guard` 직접 호출) |
| **2. Skill 확장 / 분리** (`guard` 스킬 description 수정 또는 `guard-handoff` 스킬 신설) | 세션당 최초 1회 `SKILL.md`를 읽고 실행 (탐색/`--help`는 사라짐) | 상세한 핸드오프 보고서 템플릿, 예외 처리 절차 등 분량이 긴 가이드 |

**가장 효과적인 조합**은:
1. **Rules**에 5~10줄 정도로 `guard` 바이너리 호출 방식과 `guard handoff inbox --json`, `send`, `ack`, `guard status`의 정확한 플래그만 짧게 상시 주입해 두고,
2. 상세 워크플로우가 필요할 때만 **Skill**을 트리거하도록 구성하는 것입니다.

원하시면 현재 워크스페이스의 `.agents/rules/guard-workflow.md`나 `guard` 스킬 내용을 확인해서 바로 정리해 드릴 수 있습니다.

---

## 🧑‍💻 User · `2026-11-22 10:24:30` {#step-30}

.agents/rules 의 역할은 무엇인가요.

---

## 🤖 Antigravity · `2026-11-22 10:24:47` {#step-37}

[agy-customizations/SKILL.md](file:///home/yundream/.gemini/antigravity-cli/builtin/skills/agy-customizations/SKILL.md) 및 [docs/rules.md](file:///home/yundream/.gemini/antigravity-cli/builtin/skills/agy-customizations/docs/rules.md) 기준으로 설명드립니다.

### 1. `.agents/rules/`의 역할
- **프로젝트(워크스페이스) 전용 규칙 저장소**입니다.
- 에이전트가 현재 작업 디렉토리에서 저장소 루트(`.git` 기준)까지 탐색하며 `.agents/rules/*.md` 파일을 발견해 적용합니다.
- VCS에 커밋해 프로젝트별 코딩 스타일, 제약사항, 워크플로우 규칙을 공유할 때 사용합니다.
- `trigger: always_on` 규칙은 대화 시작 시 시스템 프롬프트(`<user_rules>`)에 무조건 주입되며, 조건부(`trigger: model_decision` 등) 규칙은 필요 시에만 로드됩니다.

---

### 2. 방금 문제의 진짜 원인 (중요)
확인해 보니 이 워크스페이스에는 이미 [guard-workflow.md](file:///home/yundream/mytask/kms/guard/.agents/rules/guard-workflow.md)에 `guard handoff inbox`, `ack`, `send` 사용법이 **잘 작성되어 있습니다.**

그런데 **이번 세션의 `<user_rules>`에는 글로벌 룰(`<RULE[user_global]>`)만 주입되었고, `guard-workflow.md`는 아예 로드되지 않았습니다.** 원인은 다음 두 가지로 보입니다.

1. **[guard-workflow.md](file:///home/yundream/mytask/kms/guard/.agents/rules/guard-workflow.md)에 YAML frontmatter(`trigger: always_on`)가 없음**
   - [SKILL.md](file:///home/yundream/.gemini/antigravity-cli/builtin/skills/agy-customizations/SKILL.md#L91-L92)에 따르면 `always_on` 규칙만 무조건 컨텍스트에 주입됩니다.
2. **또는 `GEMINI.md` / `AGENTS.md`와 동작 차이**
   - [docs/rules.md](file:///home/yundream/.gemini/antigravity-cli/builtin/skills/agy-customizations/docs/rules.md#L12-L20)에는 `GEMINI.md` / `AGENTS.md`가 frontmatter 없이도 해당 디렉토리 범위에서 **항상 활성화(always active)** 된다고 명시되어 있습니다.

즉, 스킬이 부족한 문제라기보다 **이미 만들어 둔 `.agents/rules/guard-workflow.md`가 세션 시작 시 컨텍스트에 주입되지 않고 있었던 것**이 에이전트가 매번 `guard --help`를 반복 확인하던 직접적인 원인입니다.

---

### 해결 방안
- [guard-workflow.md](file:///home/yundream/mytask/kms/guard/.agents/rules/guard-workflow.md) 상단에 `trigger: always_on` frontmatter를 추가하거나,
- 프로젝트 루트에 `AGENTS.md` (또는 `GEMINI.md`)로 두어(또는 참조해) 세션 시작 시 무조건 주입되도록 하면 첫 턴부터 탐색 없이 `guard handoff`를 바로 실행합니다.

---

## 🧑‍💻 User · `2026-11-22 10:26:38` {#step-38}

guard-workflow.md 상단에 프론트매너 추가해주세요.

---

## 🤖 Antigravity · `2026-11-22 10:27:03` {#step-41}

[guard-workflow.md](file:///home/yundream/mytask/kms/guard/.agents/rules/guard-workflow.md#L1-L5) 상단에 `trigger: always_on` YAML frontmatter를 추가했습니다.

```yaml
---
trigger: always_on
---
```

새 세션 시작 시부터 해당 규칙이 자동으로 주입됩니다.

---

## 🧑‍💻 User · `2026-11-22 10:27:21` {#step-42}

guard-workdflow.md 는 언제 LLM에 주입되는 건가요.

---

## 🤖 Antigravity · `2026-11-22 10:27:21` {#step-43}

매 요청(턴)마다 Antigravity가 시스템 프롬프트의 `<user_rules>` 영역을 구성할 때 주입됩니다.

실제로 방금 [guard-workflow.md](file:///home/yundream/mytask/kms/guard/.agents/rules/guard-workflow.md)에 `trigger: always_on`을 추가하자마자, **바로 이번 턴의 시스템 프롬프트에 `<RULE[/home/yundream/mytask/kms/guard/.agents/rules/guard-workflow.md]>`로 즉시 주입된 것이 확인됩니다.**

- **진행 중인 세션**: 파일 수정 후 **바로 다음 턴부터** 적용
- **새 세션**: **첫 번째 턴부터** 자동 주입

*(직전 답변에서 "새 세션 시작 시부터"라고 안내드린 부분은 정정합니다. 수정 즉시 다음 턴부터 반영됩니다.)*

---

## 🧑‍💻 User · `2026-11-22 10:28:14` {#step-44}

시멘틱레이어라고 보면 되나요.

---

## 🤖 Antigravity · `2026-11-22 10:28:14` {#step-45}

네, **LLM 관점의 경량 시맨틱/컨텍스트 레이어**로 보시면 개념적으로 잘 맞습니다.

- **유사한 점**: 데이터 영역의 시맨틱 레이어가 원시 테이블 위에 비즈니스 용어와 규칙을 매핑해 일관된 해석을 제공하듯, `.agents/rules`는 프로젝트의 원시 파일·CLI(`guard`) 위에 **"핸드오프의 의미, 사용하는 명령어, 지켜야 할 절차"**를 매핑해 LLM이 매번 추측하지 않고 일관되게 행동하도록 돕습니다.
- **차이점**:
  - **Rules (`.agents/rules`)**: LLM 프롬프트에 주입되는 **의미·절차 안내 계층 (Soft Guidance)** — *"무엇을 어떻게 써야 하는지"* 알려줌
  - **Hooks (`guard` PreToolUse)**: 도구 실행을 실제로 가로채 차단하는 **물리적 강제 계층 (Hard Enforcement)** — *"금지된 행동을 실제로 막음"*

즉, `guard` 시스템 전체로 보면 **Rules가 시맨틱(의미·규칙 안내) 레이어**, **Hooks가 실행 통제(가드레일) 레이어**를 각각 맡는 구조입니다.

---

## 🧑‍💻 User · `2026-11-22 10:40:21` {#step-39}

ALL 모드로 전환해 주세요.

---

## 🤖 Antigravity · `2026-11-22 10:40:29` {#step-42}

`./guard all`을 실행하여 워크스페이스 및 현재 활성 세션(`85428527`)의 모드를 **ALL**로 전환했습니다.

- **변경 전**: `TEST`
- **변경 후**: `ALL` (모든 파일 열람 및 수정 제한 해제)

---

## 🧑‍💻 User · `2026-11-22 10:40:57` {#step-43}

개발자가 전달해둔 README.md 파일을 읽어보면, 인터넷에 공개하기에는 내용이 부실한 거 같습니다. 그렇지 않나요 ?

---

## 🤖 Antigravity · `2026-11-22 10:40:57` {#step-44}

네, 맞습니다. 현재 [README.md](file:///mnt/data/mytask/kms/guard/README.md)는 기존 내부용 한글 문서를 1:1로 번역한 수준이어서, **외부 공개(GitHub 등)용 프로젝트 대문 문서로 보기에는 여러 핵심 요소가 빠져 있습니다.**

현재 확인된 내용 기준으로 보완이 필요한 지점은 다음과 같습니다.

### 1. 실제 구현된 주요 기능 및 CLI 명령어 누락
- **핸드오프 프로토콜(`guard handoff`) 미기재**: `send`, `inbox`, `approve`, `reject`, `ack` 등 멀티 에이전트 협업 기능이 완전히 빠져 있습니다.
- **기타 CLI 서브커맨드 누락**: `guard reset <sessionId>`, `guard doc`(정책 템플릿/스키마 출력), `guard all`(워크스페이스 모드 전환), `guard ui`(`top` 별칭) 등이 안내되어 있지 않습니다.
- **정책 로딩 규칙 부재**: `guard-policy.yaml`을 어디에 두어야 적용되는지(탐색 우선순위 및 `GUARD_POLICY_PATH`) 설명이 없습니다.

### 2. 외부 사용자 온보딩(설치/사전 요구사항) 부족
- **사전 요구사항(Prerequisites)** 누락: 필요한 Go 버전, 지원 OS, Antigravity CLI(`agy`) 설치 전제 조건이 없습니다.
- **설치 흐름 생략**: 저장소 클론(`git clone`)이나 `~/.local/bin`의 `PATH` 등록 안내 없이 바로 `make build` / `make install`부터 시작합니다.
- **Hook 설정 맥락 부족**: `.agents/hooks.json` 예시에 `"command": "guard"`만 적혀 있어, `guard`가 stdin 파이프를 자동 감지해 `hook` 모드로 동작한다는 설명이 없으면 처음 보는 사용자가 이해하기 어렵습니다.

### 3. 시각 자료 및 데모 부재
- 핵심 기능인 **TUI 관제탑(`guard top`)**, **런처 브리핑 배너**, **실제 차단(`[POLICY VIOLATION]`) 시 터미널 출력**에 대한 스크린샷·GIF 또는 텍스트 예시가 전혀 없어 프로젝트의 매력이 바로 전달되지 않습니다.

### 4. 오픈소스 공개 기본 정보 및 링크 불일치
- **메타 정보 누락**: 라이선스(License), 이슈/기여 방법(Contributing), 문제 해결(Troubleshooting / FAQ) 섹션이 없습니다.
- **영문 문서 내 한국어 링크**: 영문 `README.md` 하단의 `Detailed Design Documents` 링크가 모두 한국어 내부 설계 문서([01_requirements.md](file:///mnt/data/mytask/kms/guard/doc/01_requirements.md)~[04_development_plan.md](file:///mnt/data/mytask/kms/guard/doc/04_development_plan.md))로 연결되어 있으며, 이후 추가된 문서(`05`~`08`)도 반영되지 않았습니다.

---

요약하면, **"기존 초안 번역"은 되었지만 "외부 사용자가 처음 보고 설치해서 써볼 수 있는 릴리즈용 README"로는 내용이 부족한 상태**입니다. 원하시면 어떤 항목을 포함할지 정리해서 DEV 세션에 보강 작업을 다시 요청하거나, 직접 개선 방향을 잡아드릴 수 있습니다.

---

## 🧑‍💻 User · `2026-11-22 10:41:49` {#step-45}

DEV 모드에 개선 요청서를 전달해주세요.

---

## 🤖 Antigravity · `2026-11-22 10:42:05` {#step-49}

DEV 세션으로 README 개선 요청서(`fix` 핸드오프)를 전달(큐 등록)했습니다.

- **Handoff ID**: `hnd-20261122-484ab8f6` (Parent: `hnd-20261122-d65609ea`)
- **유형**: `FIX` (`-> DEV`)
- **대상 파일 (`--targets`)**: [README.md](file:///mnt/data/mytask/kms/guard/README.md), [README.ko.md](file:///mnt/data/mytask/kms/guard/README.ko.md)
- **요청 핵심 내용**:
  1. 누락된 최신 기능(`guard handoff`, `reset`, `doc`, `all`, `ui`, `hook`) 및 `guard-policy.yaml` 탐색 우선순위 반영
  2. 외부 사용자 온보딩(사전 요구사항, 설치/`PATH` 설정, Hook stdin 자동 감지 설명) 보강
  3. 런처 배너, 정책 위반 차단 출력, TUI 관제탑 예시 추가
  4. License / Contributing / Troubleshooting 섹션 추가 및 설계 문서 링크(05~08 포함, 영문 README 내 한국어 표기) 정리

`guard top` 또는 `guard handoff approve hnd-20261122-484ab8f6`으로 승인하면 DEV 세션 인박스로 전달됩니다.

---

## 🧑‍💻 User · `2026-11-22 10:43:28` {#step-46}

당신 생각에 현재 애플리케이션의 장점과 단점은 뭐라고 생각하나요.

---

## 🤖 Antigravity · `2026-11-22 10:43:28` {#step-48}

현재 컨텍스트(문서 및 방금 직접 사용해 본 `guard status`, `guard handoff` 실행 경험)를 기준으로 본 장점과 단점입니다.

---

### 👍 장점

1. **프롬프트가 아닌 물리적 통제 (`PreToolUse` 훅)**
   - LLM에게 "테스트 파일 보지 마"라고 지시하는 것보다 `DEV / PLAN / TEST / ALL` 모드별로 파일 도구와 셸 우회(`cat`, `git show`, `python -c` 등)를 실행 직전에 차단하는 구조가 훨씬 확실합니다.
2. **Zero-Daemon 초경량 아키텍처**
   - 상주 데몬이나 소켓 관리 없이 `~/.guard/sessions/<id>.json` 파일 I/O + PID 생존 검사 GC로 동작하고, 훅 판정이 `<0.5ms`로 빠르기 때문에 개발 흐름에 체감 지연이 거의 없습니다.
3. **대화 중단 없는 실시간 관제 (`guard top`)**
   - 세션을 껐다 켤 필요 없이 TUI에서 단축키(`D/T/P/A`)로 모드를 즉시 바꿀 수 있어, 실무에서 "잠깐 풀어줬다가 다시 잠그는" 흐름에 잘 맞습니다.
4. **멀티 세션 협업용 핸드오프 프로토콜 (`guard handoff`)**
   - `type`, `ref`, `targets`, `parent` 구조와 사용자 전용 `approve` 경계가 명확해, PLAN ↔ DEV ↔ TEST 세션 간 역할을 분리해 협업하기 좋습니다.

---

### 👎 단점 및 개선 여지

1. **초기 셋업과 에이전트 연동 설정이 분산되어 있음**
   - `guard` 바이너리 설치, `.agents/hooks.json`, `guard-policy.yaml`, `.agents/rules/guard-workflow.md` 등이 모두 정확히 맞아떨어져야 합니다.
   - 방금 겪었듯 `guard-workflow.md`에 `trigger: always_on` 한 줄이 빠져 있어도 에이전트가 규칙을 모른 채 매번 `--help`를 탐색하는 문제가 발생합니다. (`guard init` 같은 원커맨드 셋업이 없다면 온보딩 장벽이 됩니다.)
2. **핸드오프 CLI의 회신 UX 및 경로 정규화 부족**
   - 방금 `guard handoff` 회신 시 발신자(`85428527`)와 현재 세션(`98b388bc`)을 사람이 직접 확인해야 했습니다. `guard handoff reply <parent_id>`처럼 발신자에게 자동 회신하는 단축 명령이 있으면 실수가 크게 줄어듭니다.
   - `guard status` 출력에서 같은 저장소가 `/home/yundream/mytask/kms/guard`와 `/mnt/data/mytask/kms/guard`로 다르게 표시되는 점(심볼릭 링크 미정규화)도 세션 식별 시 혼동을 줄 수 있습니다.
3. **문서([README.md](file:///home/yundream/mytask/kms/guard/README.md))와 실제 기능 간 격차**
   - 핵심 기능으로 발전한 `guard handoff` 프로토콜이 아직 메인 [README.md](file:///home/yundream/mytask/kms/guard/README.md)에 반영되어 있지 않습니다.
4. **셸 우회 차단의 구조적 한계**
   - 명령어 패턴 기반 차단은 일반적인 우회를 잘 막지만, OS 레벨 샌드박스(namespace/seccomp 등)가 아니므로 새로운 스크립트·도구 조합이 나오면 규칙을 계속 보강해야 합니다.

---

요약하면, **아키텍처(무상태·저지연·모드 격리·핸드오프 개념)는 매우 탄탄**하고, 현재 가장 아쉬운 부분은 **초기 설정 자동화(DX)와 핸드오프 회신 UX·문서 최신화** 쪽입니다.

---

## 🧑‍💻 User · `2026-11-22 10:45:38` {#step-49}

초기 설정 자동화와 관련된 설계를 @[doc] 에 추가해주세요.

---

## 🤖 Antigravity · `2026-11-22 10:47:31` {#step-59}

초기 설정 자동화 및 자가 진단 설계서를 [09_init_automation_design.md](file:///home/yundream/mytask/kms/guard/doc/09_init_automation_design.md)에 추가했습니다.

### 주요 포함 내용
- **원커맨드 초기화 (`guard init`)**: 멱등성 보장, `--preset`/`--dry-run`/`--force`/`--mode` 옵션
- **비파괴 JSON 병합**: `.agents/hooks.json`의 기존 타 도구 훅(`obsidian-chatlog` 등)을 보존하면서 `"guard"` 블록만 원자적으로 Upsert
- **스택 자동 감지 프리셋**: `go.mod`, `package.json`, `pyproject.toml` 등에 따른 `guard-policy.yaml` 자동 생성
- **Always-On 시맨틱 룰 자동 배치**: `trigger: always_on` frontmatter와 `guard handoff` CLI 문법이 포함된 `.agents/rules/guard-workflow.md` 생성
- **자가 진단 및 자동 복구 (`guard doctor [--fix]`)**: PATH, 훅 등록, 정책 유효성, `trigger: always_on` 누락, 세션 저장소, 심볼릭 링크 경로 일관성 검사(`DOC-001`~`DOC-006`)
- **패키지 배치 및 테스트 전략**: `cmd/guard/init.go`, `internal/hub/setup/` 구조 및 단위/E2E 테스트 계획

---

## 🧑‍💻 User · `2026-11-22 10:48:59` {#step-60}

핸드오프가 됐을 때, agy 가 자동으로 수신해서 핸드오프된 문서를 검토하도록 할수는 없나요 ? 지금은 사람이 프롬프팅 하고 있는데.

---

## 🤖 Antigravity · `2026-11-22 10:49:06` {#step-64}

결론부터 말씀드리면 **가능합니다.** 다만 Antigravity(`agy`)의 훅 구조상 **"완전히 멈춰 있는(Idle) 대화창을 외부에서 깨우는 것"**과 **"턴이 시작될 때 자동 주입하는 것"**을 구분해서 접근해야 합니다.

[hooks.md](file:///home/yundream/.gemini/antigravity-cli/builtin/skills/agy-customizations/docs/hooks.md) 스펙 기준으로 구현 가능한 4가지 방식입니다.

---

### 방식 1. `guard handoff watch` + Reactive Wakeup (열린 세션 **완전 자동 기상**, 추천 ⭐)
Antigravity는 **백그라운드 태스크가 종료되면 유휴 상태의 에이전트를 자동으로 깨워(Reactive Wakeup)** 다음 턴을 실행하는 메커니즘을 갖고 있습니다.

- **동작 흐름**:
  1. `guard`에 블로킹 대기 커맨드 `guard handoff watch`를 추가 (인박스에 `approved` 티켓이 들어올 때까지 대기하다가, 도착하면 Markdown Envelope을 stdout으로 출력하고 종료).
  2. 수신 대기 세션에서 `guard handoff watch`를 백그라운드 태스크로 걸어둠.
  3. `guard top`에서 사용자가 `Enter`로 승인 → `watch`가 인박스 내용을 출력하며 종료 → **`agy`가 자동으로 깨어나 별도 프롬프팅 없이 즉시 문서 검토·작업 시작**.
- **장점**: `tmux` 같은 외부 도구 의존성 없음, 사람이 프롬프트를 전혀 칠 필요 없음.
- **단점**: 작업 1건 완료 후 다시 대기하려면 `watch`를 재실행해야 함.

### 방식 2. `SessionStart` / `PreInvocation` 훅의 `injectSteps` (반자동, 가장 안정적)
`agy`의 `SessionStart`와 `PreInvocation` 훅은 `injectSteps`로 `systemMessage`나 `userMessage`를 대화에 직접 주입할 수 있습니다.

- **동작 흐름**:
  - `.agents/hooks.json`에 `PreInvocation` / `SessionStart` 훅(`guard hook`)을 등록.
  - 훅 실행 시 인박스에 미처리(`approved`, 미-`ack`) 핸드오프가 있으면 그 요약·참조 문서 경로를 `injectSteps`로 자동 주입.
- **장점**: 구현이 매우 단순하고 Zero-Daemon 철학에 완벽히 부합. 새 세션을 열자마자 바로 컨텍스트가 로드됨.
- **단점**: 이미 유휴(Idle) 상태인 대화창을 스스로 깨우지는 못함(사용자가 아무 입력이나 한 번 주어야 `PreInvocation`이 발동).

### 방식 3. `guard top` 승인 시 `agy -p` 비대화형 원샷 실행 (완전 무인)
- `guard top`에서 `Enter`로 승인하는 순간, 대상 세션이 없거나 자동 모드일 때 `guard`가 직접 `guard <mode> agy -p "<인박스 내용 검토 및 조치>"` 프로세스를 기동.
- **장점**: 터미널 창 전환이나 프롬프트 입력이 전혀 필요 없음.
- **단점**: 기존 대화 맥락을 이어받지 않고 새 단발성 세션으로 실행됨.

### 방식 4. `tmux` / `zellij` `send-keys` 연동
- `guard top`에서 `Enter` 승인 시, 대상 세션이 실행 중인 `tmux` pane에 `"도착한 핸드오프 확인해줘"` + `Enter`를 직접 전송.
- **장점**: 기존 대화 맥락을 유지하면서 유휴 세션을 즉시 깨울 수 있음.
- **단점**: `tmux`/`zellij` 환경에 종속됨.

---

### 추천 조합
- **기본값 (안정성)**: **방식 2 (`SessionStart` / `PreInvocation` `injectSteps`)** — 훅 한 줄 추가로 어떤 환경에서든 인박스 자동 주입.
- **완전 무프롬프트 수신 (고급)**: **방식 1 (`guard handoff watch`)** — `agy`의 Reactive Wakeup을 활용해 `tmux` 없이도 승인 즉시 에이전트를 깨워 문서 검토까지 자동 실행.

원하시면 이 내용도 `doc/` 설계 문서(예: [08_handoff_protocol.md](file:///home/yundream/mytask/kms/guard/doc/08_handoff_protocol.md) 보강 또는 별도 설계서)에 반영해 드릴 수 있습니다.

---

## 🧑‍💻 User · `2026-11-22 10:50:04` {#step-65}

승인

---

## 🤖 Antigravity · `2026-11-22 11:00:16` {#step-71}

[08_handoff_protocol.md](file:///home/yundream/mytask/kms/guard/doc/08_handoff_protocol.md)에 수신 자동화(무프롬프트 인박스 주입 및 유휴 세션 자동 기상) 설계를 반영했습니다.

### 변경 요약
- **4.3절** ([L160-L166](file:///home/yundream/mytask/kms/guard/doc/08_handoff_protocol.md#L160-L166)): `guard handoff watch` 서브커맨드 추가
- **6.3절 신설** ([L226-L273](file:///home/yundream/mytask/kms/guard/doc/08_handoff_protocol.md#L226-L273)):
  - **계층 1 (기본)**: `SessionStart` / `PreInvocation` 훅의 `injectSteps`로 미처리 `approved` 인박스 자동 주입 및 중복 주입 방지
  - **계층 2 (고급)**: `guard handoff watch` 백그라운드 태스크 + Antigravity **Reactive Wakeup** 기반 유휴 세션 무프롬프트 자동 기상 흐름(시퀀스 다이어그램 포함)
  - 4가지 수신 자동화 방식 비교표 및 설계 채택 근거 정리
- **7절** ([L277-L285](file:///home/yundream/mytask/kms/guard/doc/08_handoff_protocol.md#L277-L285)): `watch.go`, `hook` 어댑터(`SessionStart`/`PreInvocation`), `guard handoff watch` CLI 매핑 반영

---

## 🧑‍💻 User · `2026-11-22 11:04:08` {#step-72}

셸 우회 차단의 구조적 한계 에서 당신은 namespace 기반의 격리를 말하였습니다. 각 모드가 서로 다른 namespace 에서 작동하도록 하자는 이야기 인가요.

---

## 🤖 Antigravity · `2026-11-22 11:04:08` {#step-73}

정확히 말씀드리면 **"각 모드를 namespace로 바꾸자"는 제안이라기보다, 현재 패턴 매칭 방식의 한계를 설명하기 위한 대비 개념**으로 언급한 것입니다.

### 1. 만약 모드별로 Linux Mount Namespace를 쓴다면?
말씀하신 대로 각 모드에 서로 다른 파일시스템 뷰(Mount Namespace)를 주는 구조가 됩니다.
- **DEV 모드**: 테스트 파일/디렉토리를 마스킹(가림) → `python -c`, `base64`, 컴파일된 바이너리 등 어떤 우회로도 열람 불가
- **PLAN 모드**: 소스 디렉토리를 `read-only`로 마운트 → 어떤 명령으로도 수정 불가
- **TEST 모드**: 프로덕션 구현부 마스킹, `doc/`·테스트·계약만 노출

커널 레벨에서 차단되므로 **셸 명령어 문자열 파싱으로 놓칠 수 있는 우회를 원천 봉쇄**할 수 있다는 장점이 있습니다.

---

### 2. 하지만 현재 `guard` 아키텍처에 그대로 도입할 때의 트레이드오프
`guard`의 핵심 장점과 충돌하는 지점이 큽니다.

1. **`guard top`의 실시간 모드 전환(`D/T/P/A`)과 충돌**
   - 이미 실행 중인 `agy` 프로세스의 Mount Namespace를 외부에서 동적으로 바꾸기는 매우 어렵습니다. (매 `run_command` 호출마다 `bwrap`/`unshare`로 일회성 샌드박스를 감싸는 구조로만 가능)
2. **Cleanroom 요구사항(`go test` 허용)과의 모순**
   - `DEV` 모드에서는 에이전트가 `*_test.go`를 직접 읽는 것은 막되, `go test` 실행과 그 stdout 확인은 허용해야 합니다. 파일시스템 레벨에서 `*_test.go`를 숨겨버리면 `go test` 자체도 실행되지 않습니다.
3. **OS 및 권한 종속성**
   - Linux 전용(macOS/Windows 미지원)이며, 컨테이너나 보안 강화 환경에서는 Unprivileged User Namespace가 막혀 있는 경우가 많습니다.

---

### 요약
- **현재 `guard`의 목적(초경량 `<0.5ms`, 실시간 모드 전환, `go test` 허용, 크로스플랫폼)**에서는 지금의 `PreToolUse` 훅 기반 차단이 현실적으로 가장 균형 잡힌 선택입니다.
- Namespace(또는 Linux Landlock / `bwrap`)는 프로세스 전체를 모드별로 분리하기보다, 추후 **강력한 보안이 필요한 환경에서 `run_command` 1회 실행 단위로 선택적 샌드박싱**을 붙일 때 고려할 수 있는 확장 옵션으로 보시면 됩니다.

---

## 🧑‍💻 User · `2026-11-22 11:08:08` {#step-74}

다른 세션을 agy 를 임의 시간에 깨우지 못한다면 "swarm" 은 어떻게 구성을 하는 거죠 ? 인터넷 상에 ai swarm 구성했다는 내용들이 자주 보이는데요.

---

## 🤖 Antigravity · `2026-11-22 11:08:08` {#step-75}

핵심 차이는 **"런타임 내부 메시지 버스로 묶여 있느냐"** vs **"서로 남남인 독립 터미널 프로세스냐"**입니다.

---

### 1. Antigravity 네이티브 `swarm`의 동작 방식
Antigravity의 내장 `swarm`(및 `/teamwork`, `subagent`)은 외부 터미널 프로세스 여러 개를 따로 띄우는 게 아니라, **단일 런타임 안의 공유 메시지 보드(Shared Message Board)**로 연결되어 있습니다.

- 워커 에이전트는 일이 없을 때 `waiting`(유휴/비용 0) 상태로 보드에 대기합니다.
- 코디네이터나 다른 워커가 보드에 `@mention`으로 메시지를 올리면, **Antigravity 런타임의 메시징 시스템이 즉시 해당 워커를 Reactive Wakeup으로 깨워** 턴을 시작시킵니다.

즉, **`agy` 자체는 스웜 멤버를 임의 시점에 깨우는 기능을 이미 갖고 있지만, 그것은 런타임 내부 버스 안에서만 동작**합니다.

---

### 2. 인터넷에 보이는 외부 AI Swarm 구성 방식 3가지
런타임 내장 스웜 대신, 외부에서 독립된 CLI 세션 여러 개를 엮어 Swarm을 만든 사례들은 대부분 아래 3가지 중 하나를 씁니다.

| 패턴 | 동작 원리 | 특징 |
| :--- | :--- | :--- |
| **A. 프로세스 스폰형** (`agy -p`, `claude -p`) | 중앙 오케스트레이터(스크립트/데몬)가 작업이 생길 때마다 비대화형 CLI를 **새로 실행**하고 결과(stdout/파일)를 회수 | 구현이 가장 안정적. 단, 기존 대화 창을 "깨우는" 게 아니라 매번 새로 띄움 |
| **B. `tmux` / PTY 주입형** | 각 세션을 `tmux` pane에 띄워두고, 워처가 큐에 메시지가 오면 `tmux send-keys`로 **키보드 입력을 강제 타이핑** | 커뮤니티 멀티-터미널 스웜에서 가장 흔함. 기존 대화 맥락 유지 가능하나 `tmux` 종속적 |
| **C. 내장 런타임/데몬형** (Antigravity `swarm`, MCP/RPC 버스) | 런타임(또는 상주 데몬)이 이벤트 루프를 들고 있다가 메시지 도착 시 대기 중인 에이전트를 직접 호출 | 가장 깔끔하고 지연 없음. 단, 런타임 자체가 스웜을 지원해야 함 |

---

### 3. `guard`의 위치
`guard`는 **서로 다른 터미널에서 사용자가 각각 띄운 독립 `agy` 프로세스**들을 **Zero-Daemon**으로 중계하는 외부 도구이기 때문에, Antigravity 내부 메시지 버스에 직접 이벤트를 쏠 수 없습니다.

그래서 `guard`에서 독립 세션 간 자동 기상을 구현하려면:
- `guard handoff watch`를 각 세션의 백그라운드 태스크로 띄워 **Antigravity의 Reactive Wakeup을 간접 활용**하거나,
- 위 **패턴 A(`agy -p`)** / **패턴 B(`tmux send-keys`)** 중 하나를 선택하는 구조가 됩니다.

*(참고: 단일 프로젝트 내에서 여러 에이전트가 자율적으로 협업하는 스웜이 목적이라면 Antigravity의 `/teamwork` 명령어로 네이티브 스웜을 직접 활용하는 방법도 있습니다.)*

---

## 🧑‍💻 User · `2026-11-22 11:09:27` {#step-76}

swarm 과 가드레일은 서로 책임지는 영역이 다른 거 같은데요.

---

## 🤖 Antigravity · `2026-11-22 11:09:27` {#step-78}

정확한 통찰입니다. 두 개념은 **직교(Orthogonal)하는 서로 다른 계층**입니다.

| 구분 | **Swarm (오케스트레이션 계층)** | **Guard (정책·통제 계층)** |
| :--- | :--- | :--- |
| **핵심 질문** | *"누가, 언제, 어떤 일을 함께 처리할 것인가?"* | *"각 에이전트가 허용된 경계 안에서만 행동하는가?"* |
| **책임 영역** | 작업 분할, 워커 기상/대기, 메시지 교환, 결과 취합 | 모드별 접근 차단(`PreToolUse`), Scope Lock, 승인 게이트, 감사 로그 |
| **못하는 것** | 워커가 테스트 파일을 훔쳐보거나 허용 외 코드를 수정하는 것을 물리적으로 막지 못함 | 복잡한 에이전트 스케줄링·다중 세션 오케스트레이션 |

---

### 아키텍처적 시사점
- **Guard가 Swarm(오케스트레이터)의 영역까지 떠안기 시작하면** `guard`의 가장 큰 강점인 **초경량·Zero-Daemon·단일 책임**이 무너지고 무거워집니다.
- [08_handoff_protocol.md](file:///home/yundream/mytask/kms/guard/doc/08_handoff_protocol.md#L14-L19)에 명시된 *"파이프라인 엔진이 아닌 범용 핸드오프 프로토콜"* 원칙도 같은 맥락입니다.
- **가장 이상적인 역할 분리**:
  - 협업·자동 기상·병렬 처리는 **Swarm(또는 에이전트 런타임)**에 맡기고,
  - **Guard**는 단일 세션이든 멀티 세션이든 스웜 워커든 그 밑단에서 **규칙 위반을 물리적으로 차단하고 사람이 최종 승인할 수 있게 하는 통제 계층**에 집중하는 구조입니다.

---

## 🧑‍💻 User · `2026-11-22 11:10:23` {#step-79}

하지만 swarm 모드에 guard 가 들어가는 순간 "자율에서 HITL" 로 변환되는 거 아닌가요 ?

---

## 🤖 Antigravity · `2026-11-22 11:10:23` {#step-80}

정확히는 **`guard`의 어느 기능을 쓰느냐에 따라 다릅니다.** 두 가지를 분리해서 보면 명확해집니다.

### 1. 정책 가드레일 (`PreToolUse` 차단) → 자율성 유지 (**Bounded Autonomy**)
- `guard`가 테스트 파일 열람이나 허용 외 코드 수정을 차단(`deny`)할 때 **사람에게 묻지 않습니다.**
- `<0.5ms` 만에 차단 사유를 에이전트에게 돌려주고, Swarm 워커는 **사람 개입 없이** 허용된 울타리 안에서 스스로 다른 해결책을 찾아 계속 진행합니다.
- 즉, 가드레일은 자율성을 멈추는 것이 아니라 **"이탈할 수 없는 안전한 자율성"**을 제공합니다. (고속도로의 가드레일이 차를 세우지 않고 차선 이탈만 막는 것과 같습니다.)

### 2. 핸드오프 승인 게이트 (`guard handoff approve`) → **HITL 전환**
- 말씀하신 대로, 워커 간 작업 전달마다 사람이 `guard top`에서 `Enter`로 승인해야 한다면 그 순간 **자율 Swarm에서 HITL 워크플로우로 성격이 바뀝니다.**

---

### 실무적 결합 방식: "내부는 자율, 경계만 HITL"
둘을 함께 쓸 때는 모든 단계에 HITL을 거는 게 아니라 **위험도 기준으로 경계를 나눕니다.**

- **내부 루프 (자율 + Guardrail)**: Swarm 워커들이 `DEV`/`TEST` 모드 가드레일 안에서 **사람 개입 없이** 구현·테스트·수정을 반복
- **외부 경계 (HITL 게이트)**: 스펙 변경, Scope 확장, 최종 커밋·PR·배포 같은 **비가역적·고위험 전환 시점에만** 사람이 승인

요약하면, **가드레일(`PreToolUse`)은 Swarm의 자율성을 해치지 않고 오히려 자율 실행을 믿고 맡길 수 있게 해주는 기반**이고, **HITL 여부(`approve` 게이트)는 사용자가 어느 경계에 체크포인트를 둘지 선택하는 정책**입니다.

---

## 🧑‍💻 User · `2026-11-22 11:11:53` {#step-81}

애매모호하게 자동화 하는 것 보다 HITL을 베이스로 환경을 만드는게 명확히 보이는데, 시장 및 기술 전문가들은 어떻게 평가하고 있나요.

---

## 🤖 Antigravity · `2026-11-22 11:11:53` {#step-82}

실무 엔지니어링과 AI 아키텍처 진영의 지배적인 합의도 사용자님 생각과 거의 일치합니다. **"애매한 완전 자율보다 명확한 HITL(더 정확히는 Human-on-the-Loop) 기반이 프로덕션에서 훨씬 강하다"**는 쪽으로 수렴하고 있습니다.

---

### 1. 기술·업계 전문가들이 HITL 베이스를 지지하는 핵심 이유

1. **오류 증폭(Error Compounding) 차단**
   - 각 에이전트 단계의 정확도가 90%여도 5단계를 무검증으로 자동 연쇄하면 최종 성공률은 약 59%($0.9^5$)까지 떨어집니다.
   - 앞 단계(기획/설계)의 작은 오해가 뒷 단계(구현/테스트)로 그대로 전파되면, 나중에 수천 줄의 diff를 리뷰하고 되돌리는 비용이 처음부터 사람이 개입하는 것보다 훨씬 커집니다.
   - HITL 승인 게이트는 이 오류 전파를 중간에 끊는 **회로 차단기(Circuit Breaker)** 역할을 합니다.
2. **Anthropic 등 주요 AI 연구·엔지니어링 가이드의 권고**
   - Anthropic의 에이전트 설계 가이드(*Building Effective Agents*) 역시, 복잡한 완전 자율 멀티에이전트보다 **단순하고 예측 가능하며 명확한 체크포인트(HITL)와 가드레일을 갖춘 워크플로우**를 프로덕션 기본값으로 권장합니다.
3. **보상 해킹(테스트 치팅)과 조용한 오작동(Silent Failure) 방지**
   - 코딩 에이전트 실무에서 가장 많이 보고되는 문제는 에이전트가 테스트를 통과시키기 위해 테스트 자체를 약화시키거나 정답을 하드코딩하는 현상입니다. 이를 막으려면 **역할 격리(가드레일) + 단계 전환 시 사람의 검증**이 필수라는 평가가 많습니다.

---

### 2. 시장의 실제 양분화: 어디에 무엇을 쓰는가?

전문가들은 "자율 vs HITL"을 이분법으로 보지 않고 **실패 비용과 검증 가능성**으로 나눕니다.

| 작업 특성 | 적합한 모델 | 예시 |
| :--- | :--- | :--- |
| **실패 비용이 낮고, 기계적 검증이 명확함** | **자율 Swarm / 병렬 실행** | 대규모 코드베이스 탐색, 독립된 수십 개 파일의 기계적 마이그레이션, 린트/타입 오류 일괄 수정 |
| **실패 비용이 높고, 의도(Intent) 정렬이 중요함** | **HITL + 물리적 가드레일** | 아키텍처 설계, 핵심 비즈니스 로직 구현, 스펙 변경, 보안·인프라 작업, 최종 커밋/PR |

---

### 3. 최근 트렌드: "피곤한 HITL"에서 "Human-on-the-Loop (HOTL)"로
다만 전문가들이 경계하는 HITL의 함정은 **승인 피로(Approval Fatigue)**입니다. 매 파일 읽기·매 명령 실행마다 `y/n`을 누르게 하면 사용자가 결국 무조건 `y`를 누르게 되어 통제가 무력화됩니다.

그래서 현재 가장 높게 평가받는 형태는:
- **세부 실행은 가드레일 안에서 자율적으로 돌리고** (`guard`의 `PreToolUse` 차단),
- **사람은 관제탑에서 흐름을 조망하다가 핵심 경계(단계 전환·핸드오프·예외)에서만 짧은 요약으로 승인하는 `Human-on-the-Loop`** 모델입니다.

지금 설계하고 계신 `guard` (`PreToolUse` 자동 차단 + `guard top` 3줄 요약 모달 + `Enter` 1회 승인 게이트)는 정확히 이 **Human-on-the-Loop 표준 방향**과 맞닿아 있습니다.

---

## 🧑‍💻 User · `2026-11-22 14:43:49` {#step-0}

Https://agentlaboratory.github.io/ 에 대해서 검토해주세요.

---

## 🤖 Antigravity · `2026-11-22 14:44:20` {#step-7}

**Agent Laboratory**([agentlaboratory.github.io](https://agentlaboratory.github.io/))는 Samuel Schmidgall(Johns Hopkins Univ., AMD 등) 연구팀이 발표한 논문 *"Agent Laboratory: Using LLM Agents as Research Assistants"* ([arXiv:2501.04227](https://arxiv.org/abs/2501.04227))의 공식 프로젝트 페이지이자 오픈소스 멀티 에이전트 연구 프레임워크([SamuelSchmidgall/AgentLaboratory](https://github.com/SamuelSchmidgall/AgentLaboratory))입니다.

---

### 1. 핵심 구조 및 워크플로우

인간 연구자를 완전히 대체하기보다 **"연구 보조 코파일럿(Human-in-the-Loop)"**으로 작동하도록 설계되었으며, 연구실 역할을 모방한 멀티 에이전트(PhD, Postdoc, ML Engineer, Professor 등)가 3단계 파이프라인을 수행합니다.

1. **문헌 조사 (Literature Review)**
   - arXiv, Hugging Face 등 외부 저장소에서 관련 논문을 검색·요약하고 연구 배경을 정리합니다.
2. **실험 수행 (Experimentation — `mle-solver`)**
   - 연구 계획을 수립하고 데이터를 준비한 뒤, **`mle-solver`** 모듈이 ML 코드를 작성·실행·디버깅하는 루프를 반복합니다.
   - 보상/점수 모델을 적용해 성능이 높은 코드 구성을 유지·개선합니다.
3. **보고서 작성 (Report Writing — `paper-solver`)**
   - **`paper-solver`** 모듈이 문헌·코드·실험 결과를 취합해 LaTeX/PDF 형식의 연구 보고서 초안을 생성합니다.

---

### 2. 주요 강점 및 논문 평가 결과

* **비용 및 속도 효율성**: 기존 자율 연구 프레임워크 대비 연구 비용을 약 **84% 절감**했다고 보고합니다. (논문 기준 워크플로우 1회 평균)
  * `GPT-4o`: 약 **$2.33** / 약 19.4분 (최고 가성비)
  * `o1-mini`: 약 **$7.51** / 약 1시간
  * `o1-preview`: 약 **$13.10** / 약 1.7시간 (최고 품질)
* **실행력 (MLE-Bench)**: Kaggle 과제 기반 MLE-Bench 서브셋 평가에서 `mle-solver`가 기존 자동화 프레임워크를 상회하며 다수의 메달 수준 성능과 인간 중앙값 이상의 결과를 기록했습니다.
* **인간 개입(Co-Pilot) 효과**: 완전 자율 모드보다 각 단계 체크포인트에서 인간 연구자가 피드백을 제공하는 코파일럿 모드에서 최종 결과물의 품질이 크게 향상되었습니다.

---

### 3. 한계점 및 검토 시 주의사항

1. **보안 및 코드 실행 리스크**
   - 에이전트가 자율적으로 Python 스크립트 및 명령어를 생성·실행하므로, 호스트 환경에서 직접 실행할 경우 파일 손상이나 리소스 고갈 위험이 있습니다.
   - arXiv·데이터셋 등 외부 문서를 동적으로 수집하므로 **간접 프롬프트 인젝션(Indirect Prompt Injection)** 을 통한 API 키 유출이나 악성 코드 실행 가능성에 유의해야 합니다. **반드시 컨테이너/샌드박스 격리 환경**에서 실행해야 합니다.
2. **과학적 엄밀성 및 자기 평가 편향**
   - 에이전트가 과학적 타당성보다 "오류 없이 실행되는 코드"나 "그럴듯한 LaTeX 문서"를 만드는 데 최적화되는 경향이 있습니다.
   - LLM 기반 자동 리뷰어(예: simulated NeurIPS reviewer)는 표면적 패턴 매칭에 의존해 품질을 과대평가하는 편향이 있으며, 인용 환각이나 통계 해석 오류를 걸러내기 어렵습니다.
3. **출력 구조 및 시각화 제약**
   - `paper-solver`가 정해진 LaTeX 템플릿에 강하게 의존해 유연한 논문 구성이 어렵고, 생성 가능한 그래프/그림(Figure) 수에도 제약(예: 2개 내외)이 있습니다.
4. **세부 의도 정렬(Alignment) 마찰**
   - 코파일럿 모드에서도 연구자의 정밀한 가설이나 실험 뉘앙스를 에이전트에게 정확히 반영시키는 데 시행착오가 발생할 수 있습니다.

---

### 4. 종합 의견

* **적합한 용도**: 본격적인 학술 논문 자동 생성보다는 **초기 아이디어 타당성 검증(PoC), 관련 문헌 빠른 스캔, ML 베이스라인 코드 구축 및 반복 실험 자동화** 용도로 실용성이 높습니다.
* **도입 시 권장 사항**:
  1. Docker 등 격리된 샌드박스 환경과 API 비용 상한선을 먼저 설정할 것
  2. 완전 자율 모드 대신 단계별 인간 검토(Co-Pilot) 모드로 운영할 것
  3. 최종 생성된 코드와 실험 수치는 에이전트 보고서만 신뢰하지 말고 독립적으로 재실행해 검증할 것

---

## 🧑‍💻 User · `2026-11-22 14:45:10` {#step-8}

@docs 디렉토리에 research 만들어서 위의 내용 저장해주세요.

---

## 🤖 Antigravity · `2026-11-22 14:45:25` {#step-11}

`docs/research` 디렉토리를 생성하고 검토 내용을 저장했습니다.

- 저장 파일: [agent-laboratory-review.md](file:///home/yundream/mytask/kms/guard/docs/research/agent-laboratory-review.md)

---

## 🧑‍💻 User · `2026-11-22 14:45:56` {#step-12}

docs 디렉토리가 아니고 doc 디렉토리에 저장해주세요. docs 디렉토리는 삭제해 주세요.

---

## 🤖 Antigravity · `2026-11-22 14:46:21` {#step-16}

`doc/research`로 파일을 이동하고 `docs` 디렉토리를 삭제했습니다.

- 저장 파일: [agent-laboratory-review.md](file:///home/yundream/mytask/kms/guard/doc/research/agent-laboratory-review.md)

---

## 🧑‍💻 User · `2026-11-22 14:49:52` {#step-17}

@[doc/09_init_automation_design.md] 작업을 진행하겠습니다. 이 작업은 사용자의 경험을 개선하기 위한 목적입니다. 그런데 기능적인 측면만 기술되어 있어서 사용자 입장에서 어떻게 좋아질지를 직관적으로 이해하기가 어렵습니다. 문서 앞부분에 사용자 경험 개선에 어떤 효과가 있을지 구체적으로 기술해주세요.

---

## 🤖 Antigravity · `2026-11-22 14:50:47` {#step-23}

[09_init_automation_design.md](file:///home/yundream/mytask/kms/guard/doc/09_init_automation_design.md#L8-L35) 앞부분(`## 1. 개요 및 문제 정의` 직전)에 **`## 사용자 경험(UX) 개선 효과`** 섹션을 추가했습니다.

### 추가된 주요 내용
- **Before / After 한눈에 보기 표**: 새 프로젝트 도입, 기존 훅 유지, 에이전트 첫 턴 반응성/토큰 효율, 언어별 정책 작성, 동작 이상 원인 파악, 반복 실행 안정성 등 6개 항목의 체감 변화를 직관적으로 대비
- **구체적인 사용자 시나리오별 체감 효과**:
  1. 설치 후 1분 내 즉시 시작(온보딩 장벽 제거)
  2. `trigger: always_on` 자동 주입을 통한 에이전트 불필요 탐색 제거(체감 속도·토큰 효율 개선)
  3. 비파괴 병합·`--dry-run`·`.bak` 백업으로 인한 심리적 부담 해소
  4. `guard doctor --fix`를 통한 환경 문제 즉시 진단 및 복구(디버깅 스트레스 해소)

---

## 🧑‍💻 User · `2026-11-22 14:54:40` {#step-24}

개발 계획을 수립하겠습니다. 여기에서 개발은 비즈니스 로직에 직접적인 변화는 없다고 봐야 하나요 ?

---

## 🤖 Antigravity · `2026-11-22 14:54:40` {#step-25}

네, **핵심 비즈니스 로직(가드레일 평가·차단 엔진, 정책 매칭, 핸드오프 프로토콜)에는 직접적인 변화가 없는 비침습적(Additive) 개발**로 보셔도 됩니다. 다만, 설계서 상에 **예외 항목 1건**이 포함되어 있어 구분해서 계획을 세우시면 좋습니다.

### 1. 기존 비즈니스 로직에 영향이 없는 영역 (Phase 1 ~ 2)
[09_init_automation_design.md](file:///home/yundream/mytask/kms/guard/doc/09_init_automation_design.md#L198-L216)의 패키지 구조와 의존성 방향 기준:
- **신규 모듈 독립 추가**: `internal/hub/setup/` 패키지(`detector`, `hooks_merge`, `templates`, `initializer`, `doctor`)와 `cmd/guard/init.go`를 새로 추가합니다.
- **기존 코어 로직은 읽기/호출만 수행**: `policy.LoadDetailed()`, `session` 저장소 등 기존 `internal/hub/*`, `internal/core` 모듈을 수정하지 않고 소비(호출)만 합니다.
- **기존 파일 변경은 최소한의 연결 작업뿐**:
  - `cmd/guard/main.go`: `init`, `doctor` 서브커맨드 라우팅 추가
  - `internal/i18n`: `cli.init.*`, `cli.doctor.*` 다국어 메시지 키 추가

### 2. 예외: 기존 공통 로직에 직접 영향을 주는 1건 (Phase 3)
- **`findWorkspaceRoot()`의 `filepath.EvalSymlinks()` 경로 정규화** ([09_init_automation_design.md](file:///home/yundream/mytask/kms/guard/doc/09_init_automation_design.md#L194) / Phase 3)
  - 이 항목은 `guard status`와 핸드오프 라우팅 등 **기존 워크스페이스 경로 판별 로직 전반의 동작**에 영향을 줍니다.
  - 심볼릭 링크 환경에서 경로 불일치 버그를 해결하는 개선이지만, 기존 세션/라우팅 테스트에 회귀 영향이 생길 수 있으므로 개발 계획 수립 시 **Phase 1~2(`init`/`doctor` 신규 기능)와 명확히 분리**하여 별도 검증 단계를 두는 것이 안전합니다.

---

## 🧑‍💻 User · `2026-11-22 14:56:35` {#step-26}

@[doc/dev] 디렉토리 밑에 개발 계획 수립해주세요. 어떤 파일을 어떻게 수정할지에 대해서도 검토해주세요.

---

## 🤖 Antigravity · `2026-11-22 14:58:45` {#step-46}

개발 계획서를 [init_automation_dev_plan.md](file:///home/yundream/mytask/kms/guard/doc/dev/init_automation_dev_plan.md)에 작성했습니다.

### 코드베이스 검토 중 확인된 주요 수정 포인트
1. **신규 패키지 격리 (비즈니스 로직 무변경)**
   - 핵심 로직은 신규 `internal/hub/setup/`(`detector.go`, `hooks_merge.go`, `templates.go`, `initializer.go`, `doctor.go`)과 `cmd/guard/init.go`에 독립 구현합니다.
   - 기존 `internal/hub/policy.LoadDetailed()`, `internal/hub/session`은 수정 없이 호출(소비)만 합니다.
2. **기존 [bootstrap.go](file:///home/yundream/mytask/kms/guard/internal/adapters/launcher/bootstrap.go) 중복 제거 (중요 발견)**
   - 현재 `internal/adapters/launcher/bootstrap.go`에 이미 `EnsureWorkspaceHook`과 `EnsureWorkflowRule`이 존재하나, 각각 **원자적 쓰기/`PreToolUse` 상세 검사 누락** 및 **`trigger: always_on` frontmatter 누락** 상태입니다.
   - `internal/hub/setup`을 단일 진실 공급원(SSOT)으로 만들고 `bootstrap.go`는 기존 시그니처를 유지한 채 `setup` 패키지를 위임 호출하도록 정리하는 계획을 반영했습니다.
3. **기존 수정 파일 범위 최소화**
   - [main.go](file:///home/yundream/mytask/kms/guard/cmd/guard/main.go): `init`/`doctor` 서브커맨드 라우팅 추가 + Phase 3에서 `findWorkspaceRoot()` 심볼릭 링크 정규화 분리 적용
   - [ko.json](file:///home/yundream/mytask/kms/guard/internal/i18n/locales/ko.json) / [en.json](file:///home/yundream/mytask/kms/guard/internal/i18n/locales/en.json): `cli.help` 갱신 및 `cli.init.*`, `cli.doctor.*` 키 대칭 추가
   - `README.md` / `README.ko.md`: 빠른 시작 섹션에 `guard init` / `guard doctor` 반영

---

## 🧑‍💻 User · `2026-11-22 15:02:02` {#step-47}

Phase 3: 테스트는 당신의 역할이 아닙니다. 테스트는 전문 QA(TEST 세션)에 맡길 겁니다. QA가 테스트를 수행 할 수 있도록 @[doc/spec] 문서를 정리해주세요.

---

## 🤖 Antigravity · `2026-11-22 15:03:44` {#step-57}

QA(TEST 세션)가 프로덕션 구현을 열람하지 않고도 블랙박스·E2E 테스트를 작성·수행할 수 있도록 계약 문서를 작성하고, 개발 계획서의 역할 경계도 반영했습니다.

### 1. QA용 계약서 신규 작성
- [06_init_automation_contract.md](file:///home/yundream/mytask/kms/guard/doc/spec/06_init_automation_contract.md) (기존 `doc/spec/01~05` 번호 및 양식 준수)
  - **공개 상수·에러·DTO**: `Preset*`, `StepStatus`, `CheckSeverity`, `DOC-001`~`DOC-006`, `InitOptions/Result`, `DoctorOptions/Report`
  - **공개 함수·메서드 계약**: `DetectStack`, `MergeWorkspaceHook`, `RenderPolicyYAML`, `HasAlwaysOnFrontmatter`, `InjectAlwaysOnFrontmatter`, `RunInit`, `RunDoctor`
  - **진단 매트릭스(`DOC-001`~`DOC-006`)** 및 `--fix` 자동 복구 범위
  - **Launcher Bootstrap 호환성 계약** 및 **CLI 계약**(`guard init`, `guard doctor`, `--json`, 종료 코드)
  - **8가지 핵심 불변 계약(QA 검증 포인트)**: 비파괴 병합/깨진 JSON 보호, 멱등성, `--dry-run` 무변경, `--force` `.bak` 백업, 프리셋 유효성, `DOC-004` 비파괴 복구, 심볼릭 링크 정규화, i18n 키 대칭성

### 2. 개발 계획서 역할 분리 반영
- [init_automation_dev_plan.md](file:///home/yundream/mytask/kms/guard/doc/dev/init_automation_dev_plan.md): Phase 3에서 E2E 테스트 작성 태스크를 제거하고, QA(TEST 세션)로 `06_init_automation_contract.md` 기준 검증을 핸드오프하도록 정리했습니다.

---

## 🧑‍💻 User · `2026-11-22 15:04:43` {#step-58}

개발 시작하겠습니다. 개발 원칙 말해주세요.

---

## 🤖 Antigravity · `2026-11-22 15:04:43` {#step-59}

이번 `guard init` / `guard doctor` 개발에서 지켜야 할 핵심 원칙은 다음과 같습니다.

1. **비즈니스 로직 비침습 (Additive & 단방향 의존성)**
   - `internal/core`, `internal/hub/policy`, `internal/hub/handoff` 등 기존 핵심 엔진은 수정하지 않고 호출(소비)만 합니다.
   - 신규 로직은 `internal/hub/setup` 패키지로 격리하고, 의존성 방향(`cmd/guard/init.go` → `internal/hub/setup` → `policy` / `session` / `core`)을 엄격히 유지합니다.

2. **단일 진실 공급원 (SSOT — `bootstrap.go` 중복 제거)**
   - 훅 병합과 워크플로우 룰 생성 로직은 `internal/hub/setup`에만 구현합니다.
   - [bootstrap.go](file:///home/yundream/mytask/kms/guard/internal/adapters/launcher/bootstrap.go)의 `EnsureWorkspaceHook` / `EnsureWorkflowRule`은 기존 공개 시그니처를 100% 유지하면서 내부 구현만 `setup` 패키지로 위임합니다.

3. **비파괴 · 원자적 쓰기 · 멱등성 (Zero-Damage)**
   - **원본 보호**: `.agents/hooks.json`은 타 도구 키(`obsidian-chatlog` 등)를 100% 보존하고, 파싱 불가능한 깨진 JSON은 1바이트도 수정하지 않고 에러를 반환합니다.
   - **원자적 쓰기**: 파일 갱신 시 `.tmp` 기록 후 `os.Rename`으로 교체합니다.
   - **멱등성 및 안전장치**: 반복 실행 시 `SKIP`을 보장하며, `--dry-run`은 파일 시스템을 전혀 변경하지 않고, `--force`는 기존 파일을 `.bak`으로 백업합니다.

4. **계약 우선 (Contract-First) & 역할 경계 준수**
   - [06_init_automation_contract.md](file:///home/yundream/mytask/kms/guard/doc/spec/06_init_automation_contract.md)에 명시된 공개 상수·DTO·함수 시그니처·진단 코드(`DOC-001`~`DOC-006`)·종료 코드(`0`/`1`)를 그대로 구현합니다.
   - E2E·블랙박스 테스트는 전문 QA(TEST 세션)의 역할이므로 직접 작성하지 않고, 구현 완료 후 핸드오프로 검증을 위임합니다.

5. **Zero-Daemon · 오프라인 · i18n 대칭성**
   - 외부 템플릿 파일이나 네트워크 의존성 없이 단일 바이너리 내장 템플릿만으로 동작합니다.
   - 모든 CLI 메시지는 [ko.json](file:///home/yundream/mytask/kms/guard/internal/i18n/locales/ko.json)과 [en.json](file:///home/yundream/mytask/kms/guard/internal/i18n/locales/en.json)에 동일한 키와 포맷 동사로 1:1 대칭 등록합니다.

6. **공통 로직 변경 격리 (Phase 3 분리)**
   - 기존 워크스페이스 경로 판별에 영향을 주는 `findWorkspaceRoot()` 심볼릭 링크 정규화는 `init`/`doctor` 신규 구현(Phase 1~2)과 분리해 Phase 3에서 별도 처리합니다.

---

## 🧑‍💻 User · `2026-11-22 15:05:30` {#step-60}

표준 IT/소프트웨어 용어, 문장 사용해주세요. 번역투, 한자어 사용 금합니다. 개발원칙 다시 말해주세요.

---

## 🤖 Antigravity · `2026-11-22 15:05:30` {#step-61}

이번 `guard init` / `guard doctor` 개발 원칙입니다.

1. **Core 코드 건드리지 않기 & 한 방향 의존성 유지**
   - `internal/core`, `internal/hub/policy`, `internal/hub/handoff`는 수정하지 않고 기존 API만 호출합니다.
   - 새 코드는 `internal/hub/setup` 패키지에 모으고, 의존성 방향(`cmd/guard/init.go` → `internal/hub/setup` → `policy` / `session` / `core`)을 한 방향으로 유지합니다.

2. **SSOT (Single Source of Truth) — `bootstrap.go` 중복 코드 없애기**
   - hook merge와 workflow rule 생성 코드는 `internal/hub/setup` 한 곳에만 둡니다.
   - [bootstrap.go](file:///home/yundream/mytask/kms/guard/internal/adapters/launcher/bootstrap.go)의 `EnsureWorkspaceHook`, `EnsureWorkflowRule`은 기존 함수 시그니처를 그대로 두고, 내부에서 `setup` 패키지를 호출하도록 바꿉니다.

3. **안전한 파일 처리 (Safe Merge, Atomic Write, Idempotent)**
   - `.agents/hooks.json`을 merge할 때 다른 키(`obsidian-chatlog` 등)는 그대로 남깁니다. 깨진 JSON 파일은 전혀 건드리지 않고 바로 에러를 리턴합니다.
   - 파일을 쓸 때는 `.tmp` 파일에 먼저 쓴 뒤 `os.Rename`으로 바꾸는 Atomic Write를 씁니다.
   - `guard init`은 여러 번 돌려도 결과가 같도록(Idempotent) 이미 있는 항목은 `SKIP`합니다. `--dry-run`은 파일을 하나도 만들거나 바꾸지 않고, `--force`는 덮어쓰기 전에 `.bak` 백업을 남깁니다.

4. **Contract-First 구현 & QA 테스트 분리**
   - [06_init_automation_contract.md](file:///home/yundream/mytask/kms/guard/doc/spec/06_init_automation_contract.md)에 적힌 상수, DTO, 함수 시그니처, 체크 코드(`DOC-001`~`DOC-006`), Exit Code(`0`/`1`)에 맞춰 구현합니다.
   - E2E·Black-box 테스트는 QA(TEST 세션)가 맡습니다. 여기서는 프로덕션 코드 구현에 집중하고, 끝나면 `guard handoff`로 테스트를 넘깁니다.

5. **단일 바이너리 오프라인 동작 & i18n 키 맞추기**
   - 외부 템플릿 파일이나 네트워크 요청 없이 바이너리 안에 넣은 템플릿만으로 동작하게 만듭니다.
   - CLI 출력 문구는 [ko.json](file:///home/yundream/mytask/kms/guard/internal/i18n/locales/ko.json)과 [en.json](file:///home/yundream/mytask/kms/guard/internal/i18n/locales/en.json)에 똑같은 키와 포맷(`%s`, `%d` 등)으로 함께 넣습니다.

6. **공통 경로 로직 수정 분리 (Phase 3)**
   - `findWorkspaceRoot()`의 symlink 경로 처리는 기존 기능에도 영향을 줍니다. `init`/`doctor` 구현(Phase 1~2)과 섞지 않고 Phase 3에서 따로 작업합니다.

---

## 🧑‍💻 User · `2026-11-22 15:08:01` {#step-62}

* 테스트를 통과 시키기 위한 에러 무시, 폴백으로 실패 무시하기, 하드코등도 하면 안됩니다. fast-fail 원칙을 지켜주세요. 
* 개발 범위의 코드만 수정해야 합니다. 개발이 끝난 후에는 git diff 로 합의한 코드만 수정했는지 audiot 레포트 작성해야 합니다.

개발 시작해주세요.

---

## 🤖 Antigravity · `2026-11-22 15:11:42` {#step-87}

`guard init` / `guard doctor` 개발을 완료했고, 빌드·정적 분석·기존 테스트 검증 및 `git diff` Audit을 마쳤습니다.

---

### 1. 검증 결과 (`go build` / `go vet` / `go test`)
- `go build ./...`: 통과
- `go vet ./...`: 통과
- `go test ./...`: 기존 전체 패키지(`core`, `policy`, `session`, `i18n`, `hook`, `launcher`, `tui`, `test/e2e`) 통과 (회귀 없음)
- 테스트 코드는 합의대로 작성하지 않았으며, QA(TEST 세션) 검증 대상입니다.

---

### 2. Git Diff Audit 리포트

#### A. 이번 개발에서 생성한 코드 파일 (6개 — 합의 범위 100% 일치)
| 파일 | 역할 |
| :--- | :--- |
| [detector.go](file:///home/yundream/mytask/kms/guard/internal/hub/setup/detector.go) | 스택 마커 감지(`go.mod` → `package.json` → `pyproject.toml` → `requirements.txt`), Preset/Mode Fail-Fast 검증 |
| [hooks_merge.go](file:///home/yundream/mytask/kms/guard/internal/hub/setup/hooks_merge.go) | `.agents/hooks.json` Safe Merge, 깨진 JSON 원본 보호(`ErrInvalidHooksJSON`), Atomic Write(`.tmp` → `os.Rename`) |
| [templates.go](file:///home/yundream/mytask/kms/guard/internal/hub/setup/templates.go) | 4종 프리셋 `guard-policy.yaml`, `trigger: always_on` 포함 `guard-workflow.md` 템플릿 및 frontmatter 검사/삽입 |
| [initializer.go](file:///home/yundream/mytask/kms/guard/internal/hub/setup/initializer.go) | `RunInit` 오케스트레이터(Idempotent, `--dry-run` 무변경, `--force` `.bak` 백업) |
| [doctor.go](file:///home/yundream/mytask/kms/guard/internal/hub/setup/doctor.go) | `RunDoctor` (`DOC-001`~`DOC-006`), `--fix` 자동 복구, `ExitCode()` |
| [init.go](file:///home/yundream/mytask/kms/guard/cmd/guard/init.go) | `guard init` / `guard doctor` CLI 플래그 파싱, `--json` 출력 및 i18n 포맷팅 |

#### B. 이번 개발에서 수정한 기존 코드 파일 (4개 — 합의 범위 100% 일치)
| 파일 | Diff 요약 | 변경 내용 |
| :--- | :---: | :--- |
| [main.go](file:///home/yundream/mytask/kms/guard/cmd/guard/main.go) | `+17 / -3` | `init`, `doctor` 서브커맨드 라우팅 추가 + `findWorkspaceRoot()` symlink 경로 정규화 |
| [bootstrap.go](file:///home/yundream/mytask/kms/guard/internal/adapters/launcher/bootstrap.go) | `+10 / -74` | `EnsureWorkspaceHook`, `EnsureWorkflowRule` 시그니처 유지, 내부 구현을 `setup` 패키지로 통합(SSOT) |
| [ko.json](file:///home/yundream/mytask/kms/guard/internal/i18n/locales/ko.json) | `+17 / -1` | `cli.help` 갱신 및 `cli.init.*`, `cli.doctor.*` 키 추가 |
| [en.json](file:///home/yundream/mytask/kms/guard/internal/i18n/locales/en.json) | `+17 / -1` | `ko.json`과 1:1 키·포맷 동사 맞춤 |

#### C. 문서 파일 변경 (합의 범위)
- [README.md](file:///home/yundream/mytask/kms/guard/README.md), [README.ko.md](file:///home/yundream/mytask/kms/guard/README.ko.md): Quick Start에 `guard init` / `guard doctor` 추가
- [09_init_automation_design.md](file:///home/yundream/mytask/kms/guard/doc/09_init_automation_design.md), [init_automation_dev_plan.md](file:///home/yundream/mytask/kms/guard/doc/dev/init_automation_dev_plan.md), [06_init_automation_contract.md](file:///home/yundream/mytask/kms/guard/doc/spec/06_init_automation_contract.md)

#### D. 범위 외 영역 Audit 결과
- **Core / 기존 비즈니스 로직 (`internal/core`, `internal/hub/policy`, `internal/hub/session`, `internal/hub/handoff`, `internal/adapters/hook`, `internal/adapters/tui`)**: **변경 0건**
- **기존 워킹 트리에 이미 있던 무관한 파일** (이번 작업에서 수정하지 않음): `.obsidian/workspace.json`, `chat/guard.md`, `doc/08_handoff_protocol.md`, `.agents/rules/`, `test/handoff-user-test-guide.md`

---

### 3. 다음 단계
QA(TEST 세션) 검증을 진행하려면 아래 명령으로 핸드오프를 발행할 수 있습니다:
```bash
guard handoff send --to test --type verify --ref doc/spec/06_init_automation_contract.md -m "guard init / guard doctor 구현 완료 — 계약서 기준 블랙박스 및 E2E 테스트 요청"
```

---

## 🧑‍💻 User · `2026-11-22 15:13:55` {#step-88}

QA가 테스트를 수행 할 수 있도록 @doc/spec/ 문서를 정리해주세요. 그리고 @[doc/test] 에 QA가 테스트에 참고할 문서 작성해주세요. 가장 중요한 건 "어떤 기능이 있는지" 와 테스트를 위한 함수 시그니처, DTO, 인터페이스가 되겠죠.

---

## 🤖 Antigravity · `2026-11-22 15:15:39` {#step-103}

QA(TEST 세션)가 블랙박스·E2E 테스트를 바로 작성할 수 있도록 `doc/spec/` 계약서를 정리하고 `doc/test/`에 QA 테스트 가이드를 작성했습니다.

### 1. `doc/spec/` 계약서 정리
- [06_init_automation_contract.md](file:///home/yundream/mytask/kms/guard/doc/spec/06_init_automation_contract.md): 실제 구현과 100% 일치하도록 `DOC-005` 심각도 구분(`WARN`/`ERROR`) 및 `guard init`/`guard doctor` Fail-Fast CLI 계약 보완
- [03_adapters_cli_contract.md](file:///home/yundream/mytask/kms/guard/doc/spec/03_adapters_cli_contract.md): `EnsureWorkspaceHook`/`EnsureWorkflowRule`의 `setup` 패키지 통합(Atomic Write, `trigger: always_on`) 및 CLI 테이블에 `guard init`/`guard doctor` 항목 추가

### 2. `doc/test/` QA 테스트 참고 문서 신규 작성
- [init-automation-qa-test-brief.md](file:///home/yundream/mytask/kms/guard/doc/test/init-automation-qa-test-brief.md)
  - **구현된 기능 목록**: `guard init`, `guard doctor`, Launcher Bootstrap SSOT 통합, Symlink 경로 정규화, i18n
  - **테스트용 공개 API 레퍼런스**: `guard/internal/hub/setup` 및 `launcher`의 전체 상수, Sentinel Error(`ErrInvalidPreset`, `ErrInvalidMode`, `ErrInvalidHooksJSON`), DTO(`InitOptions`, `InitResult`, `DoctorOptions`, `DoctorReport` 등), 함수·메서드 시그니처
  - **CLI 인터페이스 & Exit Code**, **격리 테스트 환경 구성 예시**(`t.TempDir()`, `LookPath` DI mock), **6가지 수용 기준(Acceptance Criteria)** 정리

---

## 🧑‍💻 User · `2026-11-22 15:18:42` {#step-104}

TEST 모드에 테스트 요구 문서 takeoff 해주세요.

---

## 🤖 Antigravity · `2026-11-22 15:19:06` {#step-107}

TEST 모드로 검증 핸드오프 티켓을 발행했습니다.

- **티켓 ID**: `hnd-20261122-e34e80bc`
- **유형 / 대상**: `VERIFY` → `TEST`
- **첨부 참조 문서**:
  - [init-automation-qa-test-brief.md](file:///home/yundream/mytask/kms/guard/doc/test/init-automation-qa-test-brief.md)
  - [06_init_automation_contract.md](file:///home/yundream/mytask/kms/guard/doc/spec/06_init_automation_contract.md)
  - [03_adapters_cli_contract.md](file:///home/yundream/mytask/kms/guard/doc/spec/03_adapters_cli_contract.md)

`guard top` 또는 `guard handoff approve hnd-20261122-e34e80bc`로 승인하시면 TEST 세션 인박스로 전달됩니다.

---

## 🧑‍💻 User · `2026-11-22 15:19:39` {#step-0}

개발자로 부터 테스트 요청이 takeoff 됐습니다. 검토해주세요.

---

## 🤖 Antigravity · `2026-11-22 15:20:42` {#step-15}

개발자 세션(`DEV/0c6f8f4b`)으로부터 수신된 핸드오프 티켓과 참조 문서를 검토했습니다.

### 1. 수신 핸드오프 요약
- **티켓 ID**: `hnd-20261122-e34e80bc` (`VERIFY` | `DEV(0c6f8f4b) -> TEST` | 상태: `approved`)
- **요청 내용**: `guard init` / `guard doctor` 구현 완료에 따른 계약서 기준 블랙박스·E2E 테스트
- **참조 문서**:
  - [init-automation-qa-test-brief.md](file:///home/yundream/mytask/kms/guard/doc/test/init-automation-qa-test-brief.md)
  - [06_init_automation_contract.md](file:///home/yundream/mytask/kms/guard/doc/spec/06_init_automation_contract.md)
  - [03_adapters_cli_contract.md](file:///home/yundream/mytask/kms/guard/doc/spec/03_adapters_cli_contract.md)

---

### 2. 검증 대상 및 범위
| 계층 | 패키지 | 핵심 검증 대상 |
| :--- | :--- | :--- |
| **Setup Hub** | `internal/hub/setup` | `DetectStack`, `ValidatePreset`, `MergeWorkspaceHook`, `InspectWorkspaceHook`, `RenderPolicyYAML`, `WorkflowRuleContent`, `HasAlwaysOnFrontmatter`, `InjectAlwaysOnFrontmatter`, `EnsureWorkflowRule`, `RunInit`, `RunDoctor`, `DoctorReport.ExitCode` |
| **Launcher** | `internal/adapters/launcher` | `EnsureWorkspaceHook`, `EnsureWorkflowRule` 기존 시그니처 호환성 및 `trigger: always_on` 포함 여부 |
| **CLI / E2E** | `cmd/guard` | `guard init`, `guard doctor` (`--fix`, `--json`), `guard help`, symlink 경로 정규화 및 기존 `test/e2e/handoff_*` 회귀 여부 |
| **i18n** | `internal/i18n` | `ko.json` / `en.json`의 `cli.init.*`, `cli.doctor.*` 키 집합 및 포맷 동사 일치 여부 |

---

### 3. 핵심 검증 포인트 (계약 불변 조건)
1. **스택 감지 및 정책 유효성**: `go.mod` > `package.json` > `pyproject.toml` > `requirements.txt` > `generic` 우선순위 준수, 4종 프리셋 YAML 모두 `policy.LoadFile()`에서 오류·경고 없이 로드 및 1개 이상 규칙 보유.
2. **비파괴 JSON 병합 및 손상 보호**: `.agents/hooks.json`의 기존 타 최상위 키(`obsidian-chatlog` 등) 100% 보존. 깨진 JSON 입력 시 `init`과 `doctor --fix` 모두 원본을 1바이트도 수정하지 않고 에러 반환(`ErrInvalidHooksJSON`).
3. **Init 안전 옵션 및 멱등성**:
   - 2회 연속 실행 시 2회차 전 항목 `StatusSkipped` 및 파일 바이트 무변경.
   - `--dry-run` 시 파일·디렉토리(`.agents/` 포함) 미생성·무변경.
   - `--force` 시 기존 `guard-policy.yaml` / `guard-workflow.md`를 `.bak`으로 백업 후 재생성(`StatusBackedUp`).
   - `--no-hooks`, `--no-policy`, `--no-rules` 선택 스킵 및 잘못된 플래그/인자/프리셋/모드 입력 시 Fail-Fast(Exit `1`).
4. **Always-On 룰 및 `DOC-004` 비파괴 복구**: 신규 생성 파일에 `trigger: always_on` 및 `<!-- guard-managed: v1 -->` 포함. `trigger: always_on`만 누락된 사용자 파일에 `doctor --fix` 실행 시 기존 본문 100% 보존 + frontmatter 삽입 후 재진단 `OK`.
5. **Doctor 진단 매트릭스 (`DOC-001`~`DOC-006`) 및 CLI 출력**:
   - 각 코드별 `OK`/`WARN`/`ERROR`/`FIXED` 판정 및 `--fix` 자동 복구 범위(깨진 JSON/문법 오류 정책은 덮어쓰지 않고 `ERROR` 유지).
   - `ExitCode()`: `ErrorCount > 0`이면 `1`, `WARN`만 있으면 `0`.
   - `guard doctor --json`: stdout에 배너 없이 순수 유효 `DoctorReport` JSON만 출력.
6. **Symlink 정규화 및 i18n 대칭성**: symlink 경로 실행 시 canonical path 정규화 확인, 기존 핸드오프 E2E 회귀 없음, `ko.json`/`en.json` 키·포맷 동사 100% 대칭.

---

### 4. 검토 의견 및 다음 단계
- QA 테스트 브리프와 계약서 간 API 시그니처, 에러/상태 상수, 진단 매트릭스가 일관되게 정리되어 있으며, `DoctorOptions`에 `SessionMgr`·`LookPath` DI 포인트도 마련되어 있어 격리된 블랙박스 및 E2E 테스트 진행에 문제가 없습니다.
- 진행을 승인해 주시면 계약서 기준으로 블랙박스 테스트(`*_test.go`) 작성·실행 및 CLI E2E 검증을 수행한 뒤 결과를 보고하겠습니다.

---

## 🧑‍💻 User · `2026-11-22 15:22:20` {#step-16}

@[doc/test] 에 테스트시나리오 작성해주세요.

---

## 🤖 Antigravity · `2026-11-22 15:24:14` {#step-24}

[init-automation-test-scenarios.md](file:///home/yundream/mytask/kms/guard/doc/test/init-automation-test-scenarios.md)에 `guard init` / `guard doctor` QA 테스트 시나리오(총 52개 케이스)를 작성했습니다.

### 구성 요약
- **Stage 1 (`TC-INIT-001` ~ `TC-INIT-016`)**: `internal/hub/setup` 유틸리티 — 스택 감지 우선순위, 4종 프리셋 정책 YAML 파싱 유효성, `.agents/hooks.json` 비파괴 Safe Merge 및 깨진 JSON 보호, `HasAlwaysOnFrontmatter`/`InjectAlwaysOnFrontmatter` 오탐 방지 및 비파괴 주입
- **Stage 2 (`TC-INIT-017` ~ `TC-INIT-025`)**: `RunInit` 오케스트레이션 — 기본 초기화, 명시적 프리셋/모드 정규화, 멱등성(Idempotency), `--dry-run` 무변경, `--force` `.bak` 백업, `--no-*` 스킵, Fail-Fast 검증
- **Stage 3 (`TC-DOC-001` ~ `TC-DOC-012`)**: `RunDoctor` 진단 및 자동 복구 — `DOC-001`~`DOC-006` 심각도 판정, `--fix` 복구 가능/불가 경계(깨진 JSON·문법 오류 정책 덮어쓰기 금지, `DOC-004` 본문 보존, Stale 세션 정리), `ExitCode()` 경계
- **Stage 4 (`TC-LCH-001` ~ `TC-LCH-004`)**: `internal/adapters/launcher` — `EnsureWorkspaceHook`, `EnsureWorkflowRule` 시그니처 호환성 및 `trigger: always_on` 생성
- **Stage 5 (`TC-I18N-001` ~ `TC-I18N-002`)**: `internal/i18n` — `ko.json`/`en.json` 키 집합 및 포맷 동사 100% 대칭성
- **Stage 6 (`TC-E2E-001` ~ `TC-E2E-009`)**: `cmd/guard` CLI E2E — `guard init`/`guard doctor`(`--fix`, `--json`)/`guard help`, Fail-Fast 인자 검증, Symlink `WORKSPACE_ROOT` 정규화, 기존 `test/e2e/handoff_*` 회귀 확인
- **AC 추적성 매트릭스**: [init-automation-qa-test-brief.md](file:///home/yundream/mytask/kms/guard/doc/test/init-automation-qa-test-brief.md#L241-L250)의 6개 수용 기준과 전체 테스트 케이스 1:1 매핑

검토 후 시나리오대로 블랙박스·E2E 테스트 코드 작성 및 실행을 진행할까요?

---

## 🧑‍💻 User · `2026-11-22 18:27:19` {#step-25}

테스트 시나리오는 성공 케이스 외에, 다양한 예외/엣지 케이스를 포함하도록 작성했나요 ?

---

## 🤖 Antigravity · `2026-11-22 18:27:19` {#step-26}

네, [init-automation-test-scenarios.md](file:///home/yundream/mytask/kms/guard/doc/test/init-automation-test-scenarios.md)의 총 52개 시나리오 중 **예외(Negative) 11건, 경계(Edge) 7건(총 18건, 약 35%)**과 회귀 1건을 포함해 작성했습니다.

### 포함된 주요 예외·엣지 케이스
- **손상 파일 비파괴 보호**: 깨진 `hooks.json`(`{invalid`, 빈 파일, 최상위 배열 `[]`) 및 문법 오류/빈 규칙(`rules: []`) `guard-policy.yaml`에 대해 `init`, `doctor --fix`, `launcher.EnsureWorkspaceHook` 모두 원본을 1바이트도 수정하지 않고 에러 반환 (`TC-INIT-010`, `024`, `TC-DOC-004`, `006`, `TC-LCH-002`)
- **부분 구조/오탐 경계**:
  - `"guard"` 최상위 키는 있으나 `PreToolUse`에 `guard hook`만 누락된 경우 (`TC-INIT-008`)
  - 본문에만 `trigger: always_on`이 기재되었거나 닫는 `---`가 누락된 경우의 frontmatter 오탐 방지 (`TC-INIT-014`)
  - 다중 스택 마커 공존 시 우선순위 충돌 (`TC-INIT-002`)
- **입력 검증 및 Fail-Fast**: 잘못된 `--preset`/`--mode`, 알 수 없는 플래그, 예상치 못한 positional args 입력 시 파일 시스템 변경 0건 + Exit `1` (`TC-INIT-003`, `005`, `019`, `TC-E2E-004`, `007`)
- **환경·권한·Exit Code 경계**:
  - `PATH`에서 `guard` 미발견(`DOC-001`), 읽기 전용 세션 디렉토리(`DOC-005`), Symlink `WorkspaceRoot`(`DOC-006`) (`TC-DOC-002`, `010`, `011`)
  - `WARN`만 있을 때 Exit `0` vs `ERROR >= 1`일 때 Exit `1` 경계, 기존 `.agents/current_mode` 보존 (`TC-DOC-012`, `TC-INIT-025`)

### 추가로 보강할 수 있는 심화 엣지 케이스 (선택)
필요하시다면 아래 케이스도 시나리오에 추가할 수 있습니다:
1. **CRLF(`\r\n`) 개행 / BOM 포함 `guard-workflow.md`**에서의 `HasAlwaysOnFrontmatter` 및 `InjectAlwaysOnFrontmatter` 동작
2. **`hooks.json` 타입 불일치**: 유효한 JSON이지만 `"guard": "string"` 또는 `"PreToolUse": {}`처럼 예상과 다른 스키마 타입일 때의 처리
3. **마커 이름 충돌**: `go.mod`가 일반 파일이 아닌 디렉토리일 때 `DetectStack` 판별
4. **권한/경로 경계**: `WorkspaceRoot`가 빈 문자열이거나 존재하지 않는 경로일 때 `RunInit` / `RunDoctor`의 안전한 에러 처리

위 심화 케이스를 시나리오 문서에 추가할까요, 아니면 현재 시나리오로 바로 테스트 구현을 진행할까요?

---

## 🧑‍💻 User · `2026-11-22 18:36:29` {#step-27}

테스트 구현해주세요.

---

## 🧑‍💻 User · `2026-11-22 18:42:25` {#step-47}

실패가 발생하면 임의로 수정하지 말고 즉시 ㅂ고홰주세요.

---

## 🤖 Antigravity · `2026-11-22 18:42:38` {#step-51}

테스트를 구현하고 실행한 결과, **`guard/internal/hub/setup`의 `DOC-003` (`guard-policy.yaml` 진단 및 `--fix`)에서 실제 결함이 발견되어 실패**했습니다. 지시하신 대로 어떤 파일도 임의로 수정하지 않고 즉시 보고합니다.

---

### 1. 테스트 구현 및 실행 결과 요약
작성한 테스트 파일:
- [blackbox_init_doctor_test.go](file:///home/yundream/mytask/kms/guard/internal/hub/setup/blackbox_init_doctor_test.go)
- [init_bootstrap_compat_test.go](file:///home/yundream/mytask/kms/guard/internal/adapters/launcher/init_bootstrap_compat_test.go)
- [init_doctor_i18n_test.go](file:///home/yundream/mytask/kms/guard/internal/i18n/init_doctor_i18n_test.go)
- [init_doctor_cli_test.go](file:///home/yundream/mytask/kms/guard/test/e2e/init_doctor_cli_test.go)

| 패키지 | 결과 | 비고 |
| :--- | :---: | :--- |
| `guard/internal/adapters/launcher` | **PASS** | `TC-LCH-001`~`004` + 기존 테스트 전체 통과 |
| `guard/internal/i18n` | **PASS** | `TC-I18N-001`~`002` + 기존 테스트 전체 통과 |
| `guard/test/e2e` | **PASS** | `TC-E2E-001`~`008` + 기존 `handoff_*` 회귀 전체 통과 |
| `guard/internal/hub/setup` | **FAIL** | **3건 실패** (Stage 1~2 `RunInit` 전체 PASS, Stage 3 `DOC-003` 3건 FAIL) |

---

### 2. 실패 상세 내역 (모두 `DOC-003` 관련)

1. **`TestSetup_RunDoctor_HealthyWorkspace` (`TC-DOC-001`)**
   - `RunInit(WorkspaceRoot: ws)`로 `<ws>/guard-policy.yaml`을 생성한 직후 `RunDoctor(WorkspaceRoot: ws)` 실행
   - **기대**: `DOC-003 == OK`
   - **실제**: `DOC-003 == WARN` (`no guard-policy.yaml found; using builtin fallback (run 'guard init' or 'guard doctor --fix')`)

2. **`TestSetup_RunDoctor_DOC003_Policy/missing_policy_builtin_warn_and_autofix` (`TC-DOC-005`)**
   - `RunDoctor(WorkspaceRoot: ws, Fix: true)`로 `<ws>/guard-policy.yaml`을 생성(`FIXED`)한 뒤 재진단
   - **기대**: 재진단 시 `DOC-003 == OK`
   - **실제**: 재진단 시에도 `OK`가 되지 않고 계속 `builtin fallback`으로 판정

3. **`TestSetup_RunDoctor_DOC003_Policy/broken_existing_policy_not_overwritten_on_fix` (`TC-DOC-006`, 치명적 계약 위반)**
   - `<ws>/guard-policy.yaml`에 문법이 깨진 YAML을 배치한 뒤 `RunDoctor(WorkspaceRoot: ws, Fix: true)` 실행
   - **계약** ([06_init_automation_contract.md §2.3](file:///home/yundream/mytask/kms/guard/doc/spec/06_init_automation_contract.md#L140)): 기존 정책 파일의 문법 오류는 덮어쓰지 않고 `ERROR` 유지 (`Fixed: false`, `ExitCode: 1`)
   - **실제**: `{Code:DOC-003 Severity:FIXED Message:created guard-policy.yaml (preset: go) Fixable:true Fixed:true}` 반환 (`ExitCode: 0`) 및 **기존 깨진 `guard-policy.yaml` 파일을 덮어씀**

---

### 3. 원인 분석
- `RunInit`과 `DOC-003 --fix`는 정책 파일을 `<WorkspaceRoot>/guard-policy.yaml`에 생성하지만, `RunDoctor`의 `DOC-003`가 호출하는 `policy.LoadDetailed(workspaceRoot)`가 **`CWD != WorkspaceRoot` 환경에서 `<WorkspaceRoot>/guard-policy.yaml`을 탐색하지 못하는 것**으로 보입니다 (`CWD/guard-policy.yaml` 및 `<WorkspaceRoot>/.agents/guard-policy.yaml`만 조회).
- 그 결과 `CWD == WorkspaceRoot`인 CLI E2E 테스트는 우연히 통과했지만, `WORKSPACE_ROOT`를 별도 경로로 지정해 실행하거나 패키지 단위 테스트(`CWD != WorkspaceRoot`)로 호출하면:
  1. 정상 존재하는 `<WorkspaceRoot>/guard-policy.yaml`을 찾지 못해 `WARN` 오탐 발생
  2. 문법이 깨진 `<WorkspaceRoot>/guard-policy.yaml`도 "미존재(`builtin`)"로 오판해 **`doctor --fix`가 기존 정책 파일을 덮어쓰는 데이터 손실** 발생

---

### 다음 조치 확인
이 결함에 대해 개발자 세션(`DEV/0c6f8f4b`)으로 `guard handoff send --to dev --type fix --parent hnd-20261122-e34e80bc ...` 수정 요청 티켓을 발행할까요?

---

## 🧑‍💻 User · `2026-11-22 18:43:28` {#step-52}

승인

---

## 🤖 Antigravity · `2026-11-22 18:44:27` {#step-55}

개발자 세션(`DEV`)으로 수정 요청 핸드오프를 발행하고 기존 검증 티켓을 완료 처리했습니다.

- **발행 티켓**: `hnd-20261122-2068c700` (`FIX` | `TEST -> DEV` | Parent: `hnd-20261122-e34e80bc`)
  - **대상 범위**: `internal/hub/setup/**`, `internal/hub/policy/**`
  - **참조 문서**: [06_init_automation_contract.md](file:///home/yundream/mytask/kms/guard/doc/spec/06_init_automation_contract.md), [init-automation-test-scenarios.md](file:///home/yundream/mytask/kms/guard/doc/test/init-automation-test-scenarios.md)
  - **내용**: `DOC-003` 결함 3건 (`CWD != WorkspaceRoot` 환경에서 `<WorkspaceRoot>/guard-policy.yaml` 미탐지 및 `--fix` 시 깨진 정책 파일 덮어쓰기) 및 재현 명령(`go test -v ./internal/hub/setup/...`) 전달
- **완료 처리 (`ack`)**: `hnd-20261122-e34e80bc`

> 티켓은 `queued` 상태로 등록되었으므로 사용자 승인(`guard handoff approve hnd-20261122-2068c700` 또는 TUI) 후 `DEV` 세션으로 전달됩니다.

---

## 🧑‍💻 User · `2026-11-22 18:58:19` {#step-108}

QA(테스트모드)로 부터 테스트 결과가 (handoff)전달됐습니다. 내용 분석해주세요.

---

## 🤖 Antigravity · `2026-11-22 18:58:44` {#step-114}

QA(TEST 세션)로부터 수신된 핸드오프 티켓(`hnd-20261122-2068c700`, `FIX` → `DEV`) 분석 결과입니다.

---

### 1. 테스트 결과 요약
- **통과 영역**: `internal/adapters/launcher`, `internal/i18n`, `test/e2e`(신규 + 기존 `handoff_*` 회귀), `internal/hub/setup`의 나머지 테스트 **전체 PASS**
- **실패 영역**: `internal/hub/setup`의 `DOC-003` 관련 **3건 실패** (동일한 근본 원인 1개에서 파생)
  1. `TestSetup_RunDoctor_HealthyWorkspace` (`TC-DOC-001`): `RunInit`으로 `<ws>/guard-policy.yaml` 생성 후 `RunDoctor` 호출 시 `DOC-003`이 `OK`가 아닌 `WARN` (`builtin` fallback) 반환
  2. `TestSetup_RunDoctor_DOC003_Policy/missing_policy_builtin_warn_and_autofix` (`TC-DOC-005`): `Fix: true`로 `<ws>/guard-policy.yaml` 생성 후 재진단해도 `DOC-003`이 `OK`가 되지 않음
  3. `TestSetup_RunDoctor_DOC003_Policy/broken_existing_policy_not_overwritten_on_fix` (`TC-DOC-006`): `<ws>/guard-policy.yaml`에 문법 오류 YAML이 존재하는데도 `RunDoctor(Fix: true)`가 `builtin`으로 오판해 기존 깨진 파일을 덮어씀 (계약 위반)

---

### 2. 근본 원인 (코드 확인 결과)
`internal/hub/setup/doctor.go`의 `checkPolicyYAML`이 `policy.LoadDetailed(workspaceRoot)`를 호출하는데, [loader.go](file:///home/yundream/mytask/kms/guard/internal/hub/policy/loader.go#L48-L57) 및 [candidatePaths](file:///home/yundream/mytask/kms/guard/internal/hub/policy/loader.go#L131-L166)의 실제 탐색 경로는 다음과 같습니다:

1. `$GUARD_POLICY_PATH`
2. `<현재 프로세스 CWD>/guard-policy.yaml` (`os.Getwd()`)
3. `<WorkspaceRoot>/.agents/guard-policy.yaml`
4. `~/.config/guard/guard-policy.yaml`

즉, **`CWD != WorkspaceRoot` 환경**(하위 디렉토리에서 실행, `WORKSPACE_ROOT` 지정, 또는 `t.TempDir()` 테스트)에서는 `<WorkspaceRoot>/guard-policy.yaml`이 탐색 대상에 아예 포함되지 않습니다.

이로 인해:
- `RunInit` / `doctor --fix`가 생성한 `<WorkspaceRoot>/guard-policy.yaml`을 `LoadDetailed`가 읽지 못해 항상 `builtin` fallback(`WARN`)으로 판정됩니다.
- `<WorkspaceRoot>/guard-policy.yaml`에 문법 오류 YAML이 있어도 읽지 못하므로 `Warnings`가 비어 있고 `Source == builtin`으로 오판되어, `--fix`가 파일이 없는 줄 알고 기존 깨진 파일을 덮어쓰게 됩니다.

---

### 3. 수정 방안

**권장안 (`internal/hub/setup/doctor.go`만 수정 — 기존 `policy` 로더 무변경 원칙 유지)**
- `checkPolicyYAML`에서 `policy.LoadDetailed(workspaceRoot)` 대신 `policy.LoadDetailedWithOptions(...)`를 호출하며 `CWD`에 `workspaceRoot`를 명시 전달합니다.
  - 이렇게 하면 `<workspaceRoot>/guard-policy.yaml`과 `<workspaceRoot>/.agents/guard-policy.yaml`을 모두 정확히 검사하고, 깨진 YAML 파싱 시 `Warnings`가 채워져 `ERROR`로 Fail-Fast 처리됩니다.
- 또한 `Fix: true`로 새 `guard-policy.yaml`을 쓰기 전에 `<workspaceRoot>/guard-policy.yaml` 파일 존재 여부를 방어적으로 확인해 기존 파일 덮어쓰기를 원천 차단합니다.

수정 진행을 지시해 주시면 `doctor.go` 수정 후 `go test -v ./internal/hub/setup/...`로 재현 검증, `guard handoff ack hnd-20261122-2068c700` 처리 및 QA 재검증 핸드오프까지 진행하겠습니다.

---

## 🧑‍💻 User · `2026-11-22 19:02:38` {#step-115}

당신이 하는 말을 이해하기가 어렵습니다. 테스트 레포트에 대한 분석에는 체계적으로 답해야 하지 않나요 ?
핵심적인 것은 "어떤 일을 하는 코드에서", "왜 발생 했는가", "사용자 혹은 시스템에 미치는 영향과 임펙트의 범위"
그외에 수정을 할 경우 다른 컴포넌트에 미치는 영향 등을 설명해줘야죠. 
당신의 말은 그냥 이 코드에 이게 문제입다라는 정보만 담고 있습니다.

---

## 🤖 Antigravity · `2026-11-22 19:02:38` {#step-116}

피드백 감사합니다. QA 테스트 리포트(`hnd-20261122-2068c700`)에 대한 결함 분석을 체계적으로 정리해 드립니다.

---

### 1. 어떤 일을 하는 코드에서 발생했는가
- **대상 기능**: `guard doctor`의 정책 파일 진단 기능 (`DOC-003`, [doctor.go](file:///home/yundream/mytask/kms/guard/internal/hub/setup/doctor.go))
- **역할**: 현재 워크스페이스에 유효한 `guard-policy.yaml`이 있는지 검사하고,
  - 정책 파일이 정상이면 `OK`,
  - 정책 파일이 아예 없으면 `WARN` (`--fix` 시 기본 프리셋 생성),
  - 기존 정책 파일에 YAML 문법 오류가 있으면 `ERROR` (사용자 파일을 덮어쓰지 않고 유지)로 판정해야 합니다.

---

### 2. 왜 발생했는가 (설계·호출 간 불일치)
정책 파일을 **생성하는 기준 경로**와 **검사하는 기준 경로**가 서로 다르게 동작했기 때문입니다.

- **생성 측 (`guard init`, `guard doctor --fix`)**: 항상 `<WorkspaceRoot>/guard-policy.yaml`에 파일을 만듭니다.
- **검사 측 (`policy.LoadDetailed`)**: 내부적으로 다음 순서로 파일을 찾습니다 ([loader.go](file:///home/yundream/mytask/kms/guard/internal/hub/policy/loader.go#L131-L166)).
  1. `$GUARD_POLICY_PATH`
  2. `<현재 터미널 작업 디렉토리(CWD)>/guard-policy.yaml`
  3. `<WorkspaceRoot>/.agents/guard-policy.yaml`
  4. `~/.config/guard/guard-policy.yaml`

즉, **`CWD`와 `WorkspaceRoot`가 다를 때**(예: 프로젝트 하위 디렉토리에서 실행하거나 `WORKSPACE_ROOT` 환경변수로 다른 경로를 지정한 경우), 검사 로직이 `<WorkspaceRoot>/guard-policy.yaml`을 아예 찾지 못합니다.
그 결과 파일이 실제로 존재하더라도 "파일이 없다(`builtin` fallback)"고 오판하게 됩니다.

---

### 3. 사용자 및 시스템에 미치는 영향과 임팩트 범위

#### 사용자 체감 영향
1. **거짓 경고(False Warning) 반복**: 사용자가 프로젝트 하위 폴더(예: `cmd/app/`)에서 `guard doctor`를 실행하면, 루트에 정상적인 `guard-policy.yaml`이 있는데도 "정책 파일이 없다"는 `WARN`이 계속 뜹니다. `guard doctor --fix`를 실행해도 여전히 `WARN`이 해소되지 않습니다.
2. **사용자 설정 파일 유실(데이터 손실 위험 — 심각)**: 사용자가 루트의 `guard-policy.yaml`을 수정하다가 오타(문법 오류)를 낸 상태에서 하위 폴더나 `WORKSPACE_ROOT` 지정으로 `guard doctor --fix`를 실행하면, 문법 오류를 감지하지 못하고 "파일 없음"으로 오판해 **사용자가 작성하던 정책 파일을 기본 템플릿으로 덮어써 버립니다.**

#### 임팩트 범위
- **영향받는 영역**: `guard doctor`의 `DOC-003` 진단 및 `--fix` 복구 로직 (`CWD != WorkspaceRoot`인 모든 실행 환경)
- **영향받지 않는 영역**:
  - `guard init` 자체의 파일 생성·병합·백업 로직 (QA 테스트 전체 통과)
  - `guard doctor`의 나머지 5개 항목(`DOC-001`, `DOC-002`, `DOC-004`, `DOC-005`, `DOC-006`)
  - 기존 핵심 기능(`launcher`, `i18n`, `handoff` E2E 회귀 테스트 전체 통과)

---

### 4. 수정 시 다른 컴포넌트에 미치는 영향 분석

수정 방법은 두 가지가 있으며, 다른 컴포넌트에 미치는 영향이 다릅니다.

| 구분 | 방안 A: `setup/doctor.go`만 수정 (권장) | 방안 B: `policy/loader.go`의 탐색 경로 변경 |
| :--- | :--- | :--- |
| **수정 내용** | `checkPolicyYAML`에서 `policy.LoadDetailedWithOptions`를 호출해 `CWD`를 `workspaceRoot`로 지정 + `--fix` 쓰기 전 파일 존재 여부 재확인 | `candidatePaths()`에 `<WorkspaceRoot>/guard-policy.yaml` 경로를 추가 |
| **기존 컴포넌트 영향** | **없음.** `internal/hub/policy`, `hook`, `launcher`, `tui` 등 기존 컴포넌트의 동작이 전혀 바뀌지 않음 | **있음.** `hook`, `launcher`, `status`, `tui` 등 `policy.Load*`를 쓰는 모든 컴포넌트의 정책 탐색 우선순위가 바뀜 |
| **회귀 리스크** | **매우 낮음.** `DOC-003` 검사 범위로만 국한됨 | **중간.** 기존 `01_core_policy_contract.md`의 5단계 정책 탐색 계약 및 기존 정책 테스트와 충돌 가능성 검토 필요 |
| **개발 원칙 부합** | 핵심 엔진/기존 정책 로더 무수정 원칙과 100% 일치 | 기존 공통 정책 로더 계약을 수정하게 됨 |

---

### 5. 결론 및 제안
- 이번 3건의 테스트 실패는 모두 **`DOC-003`이 `CWD` 기준으로 정책을 조회해 발생한 단일 결함**이며, 특히 깨진 사용자 정책 파일을 덮어쓰는 **데이터 손실 위험**이 있으므로 반드시 수정해야 합니다.
- 다른 컴포넌트에 영향이 없는 **방안 A(`internal/hub/setup/doctor.go`만 수정)**로 처리하는 것이 가장 안전합니다.

방안 A로 수정 진행할까요? (또는 `hook`/`status` 등에서도 하위 디렉토리 실행 시 `<WorkspaceRoot>/guard-policy.yaml`을 읽게 하려면 방안 B를 선택할 수도 있습니다.)

---

## 🧑‍💻 User · `2026-11-22 19:04:27` {#step-117}

사용자가 작업디렉토리를 벗어나서 진단 명령을 실행할 경우 발생하는 에러라는 건가요 ?

---

## 🤖 Antigravity · `2026-11-22 19:04:28` {#step-118}

네, 맞습니다. 정확히는 **명령을 실행하는 현재 위치(`CWD`)가 프로젝트 루트(`WorkspaceRoot`)와 다를 때** 발생합니다.

구체적으로는 다음 두 가지 상황에서 발생합니다:

1. **프로젝트 안의 하위 폴더에서 실행할 때**
   - 예: 프로젝트 루트가 `/repo`인데 `cd /repo/cmd/app`으로 이동한 상태에서 `guard doctor`를 실행하는 경우
   - `guard`는 상위 폴더의 `.agents`를 찾아 `WorkspaceRoot`를 `/repo`로 인식하지만, 정책 파일 검사는 현재 위치인 `/repo/cmd/app/guard-policy.yaml`만 확인하고 `/repo/guard-policy.yaml`을 놓칩니다.
2. **프로젝트 바깥에서 `WORKSPACE_ROOT`를 지정해 실행할 때**
   - 예: 홈 디렉토리 등에서 `WORKSPACE_ROOT=/repo guard doctor`를 실행하거나, 테스트 코드가 임시 디렉토리(`t.TempDir()`)를 `WorkspaceRoot`로 넘겨 호출하는 경우

반대로 사용자가 **정확히 프로젝트 루트 디렉토리(`/repo`)에 위치해서** `guard doctor`를 실행하면 `CWD == WorkspaceRoot`이므로 발생하지 않습니다.

---

## 🧑‍💻 User · `2026-11-22 19:05:46` {#step-119}

그러한 문제는 사용자에게 맡기는게 낫지 않나요 ? "000 설정을 발견하지 못했습니다. 진단 실패, 작업디렉토리를 확인해 주세요" 등으로 명시적으로 알려주는게 낫지, 뭔가 장치를 만드는게 좋은 판단인가요 ? 전문가로써 객관적이고 냉정하게 판단해주세요.

---

## 🤖 Antigravity · `2026-11-22 19:05:46` {#step-120}

객관적으로 판단하면, **사용자님이 경계하시는 "불필요한 편의 장치"를 붙이는 문제가 아니라, `doctor.go` 내부의 API 불일치 버그**입니다. 따라서 사용자에게 책임을 넘기기보다 `doctor.go`의 호출 방식을 바로잡는 것이 맞습니다.

그 이유를 3가지 근거로 정리합니다.

---

### 1. 이건 "마법 장치 추가"가 아니라 "같은 함수 안에서의 모순"입니다
현재 `RunDoctor(DoctorOptions{WorkspaceRoot: ws})`의 동작을 보면:
- `DOC-002` (`.agents/hooks.json`): `opts.WorkspaceRoot` 기준으로 검사
- `DOC-004` (`.agents/rules/guard-workflow.md`): `opts.WorkspaceRoot` 기준으로 검사
- `DOC-006` (symlink): `opts.WorkspaceRoot` 기준으로 검사
- **`DOC-003` (`guard-policy.yaml`)**:
  - **읽을 때** → `os.Getwd()` (프로세스 전역 CWD) 기준
  - **`--fix`로 쓸 때** → `opts.WorkspaceRoot` 기준

즉, 같은 함수 안에서 **읽는 경로와 쓰는 경로가 서로 다릅니다.** 이 모순 때문에 깨진 정책 파일이 있는데도 "없다"고 오판하고 기존 파일을 덮어쓰는 버그가 발생했습니다.

### 2. "작업 디렉토리를 확인해 주세요"로 사용자에게 맡길 경우의 문제
만약 `CWD != WorkspaceRoot`일 때 에러를 내고 사용자에게 루트로 이동하라고 요구하면 다음 두 가지 문제가 생깁니다.
1. **CLI 일관성 훼손**: `guard`는 이미 `findWorkspaceRoot()`로 상위 `.agents`를 찾아 프로젝트 루트를 인식하고 `WORKSPACE_ROOT` 환경변수도 지원합니다. `git`이나 `go`처럼 하위 폴더(`cmd/app`)에서 실행해도 프로젝트 루트 기준으로 동작하는 것이 표준 CLI 경험입니다.
2. **라이브러리 API 결함**: `RunDoctor(DoctorOptions{WorkspaceRoot: dir})`처럼 대상 경로를 파라미터로 받는 함수가 내부에서 전역 상태(`os.Getwd()`)에 의존하면, 호출자가 `os.Chdir()`을 강제로 호출해야만 정상 동작합니다. 이는 Go API 설계에서 안티패턴(전역 상태 의존)입니다.

### 3. 수정이 "장치 추가"가 아닌 이유
- `internal/hub/policy` 패키지에는 이미 `LoadDetailedWithOptions(opts LoadOptions)` 함수가 공식 API로 존재합니다.
- `doctor.go`에서 전역 `os.Getwd()`를 쓰는 `LoadDetailed(workspaceRoot)` 대신, `LoadDetailedWithOptions`에 `CWD: workspaceRoot`를 넘겨주는 것만으로 끝납니다(약 5줄 수정).
- 새로운 폴백 로직이나 추측 장치를 만드는 게 아니라, **`DOC-001`~`DOC-006` 전체가 동일하게 `opts.WorkspaceRoot`만 바라보도록 통일**하는 것입니다.

---

### 결론 (냉정한 판단)
- **맞는 판단 (수정해야 할 것)**: `doctor.go`의 `DOC-003`이 전역 `os.Getwd()` 대신 명시된 `opts.WorkspaceRoot`를 기준으로 검사하도록 고치는 것. (내부 모순 제거 + 데이터 손실 방지)
- **하지 말아야 할 것 (사용자님 지적이 맞는 부분)**:
  1. `internal/hub/policy/loader.go`의 공통 탐색 규칙을 억지로 늘리는 것 (기존 정책 로딩 우선순위를 흔드는 과잉 수정)
  2. 깨진 정책 파일이나 잘못된 설정을 임의로 고치려 드는 것 (이 부분은 명확히 `ERROR`로 알리고 사용자에게 맡겨야 함)

따라서 `doctor.go`에서 `LoadDetailedWithOptions`를 쓰도록 깔끔하게 1곳만 수정하는 것을 권장합니다. 진행할까요?

---

## 🧑‍💻 User · `2026-11-22 19:06:47` {#step-121}

개발 원칙 알려주세요.

---

## 🤖 Antigravity · `2026-11-22 19:06:47` {#step-122}

현재 프로젝트에 적용 중인 개발 원칙입니다.

1. **Core 코드 건드리지 않기 & 한 방향 의존성**
   - `internal/core`, `internal/hub/policy`, `internal/hub/handoff`는 수정하지 않고 기존 공개 API만 호출합니다.
   - 새 코드는 `internal/hub/setup`에 모으고, 의존성 방향(`cmd/guard/init.go` → `internal/hub/setup` → `policy` / `session` / `core`)을 한 방향으로 유지합니다.

2. **Fail-Fast (에러 무시·폴백·하드코딩 금지)**
   - 테스트를 통과시키려고 에러를 무시하거나, 폴백으로 실패를 숨기거나, 값을 하드코딩하지 않습니다.
   - 잘못된 입력이나 깨진 설정 파일은 즉시 에러를 리턴해 사용자에게 명확히 알립니다.

3. **안전한 파일 처리 (Safe Merge, Atomic Write, Idempotent)**
   - `.agents/hooks.json`을 merge할 때 기존 다른 키는 그대로 유지하고, 깨진 JSON이나 문법 오류가 있는 기존 정책 파일은 절대 덮어쓰지 않습니다.
   - 파일 쓰기는 `.tmp` → `os.Rename`의 Atomic Write를 사용하고, 반복 실행해도 결과가 같도록(Idempotent) 처리합니다.

4. **SSOT (Single Source of Truth)**
   - hook merge와 workflow rule 생성 로직은 `internal/hub/setup` 한 곳에만 두고, `launcher/bootstrap.go`는 기존 시그니처를 유지한 채 이를 호출합니다.

5. **Contract-First & QA 테스트 분리**
   - `doc/spec/06_init_automation_contract.md`의 상수, DTO, 시그니처, Exit Code 계약에 맞춰 구현합니다.
   - Black-box·E2E 테스트는 QA(TEST 세션)에 맡기고, 개발자는 프로덕션 코드 수정과 핸드오프에 집중합니다.

6. **수정 범위 엄수 & Git Diff Audit**
   - 합의된 개발 범위의 파일만 수정하고, 작업이 끝나면 반드시 `git diff`로 수정 범위를 검증하는 Audit 리포트를 작성합니다.

7. **단일 바이너리 오프라인 동작 & i18n 키 맞추기**
   - 외부 파일이나 네트워크 없이 바이너리 내장 템플릿만으로 동작하게 만들고, `ko.json`과 `en.json`의 키와 포맷 동사를 1:1로 맞춥니다.

---

## 🧑‍💻 User · `2026-11-22 19:07:13` {#step-123}

수정 작업 진행해 주세요.

---

## 🤖 Antigravity · `2026-11-22 19:09:36` {#step-129}

`DOC-003` 결함 수정, 테스트 검증, 핸드오프 완료 처리 및 QA 재검증 요청을 마쳤습니다.

### 1. 수정 내용 (Audit)
- **수정 파일 (1개)**: [doctor.go](file:///home/yundream/mytask/kms/guard/internal/hub/setup/doctor.go#L169-L220) (`+14 / -2`)
  - `checkPolicyYAML`이 프로세스 전역 CWD(`os.Getwd()`) 대신 `opts.WorkspaceRoot`를 기준으로 정책을 로드하도록 `policy.LoadDetailedWithOptions(...)` 호출로 변경.
  - `builtin` fallback 상태에서 `--fix`로 새 `guard-policy.yaml`을 쓰기 전, `<workspaceRoot>/guard-policy.yaml`이 이미 존재하면 덮어쓰지 않고 `ERROR`로 Fail-Fast 처리하는 방어 확인 추가.
- **기존 컴포넌트 영향**: `internal/hub/policy`, `internal/core`, `internal/hub/session`, `internal/hub/handoff` 등 기존 패키지 **변경 0건**.

### 2. 검증 결과 (`go test`)
- `go test -v ./internal/hub/setup/...`: 실패했던 3건 포함 **전체 PASS**
  - `TestSetup_RunDoctor_HealthyWorkspace`: PASS
  - `TestSetup_RunDoctor_DOC003_Policy/missing_policy_builtin_warn_and_autofix`: PASS
  - `TestSetup_RunDoctor_DOC003_Policy/broken_existing_policy_not_overwritten_on_fix`: PASS
- `go test ./...`: 전체 패키지(`setup`, `launcher`, `i18n`, `test/e2e` 등) **전체 PASS**

### 3. 핸드오프 처리
- **수신 티켓 완료 (`ack`)**: `hnd-20261122-2068c700`
- **QA 재검증 티켓 발행 (`send`)**: `hnd-20261122-c9db5614` (`VERIFY` → `TEST`, parent: `hnd-20261122-2068c700`)
  - `guard top` 또는 `guard handoff approve hnd-20261122-c9db5614`로 승인해 주시면 TEST 세션에서 최종 재검증을 진행합니다.

---

## 🧑‍💻 User · `2026-11-22 19:10:01` {#step-56}

개발자가 수정 작업을 완료하고 재검증 요청을 보내왔습니다. 검토해주세요.

---

## 🧑‍💻 User · `2026-11-22 20:38:13` {#step-71}

개발자가 수정 작업을 완료하고 재검증 요청을 보내왔습니다. 검토해주세요.

---

## 🤖 Antigravity · `2026-11-22 20:38:27` {#step-75}

개발자(`DEV/0c6f8f4b`)의 `DOC-003` 수정 건(`hnd-20261122-c9db5614`, Parent: `hnd-20261122-2068c700`)에 대한 재검증을 완료했습니다.

### 재검증 결과: **전체 PASS**
캐시 비활성화(`-count=1`) 기준으로 전체 테스트를 재실행한 결과 모든 패키지가 통과했습니다.

| 패키지 | 결과 | 소요 시간 | 주요 확인 항목 |
| :--- | :---: | :---: | :--- |
| `guard/internal/hub/setup` | **PASS** | 0.020s | 기존 실패 3건(`TestSetup_RunDoctor_HealthyWorkspace`, `DOC003/missing_policy_builtin_warn_and_autofix`, `DOC003/broken_existing_policy_not_overwritten_on_fix`) 모두 통과 |
| `guard/internal/adapters/launcher` | **PASS** | 0.013s | `EnsureWorkspaceHook` / `EnsureWorkflowRule` 호환성 및 `trigger: always_on` 포함 확인 |
| `guard/internal/i18n` | **PASS** | 0.004s | `ko.json` / `en.json` 키 및 포맷 동사 100% 대칭 |
| `guard/test/e2e` | **PASS** | 0.601s | `guard init` / `guard doctor` CLI E2E, Symlink 정규화, 기존 `handoff_*` 회귀 0건 |

### 핸드오프 처리
- 재검증 티켓 `hnd-20261122-c9db5614`을 `guard handoff ack`로 완료 처리했습니다. (현재 인박스 대기 티켓 0건)

별도로 [doc/test/](file:///home/yundream/mytask/kms/guard/doc/test)에 QA 최종 검증 리포트 작성이 필요하시면 말씀해 주세요.

---

