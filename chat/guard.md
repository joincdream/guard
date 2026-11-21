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

