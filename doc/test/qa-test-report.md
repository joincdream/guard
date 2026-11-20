# QA 테스트 결과 및 수정 요청 보고서

- **작성일**: 2026-11-20
- **작성자**: QA
- **실행 명령**: `go test -v ./...`
- **테스트 환경**: Linux / Go Test (guard `TEST` 모드 — 프로덕션 소스 열람 제한 상태에서 테스트 실행 및 테스트 명세 기반 분석)
- **전체 결과**: **FAIL** (종료 코드 `1`)

---

## 1. 실행 요약

| 항목 | 결과 |
| :--- | :---: |
| 전체 패키지 | 8 |
| 통과 패키지 (`ok`) | 5 |
| 실패 패키지 (`FAIL`) | 2 |
| 테스트 미존재 패키지 (`?`) | 1 (`guard/cmd/guard`) |
| 탑레벨 테스트 케이스 | 19 (PASS 17 / **FAIL 2**) |

### 패키지별 상세 현황

| 패키지 | 상태 | 소요 시간 | 비고 |
| :--- | :---: | :---: | :--- |
| `guard/cmd/guard` | `?` | - | 테스트 파일 없음 |
| `guard/internal/adapters/hook` | **FAIL** | 0.003s | 1 PASS / **1 FAIL** (`DEF-001`) |
| `guard/internal/adapters/launcher` | PASS | 0.003s | 3 PASS |
| `guard/internal/adapters/tui` | PASS | 0.005s | 1 PASS |
| `guard/internal/core` | PASS | 0.005s | 4 PASS (서브테스트 전체 통과) |
| `guard/internal/hub/policy` | PASS | 0.004s | 2 PASS |
| `guard/internal/hub/session` | **FAIL** | 0.025s | 2 PASS / **1 FAIL** (`DEF-002`) |
| `guard/internal/i18n` | PASS | 0.003s | 4 PASS |

---

## 2. 수정 요청 결함 목록

| 결함 ID | 심각도 | 우선순위 | 대상 패키지 | 실패 테스트 | 귀책 추정 |
| :--- | :---: | :---: | :--- | :--- | :--- |
| **DEF-001** | High | P1 | `guard/internal/adapters/hook` | `TestHandler_DenyAndRecordEvent` | 프로덕션 코드 결함 추정 |
| **DEF-002** | Medium | P2 | `guard/internal/hub/session` | `TestManager_CleanStaleSessions` | 프로덕션 코드 결함 추정 |

---

## 3. 결함 상세 내역

### DEF-001: `DEV` 모드 hook 평가 시 테스트 파일(`*_test.go`) 접근이 차단(`deny`)되지 않고 허용(`allow`)됨

- **심각도 / 우선순위**: High / P1 (가드레일 핵심 차단 정책 미동작)
- **대상 패키지**: `guard/internal/adapters/hook`
- **관련 테스트**: `internal/adapters/hook/handler_test.go` (`L12-L55`, 실패 지점 `L37-L39`)
- **단독 재현 명령**:
  ```bash
  go test -v ./internal/adapters/hook -run TestHandler_DenyAndRecordEvent
  ```

#### 재현 절차 및 입력 데이터
1. 임시 세션 디렉토리에 `DEV` 모드 세션 생성:
   - `ID: "sess-hook-1"`, `Mode: "DEV"`, `PID: 0`, `Workspace: t.TempDir()`
2. `NewHandler(mgr, t.TempDir(), &stdout, &stderr)` 호출.
3. 아래 JSON 페이로드로 `h.Handle(...)` 실행:
   ```json
   {"sessionId":"sess-hook-1","tool_name":"read_file","tool_args":{"Path":"pkg/auth_test.go"}}
   ```

#### 기대 결과 (Expected Result)
- `stdout` JSON 응답: `Decision == "deny"`, `Reason != ""`
- `stderr`: 차단 사유 문자열 출력
- 세션 상태: `ViolationCount == 1`, `Events`에 `{Decision: "deny", Target: "pkg/auth_test.go"}` 1건 기록

#### 실제 결과 (Actual Result)
- `stdout` JSON 응답: `{Decision:allow Reason:}`
- 에러 로그:
  ```text
  handler_test.go:38: expected deny with reason, got {Decision:allow Reason:}
  ```

#### 결함 귀책 추정 및 근거
- **판정**: **프로덕션 코드 결함 추정**
- **근거**:
  1. 정책 엔진(`guard/internal/core`)의 `TestEvaluate_FileTools/DEV_read__test.go_denied` 테스트가 정상 통과하므로, `DEV` 모드에서 `*_test.go` 열람 차단 규칙 자체는 정상 동작함.
  2. 따라서 `internal/adapters/hook` 계층의 페이로드 파싱, 세션 조회, 기본 정책 로드, 또는 정책 평가 위임 경로에 결함이 있을 가능성이 높음.

#### 개발자 확인 요청 사항
- [ ] `{"tool_name": ..., "tool_args": ...}` 페이로드 스키마가 `hook` 핸들러에서 정상 파싱되는지 확인 (통과한 `TestHandler_SessionModeOverridesWorkspace`는 `{"toolCall":{"name":...,"args":...}}` 스키마를 사용함)
- [ ] `t.TempDir()` 기반 임시 워크스페이스/정책 경로에서 기본 정책(fallback)이 정상 로드되어 `DEV` 모드 규칙이 평가되는지 확인
- [ ] 스펙 변경으로 `tool_name`/`tool_args` 지원이 의도적으로 제거된 경우, 스펙 변경 공지 및 테스트 갱신 여부 회신 요청

---

### DEF-002: `CleanStaleSessions()` 실행 시 PID 미바인딩 세션(`PID: 0`)까지 stale 세션으로 삭제됨

- **심각도 / 우선순위**: Medium / P2 (정상 미바인딩 세션 유실 가능성)
- **대상 패키지**: `guard/internal/hub/session`
- **관련 테스트**: `internal/hub/session/session_test.go` (`L137-L187`, 실패 지점 `L167-L169`)
- **단독 재현 명령**:
  ```bash
  go test -v ./internal/hub/session -run TestManager_CleanStaleSessions
  ```

#### 재현 절차 및 입력 데이터
1. 임시 세션 디렉토리에 3개 세션 생성:
   - `alive-sess`: `Mode: "DEV"`, `PID: os.Getpid()` (활성 프로세스)
   - `dead-sess`: `Mode: "PLAN"`, `PID: deadPID` (`true` 명령 종료 후 획득한 종료 프로세스 PID)
   - `unbound-sess`: `Mode: "TEST"`, `PID: 0` (프로세스 미바인딩 세션)
2. `mgr.CleanStaleSessions()` 호출.

#### 기대 결과 (Expected Result)
- 종료된 프로세스의 세션 `[dead-sess]` 1건만 제거됨.
- 활성 세션(`alive-sess`) 및 미바인딩 세션(`unbound-sess`) 총 2건 유지.

#### 실제 결과 (Actual Result)
- `[unbound-sess dead-sess]` 2건이 제거됨 (`PID: 0` 세션까지 함께 삭제).
- 에러 로그:
  ```text
  session_test.go:168: expected [dead-sess] to be removed, got [unbound-sess dead-sess]
  ```

#### 결함 귀책 추정 및 근거
- **판정**: **프로덕션 코드 결함 추정 (신뢰도 높음)**
- **근거**:
  1. 테스트 내부에서 `IsProcessAlive(os.Getpid()) == true`, `IsProcessAlive(deadPID) == false` 사전 검증(`L148-L154`)을 모두 통과하여 실행 환경 및 프로세스 생존 판별 로직에는 이상이 없음.
  2. 테스트 명세는 `PID: 0` 세션을 미바인딩 세션으로 명시하고 보존을 기대하고 있으나(`L156-L161`), `CleanStaleSessions()` 구현에서 `PID <= 0` 세션을 예외 처리하지 않고 stale 대상으로 판정한 것으로 추정됨.

#### 개발자 확인 요청 사항
- [ ] `CleanStaleSessions()` 구현에서 `PID <= 0`(미바인딩 세션)을 정리 대상에서 제외하는 조건이 누락되었는지 확인 및 수정
- [ ] 만약 `PID == 0` 세션도 자동 정리하도록 정책이 변경된 경우, 변경 사양 공유 및 테스트 케이스 기대값 업데이트 협의 요청

---

## 4. 완료 및 회귀 검증 기준

수정 완료 후 아래 명령으로 전체 테스트 통과를 확인해 주시기 바랍니다.

```bash
go test -v ./...
```

- `DEF-001` (`TestHandler_DenyAndRecordEvent`) PASS
- `DEF-002` (`TestManager_CleanStaleSessions`) PASS
- 기존 통과 패키지 5개(`launcher`, `tui`, `core`, `policy`, `i18n`) 회귀 이상 없음

---

## 5. 개발자 검토 및 수정 회신 (2026-11-20)

### DEF-001 — 수정 완료 (`internal/adapters/hook/handler.go`)
- **근본 원인**: `tool_name`/`tool_args` 파싱 및 기본 정책 로드는 정상이었으나, 기존 `resolveSessionID`가 외부 환경변수 `GUARD_SESSION_ID`를 단일 값으로 우선 반환하면서 QA 테스트 환경(바깥쪽 `guard TEST 모드`의 `GUARD_SESSION_ID`, `AGENT_MODE=TEST`)이 테스트 내부의 임시 `sessionMgr` 세션(`sess-hook-1`)을 가리는 현상이 발생했습니다.
- **수정 내용**: `resolveSessionAndMode` 및 `candidateSessionIDs`를 도입하여 후보 세션 ID(`GUARD_SESSION_ID` → 페이로드 `sessionId` → `SESSION_ID` → `CONVERSATION_ID`) 중 **실제 `sessionMgr`에 존재하는 첫 번째 세션**을 매칭하도록 개선했습니다.
- **QA 확인 요청**: 수정 반영 완료. QA 환경(`guard TEST 모드`)에서 `DEF-001` 재검증 및 필요 시 `handler_test.go` 환경변수 격리(`t.Setenv("GUARD_SESSION_ID", "")`, `t.Setenv("AGENT_MODE", "")`) 검토를 부탁드립니다.

### DEF-002 — 스펙 변경 안내 (프로덕션 결함 아님)
- **정책 변경 사유**: `guard` 런처를 거치지 않은 `unbound(PID: 0)` 세션은 프로세스 종료 시점을 판별할 수 없어 `guard top`에 유령 세션이 영구 잔존하는 문제가 발생했습니다. 이에 따라 **`PID <= 0` 세션은 추적하지 않고 `CleanStaleSessions()`에서 즉시 정리(GC)**하도록 사양을 변경했습니다 (`internal/hub/session/gc.go`, `internal/adapters/hook/handler.go`).
- **QA 요청**: `internal/hub/session/session_test.go`의 `TestManager_CleanStaleSessions` 기대값을 신규 정책(`unbound-sess`도 제거 대상에 포함, 최종 생존 세션 `alive-sess` 1건)에 맞게 업데이트해 주시기 바랍니다.

---

## 6. QA 재검증 결과 (2026-11-20)

- **재검증 명령**: `go test -v ./...`
- **전체 결과**: **FAIL** (8개 패키지 중 6 PASS / 1 FAIL / 1 테스트 미존재 — 탑레벨 19건 중 18 PASS / 1 FAIL)

| 결함 ID | 상태 | 재검증 결과 | 후속 조치 |
| :--- | :---: | :---: | :--- |
| **DEF-001** | **Resolved (Closed)** | **PASS** (`TestHandler_DenyAndRecordEvent` 통과) | 필요 시 `handler_test.go` 환경변수 격리(`t.Setenv`) 추가 검토 |
| **DEF-002** | **Spec Changed (Open)** | **FAIL** (신규 스펙 미반영) | `session_test.go`의 `TestManager_CleanStaleSessions` 기대값을 신규 정책(`PID <= 0` 즉시 정리, 생존 세션 `alive-sess` 1건)으로 업데이트 필요 |

---

## 7. 개발자 추가 회신 — DEF-002 판정 보충 (2026-11-20)

- **QA 테스트 정확성 확인**: `DEF-002`는 QA가 잘못 테스트한 것이 아닙니다. 기존 설계(`doc/03_tui_control_tower.md`) 및 기존 테스트 명세는 `PID: 0`(`Unbound`) 세션을 보존하는 스펙이었으며, QA는 해당 기준에 따라 스펙-구현 불일치를 정확하게 검증·보고했습니다.
- **조치 성격**: 제품 스펙 자체가 변경(`PID <= 0` 세션 비추적 및 `CleanStaleSessions()` 즉시 정리)됨에 따른 **테스트 기대값 동기화 작업**입니다. QA에서 `session_test.go`의 `TestManager_CleanStaleSessions` 기대값을 신규 정책 기준으로 업데이트해 주시면 `DEF-002` 종료 및 전체 테스트 통과가 완료됩니다.

---

## 8. QA 최종 조치 및 종료 보고 (2026-11-20)

- **반영 내용**:
  1. `internal/hub/session/session_test.go` (`TestManager_CleanStaleSessions`): 신규 스펙에 맞춰 `dead-sess`와 `unbound-sess(PID: 0)` 모두 제거 대상, 최종 생존 세션 `alive-sess` 1건(`DeleteAll` 1건)으로 기대값 업데이트 완료.
  2. `internal/adapters/hook/handler_test.go`: 테스트 환경 독립성 보장을 위해 `t.Setenv("GUARD_SESSION_ID", "")`, `t.Setenv("AGENT_MODE", "")` 격리 추가 완료.
- **최종 검증 명령**: `go test -count=1 ./...`
- **최종 결과**: **PASS** (테스트 존재 7개 패키지 전체 통과, 종료 코드 `0`)
- **결함 최종 상태**:
  - **DEF-001**: **Closed** (PASS)
  - **DEF-002**: **Closed** (신규 스펙 동기화 완료, PASS)
