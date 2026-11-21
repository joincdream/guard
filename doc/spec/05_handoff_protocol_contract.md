# [05_handoff_protocol_contract.md] 범용 핸드오프 프로토콜 계약서 (`guard handoff`)

> **대상 독자**: QA (블랙박스 테스트 작성 기준)  
> **대상 패키지**: `guard/internal/hub/handoff`, `guard/internal/core`, `guard/internal/adapters/launcher`, `guard/internal/adapters/tui`, `guard/cmd/guard`  
> **관련 설계서**: [08_handoff_protocol.md](../08_handoff_protocol.md)

---

## 1. Handoff Hub 공개 상수 · 에러 · DTO (`guard/internal/hub/handoff`)

```go
type HandoffType string
const (
    TypeTask   HandoffType = "task"
    TypeVerify HandoffType = "verify"
    TypeFix    HandoffType = "fix"
    TypeReview HandoffType = "review"
)

type HandoffStatus string
const (
    StatusPending  HandoffStatus = "pending"
    StatusApproved HandoffStatus = "approved"
    StatusRejected HandoffStatus = "rejected"
    StatusAcked    HandoffStatus = "acked"
)

const (
    ModePlan = "plan"
    ModeDev  = "dev"
    ModeTest = "test"
    ModeAll  = "all"
)

const DefaultMaxQueueItems = 200

var (
    ErrNotFound          = errors.New("handoff item not found")
    ErrInvalidItem       = errors.New("invalid handoff item")
    ErrInvalidTransition = errors.New("invalid handoff status transition")
    ErrInvalidSessionID  = errors.New("invalid session ID")
)

type HandoffNode struct {
    Mode      string `json:"mode,omitempty"`
    SessionID string `json:"session_id,omitempty"`
}

type HandoffItem struct {
    ID           string        `json:"id"`
    ParentID     string        `json:"parent_id,omitempty"`
    Type         HandoffType   `json:"type"`
    From         HandoffNode   `json:"from"`
    To           HandoffNode   `json:"to"`
    Summary      string        `json:"summary"`
    References   []string      `json:"references,omitempty"`
    Targets      []string      `json:"targets,omitempty"`
    Payload      string        `json:"payload,omitempty"`
    Status       HandoffStatus `json:"status"`
    RejectReason string        `json:"reject_reason,omitempty"`
    CreatedAt    time.Time     `json:"created_at"`
    UpdatedAt    time.Time     `json:"updated_at"`
}

type ListFilter struct {
    Status      HandoffStatus
    ToMode      string
    ToSessionID string
    OnlyActive  bool // pending 또는 approved
}

type ApproveResult struct {
    Item        HandoffItem
    SessionID   string
    InboxPath   string
    ScopeLocked bool
    Buffered    bool
}
```

---

## 2. 공개 함수 및 `Manager` 메서드 계약

### 2.1 모델 · 검증 · Envelope
| 시그니처 | 동작 계약 |
| :--- | :--- |
| `ValidType(t HandoffType) bool` | `task`, `verify`, `fix`, `review`만 `true` |
| `ValidStatus(s HandoffStatus) bool` | `pending`, `approved`, `rejected`, `acked`만 `true` |
| `ValidAgentMode(m string) bool` | 대소문자 무관 `plan`, `dev`, `test`, `all`만 `true` |
| `ValidSafeID(id string) bool` | 빈 값, `.`, `..`, `..` 포함, `^[A-Za-z0-9._-]+$` 위반 시 `false` |
| `ParseTarget(raw string) (HandoffNode, error)` | `plan\|dev\|test\|all`이면 `Mode` 설정, 안전한 ID면 `SessionID` 설정, 빈 값·경로 탈출(`../`) 등은 `ErrInvalidItem` 반환 |
| `(item HandoffItem) Validate() error` | `Type`, `Status`, `From`, `To`, 비어있지 않은 `Summary`, 빈 요소 없는 `References`/`Targets`, 안전한 `ID`/`ParentID` 검증 |
| `ValidateStatusTransition(from, to HandoffStatus) error` | 허용 전이: `pending → approved`, `pending → rejected`, `approved → acked`. 그 외 모든 전이는 `ErrInvalidTransition` 반환 |
| `GenerateID(now time.Time) (string, error)` | `hnd-YYYYMMDD-<8자리 hex>` 형식의 충돌 방지 ID 생성 |
| `ShouldApplyScopeLock(item HandoffItem, targetMode string) bool` | `(Type == fix \|\| Type == task) && len(Targets) > 0 && (targetMode == dev \|\| targetMode == all)`일 때만 `true` |
| `FormatEnvelope(item HandoffItem, scopeLocked bool) (string, error)` | `<!-- GUARD_HANDOFF_ENVELOPE: START ... -->` ~ `<!-- GUARD_HANDOFF_ENVELOPE: END -->` 마크다운 생성. `len(Targets) > 0`일 때 `scopeLocked == true`이면 `수정 허용 대상 (Scope Locked)`, `false`이면 `작업 대상` 헤더 출력(`Targets`/`References`/`Payload`가 비어 있으면 해당 섹션 생략). `ID`/`Type`/`Summary` 누락 시 `ErrInvalidItem` |

### 2.2 큐 · 인박스 · 승인 라우터
| 시그니처 | 동작 계약 |
| :--- | :--- |
| `NewManager(handoffDir, sessionDir string) (*Manager, error)` | 빈 경로 시 기본값(`~/.guard/handoff`, `~/.guard/sessions`) 사용 |
| `(m *Manager) QueuePath() string` | `<handoffDir>/queue.json` 반환 |
| `(m *Manager) InboxPath(sessionID string) (string, error)` | `<sessionDir>/<sessionID>.inbox` 반환 (`ValidSafeID` 위반 시 `ErrInvalidSessionID`) |
| `(m *Manager) Enqueue(item HandoffItem) (HandoffItem, error)` | ID 미지정 시 자동 생성, `Status = pending`, 유효성 검증 및 중복 ID 차단 후 `Flock` + Atomic Write 저장 |
| `(m *Manager) List(filter ListFilter) ([]HandoffItem, error)` | `Status`, `ToMode`, `ToSessionID`, `OnlyActive` 필터 적용 조회 |
| `(m *Manager) Get(id string) (*HandoffItem, error)` | ID로 단건 조회 (미존재 시 `ErrNotFound`) |
| `(m *Manager) UpdateStatus(id string, status HandoffStatus, reason string) (*HandoffItem, error)` | `ValidateStatusTransition` 검증 후 `Flock` 보호 하에 상태·`UpdatedAt`·`RejectReason` 갱신 |
| `(m *Manager) DeliverToInbox(sessionID string, item HandoffItem, scopeLocked bool) (string, error)` | 세션별 `.inbox.lock` 보호 하에 Markdown Envelope을 원자적으로 추가 기록 |
| `(m *Manager) ConsumePendingForMode(mode, sessionID string) ([]HandoffItem, error)` | `Flock` 임계 구역 내에서 `Status == approved && To.SessionID == "" && To.Mode == mode` 티켓을 모두 원자적으로 `sessionID`에 할당하고 `.inbox`에 전달 |
| `ApproveAndRoute(handoffMgr, sessMgr, workspaceRoot, id string) (ApproveResult, error)` | `Flock` 임계 구역 내에서 `pending → approved` 검증 후 ① 활성 세션 존재 시 조건부 `UpdateScope` + `DeliverToInbox` + `To.SessionID` 기록(`Buffered: false`), ② `--to <mode>`에 활성 세션 부재(`session.ErrNoActiveSession`) 시 `approved` 보관(`Buffered: true`), ③ `--to <sessionId>`가 미존재 시 에러 반환(상태 변경 안 함) |

---

## 3. Core 신뢰 경계 계약 (`guard/internal/core`)

- **`guard handoff approve` 자체 승인 차단**:
  - `EvaluateRequest(policy, req)`는 **모드(`DEV`/`TEST`/`PLAN`/`ALL`)와 정책(`policy == nil` 포함)에 무관하게** `req.CommandLine`에서 `guard handoff approve` 실행을 차단합니다.
  - 차단 결과: `Decision{Allowed: false, RuleName: "handoff-self-approve-block", Reason: "..."}`.
  - 직접 호출(`guard handoff approve hnd-1`), 셸 체이닝(`echo ok && guard handoff approve hnd-1`, `a; guard handoff approve hnd-1`), 인라인 스크립트 래퍼(`bash -c "guard handoff approve hnd-1"`, `sh -c`, `python3 -c "...guard...handoff...approve..."`) 모두 차단되어야 합니다.
  - 반면 `guard handoff send`, `guard handoff inbox`, `guard handoff ack`, `guard handoff reject`는 차단하지 않습니다.

---

## 4. CLI 계약 (`guard handoff`)

| 명령 | 동작 계약 | 종료 코드 |
| :--- | :--- | :---: |
| `guard handoff send --to <mode\|sessionId> [--type <type>] [--ref <file>...] [--targets <glob>...] [--parent <id>] -m "<summary>" [--payload "<text>"]` | `pending` 티켓 발행 및 큐 저장 (`--ref`, `--targets`는 반복 플래그 및 쉼표 구분 지원) | `0` / `1` |
| `guard handoff inbox [--all] [--mode <mode>] [--session <id>] [--json]` | 기본은 활성(`pending`/`approved`) 티켓, `--all`은 완료 포함 전체 출력 | `0` / `1` |
| `guard handoff approve <id>` | `ApproveAndRoute` 실행 (전달 완료 또는 오프라인 대기 결과 출력) | `0` / `1` |
| `guard handoff reject <id> [-m "<reason>"]` | `rejected` 상태 전이 | `0` / `1` |
| `guard handoff ack <id>` | `approved → acked` 상태 전이 | `0` / `1` |

---

## 5. 핵심 불변 계약 (QA 검증 포인트)

1. **상태 머신 무결성**:
   - `pending → acked`, `rejected → approved`, `acked → approved`, `approved → approved`(중복 승인)는 모두 `ErrInvalidTransition`으로 거부되어야 합니다.
2. **오프라인 버퍼링 중복 소비 방지**:
   - `approved` 상태의 모드 대상 티켓 1건에 대해 두 세션이 동시에 `ConsumePendingForMode`를 호출해도 **정확히 1개 세션에만 전달**되어야 합니다.
   - 이미 활성 세션에 전달된(`To.SessionID != ""`) 티켓은 이후 새 런처 기동 시 `ConsumePendingForMode`가 재전달하지 않아야 합니다.
3. **조건부 Scope Lock 결합**:
   - `Type == fix` 또는 `Type == task` + `Targets` 비어있지 않음 + 수신 모드 `DEV`/`ALL`일 때만 `Session.AllowedWritePaths`가 설정됩니다.
   - `Type == review` 또는 `Type == verify`, 또는 수신 모드 `PLAN`/`TEST`에서는 `AllowedWritePaths`를 변경하지 않습니다.
4. **큐 회전(Compaction) 보존 규칙**:
   - `DefaultMaxQueueItems(200)` 초과 시 완료된(`acked`/`rejected`) 티켓만 오래된 순으로 제거하며, 활성(`pending`/`approved`) 티켓은 절대 유실하지 않습니다.
5. **TUI 오버레이 우선순위 및 대용량 파일 방어**:
   - 도움말(`?`) 또는 이벤트 상세 모달(`Space`)이 열려 있을 때는 새 `pending` 핸드오프가 도착해도 포커스를 강제로 가로채지 않습니다.
   - `[V]` 참조 문서 뷰어는 256KB를 초과하는 파일에서도 크래시·프리즈 없이 잘림 안내(`tui.handoff.ref_truncated`)와 함께 렌더링되어야 합니다.
   - `ko.json` / `en.json`의 신규 `tui.handoff.*`, `tui.footer.handoff` 키 집합과 포맷 동사 개수가 100% 일치해야 합니다.
