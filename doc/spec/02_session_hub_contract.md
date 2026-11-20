# [02_session_hub_contract.md] Session Hub 영속화 · GC · 동시성 계약서

> **대상 독자**: QA (블랙박스 테스트 작성 기준)
> **대상 패키지**: `guard/internal/hub/session`

---

## 1. 공개 상수 & DTO / Struct

```go
const MaxEvents = 50

type Event struct {
    Timestamp time.Time `json:"timestamp"`
    Decision  string    `json:"decision"` // "allow" 또는 "deny"
    Tool      string    `json:"tool"`
    Target    string    `json:"target"`
    RuleName  string    `json:"ruleName,omitempty"`
    Reason    string    `json:"reason,omitempty"`
}

type Session struct {
    ID             string    `json:"id"`
    Mode           string    `json:"mode"` // DEV, TEST, PLAN, ALL
    PID            int       `json:"pid"`
    Workspace      string    `json:"workspace"`
    CreatedAt      time.Time `json:"createdAt"`
    UpdatedAt      time.Time `json:"updatedAt"`
    ViolationCount int       `json:"violationCount"`
    Events         []Event   `json:"events"`
}
```

---

## 2. 공개 함수 및 `Manager` 메서드 계약

| 시그니처 | 동작 계약 |
| :--- | :--- |
| `DefaultDir() string` | 기본 저장소 경로 반환 (`~/.guard/sessions`) |
| `NewManager(dir string) *Manager` | 지정 디렉토리 기반 매니저 생성 (`dir == ""`이면 `DefaultDir()` 사용) |
| `ShortID(id string) string` | 8자 초과 시 앞 8자리 반환, 8자 이하이면 원문 반환 |
| `IsProcessAlive(pid int) bool` | `pid <= 0`이면 `false`. POSIX `Signal(0)` 기준 생존(`nil` 또는 `EPERM`) 시 `true` |
| `(m *Manager) Dir() string` | 세션 디렉토리 경로 반환 |
| `(m *Manager) SessionPath(id string) string` | `<dir>/<sanitizedID>.json` 경로 반환 (정상 ID는 원문 유지; `/`, `\`, `..`, 선행 `.` 치환이 발생한 비표준 ID는 `_` 치환 후 원본의 결정적 8자리 SHA-256 해시 접미사를 부여해 파일명 충돌 방지) |
| `(m *Manager) Create(sess *Session) error` | `sess == nil` 또는 빈 ID 시 에러. `CreatedAt`/`UpdatedAt` 초기화, `Mode` 정규화(빈 값 → `DEV`), `Events` nil 방지 후 원자적 저장 |
| `(m *Manager) Save(sess *Session) error` | 임시 파일 + `Sync()` + `os.Rename()`으로 원자적 덮어쓰기 |
| `(m *Manager) Get(id string) (*Session, error)` | 단일 세션 JSON 로드 (`Events`는 nil 대신 빈 슬라이스 보장) |
| `(m *Manager) List() ([]*Session, error)` | 디렉토리 내 유효한 `*.json` 로드 (`.` 접두사 파일 및 손상된 JSON은 스킵). `UpdatedAt` 내림차순, 동률 시 `ID` 오름차순 정렬 |
| `(m *Manager) Delete(id string) error` | 세션 JSON 및 연관 `.lock` 파일 삭제. 이미 없는 파일은 에러 없이 `nil` 반환 |
| `(m *Manager) DeleteAll() (int, error)` | 모든 `.json` 세션 파일과 `.lock` 파일 삭제. 삭제된 `.json` 건수만 반환 |
| `(m *Manager) UpdateMode(id, mode string) (*Session, error)` | **파일 락(`syscall.Flock`) 보호** 하에 모드 정규화·갱신 및 `UpdatedAt` 갱신 |
| `(m *Manager) ResetViolations(id string) (*Session, error)` | **파일 락 보호** 하에 `ViolationCount = 0` 초기화 및 `UpdatedAt` 갱신 |
| `(m *Manager) AppendEvent(id string, ev Event) (*Session, error)` | **파일 락 보호** 하에 이벤트 추가, `MaxEvents(50)` 초과 시 오래된 순으로 로테이션, `decision == "deny"`이면 `ViolationCount++`, `UpdatedAt` 갱신 |
| `(m *Manager) CleanAndList() ([]*Session, []string, error)` | 디렉토리 1회 스캔으로 **`PID <= 0` 또는 사망 PID 세션을 즉시 삭제**하고, `(생존 세션 목록, 삭제된 ID 목록, err)` 반환 |
| `(m *Manager) CleanStaleSessions() ([]string, error)` | `CleanAndList()` 래퍼 — `(삭제된 ID 목록, err)` 반환 |

---

## 3. 핵심 불변 계약 (QA 검증 포인트)

1. **`PID <= 0` (Unbound) 정책**:
   - `guard` 런처를 거치지 않은 `PID <= 0` 세션은 보존하지 않으며 `CleanAndList()` / `CleanStaleSessions()` 호출 시 **즉시 삭제**됩니다.
2. **동시성 무결성 (`Flock`)**:
   - 여러 고루틴/프로세스가 동일 세션에 `AppendEvent`, `UpdateMode`, `ResetViolations`를 동시 호출해도 **Lost Update 없이** 모든 이벤트/차단 횟수/모드 변경이 보존되어야 합니다.
3. **손상 파일 내성**:
   - 디렉토리 안에 깨진 JSON 파일이나 `.lock` 임시 파일이 있어도 `List()`는 에러를 내지 않고 정상 세션만 반환해야 합니다.
4. **숨김 파일 방어**:
   - `.hidden` 같은 선행 `.`이 포함된 ID도 `sanitizeID`가 `_hidden`으로 치환하므로 `Create()` 후 `List()`에서 누락되지 않아야 합니다.
