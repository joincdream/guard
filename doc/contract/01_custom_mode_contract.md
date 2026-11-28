# Contract 01: 동적(커스텀) 모드 지원 (`core`, `handoff`, `cli`)

## 1. `guard/internal/core`

### 관련 타입 및 상수
```go
const (
    ModeDev  = "DEV"
    ModeTest = "TEST"
    ModePlan = "PLAN"
    ModeAll  = "ALL"
    ModeQA   = "QA"
)

type Rule struct {
    Name            string   `yaml:"name"`
    Mode            string   `yaml:"mode"`
    Type            string   `yaml:"type"` // "read", "write", "command", "exec"
    Patterns        []string `yaml:"patterns"`
    ExcludePatterns []string `yaml:"exclude_patterns"`
    Message         string   `yaml:"message"`
}

type Policy struct {
    Version string `yaml:"version"`
    Rules   []Rule `yaml:"rules"`
}
```

### 신규 공개 함수

#### `ExtractModes`
```go
func ExtractModes(policy *Policy) []string
```
* **동작 계약**:
  * `policy != nil`인 경우 `policy.Rules`를 순서대로 순회하며 각 `rule.Mode`를 `NormalizeMode`(공백 제거 + 대문자 변환)한 뒤 중복을 제거해 반환한다.
  * `rule.Mode`가 빈 문자열(`""`)인 전역 규칙은 모드 목록에 포함하지 않는다.
  * 규칙 유무와 무관하게 기본 바이패스 모드 `ModeAll` (`"ALL"`)은 항상 결과 슬라이스에 포함된다(이미 있으면 중복 추가하지 않음).
  * `policy == nil`이면 `[]string{"ALL"}`을 반환한다.

#### `HasMode`
```go
func HasMode(policy *Policy, mode string) bool
```
* **동작 계약**:
  * `mode`를 `NormalizeMode`로 정규화한다. 빈 문자열이면 `false`를 반환한다.
  * `IsBypassMode(mode)` (`"ALL"` 또는 `"QA"`)이면 `policy`가 `nil`이더라도 `true`를 반환한다.
  * `policy == nil`이면 바이패스 모드가 아닌 경우 `false`를 반환한다.
  * `policy.Rules` 중 `NormalizeMode(rule.Mode) == NormalizeMode(mode)`인 규칙이 하나 이상 있으면 `true`, 없으면 `false`를 반환한다.

---

## 2. `guard/internal/hub/handoff`

### 관련 DTO
```go
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
```

### 신규 및 확장 공개 함수 / 메서드

#### `IsKnownMode`
```go
func IsKnownMode(m string, knownModes []string) bool
```
* **동작 계약**:
  * `ValidAgentMode(m)`(기본 모드 `"plan"`, `"dev"`, `"test"`, `"all"`, 대소문자/앞뒤 공백 무시)이면 `true`를 반환한다.
  * `m`이 빈 문자열/공백이면 `false`를 반환한다.
  * `knownModes`의 원소 중 `strings.EqualFold`로 일치하는 항목이 있으면 `true`, 없으면 `false`를 반환한다.

#### `ParseTarget` / `ParseTargetWithModes`
```go
func ParseTarget(raw string) (HandoffNode, error)
func ParseTargetWithModes(raw string, knownModes []string) (HandoffNode, error)
```
* **동작 계약**:
  * `raw`를 `strings.TrimSpace`한 값이 빈 문자열이면 `ErrInvalidItem` 래핑 에러를 반환한다.
  * `IsKnownMode(strings.ToLower(trimmed), knownModes)`가 `true`이면 `HandoffNode{Mode: strings.ToLower(trimmed)}`를 반환한다.
  * 모드가 아닌 경우 `ValidSafeID(trimmed)`를 검사해 유효하면 `HandoffNode{SessionID: trimmed}`를 반환하고, 유효하지 않으면 `ErrInvalidItem` 래핑 에러를 반환한다.
  * `ParseTarget(raw)`는 `ParseTargetWithModes(raw, nil)`과 동일하게 동작한다(하위 호환성 유지).

#### 검증 메서드 (`WithModes` 계열)
```go
func (n HandoffNode) ValidateAsTarget() error
func (n HandoffNode) ValidateAsTargetWithModes(knownModes []string) error
func (n HandoffNode) ValidateAsSender() error
func (n HandoffNode) ValidateAsSenderWithModes(knownModes []string) error
func (item HandoffItem) Validate() error
func (item HandoffItem) ValidateWithModes(knownModes []string) error
```
* **동작 계약**:
  * `ValidateAsTarget` / `ValidateAsSender` / `Validate`는 각각 `knownModes = nil`로 `...WithModes`에 위임한다.
  * `n.Mode != ""`일 때 `IsKnownMode(n.Mode, knownModes)`가 `false`이면 `ErrInvalidItem` 래핑 에러를 반환한다.

#### `Manager.SetKnownModes`
```go
func (m *Manager) SetKnownModes(modes []string)
```
* **동작 계약**:
  * `Manager.Enqueue(item)` 호출 시 `item.ValidateWithModes(m.knownModes)`를 사용하여 커스텀 모드를 포함한 발신/수신 노드를 검증한다.
  * `SetKnownModes`를 호출하지 않으면(`knownModes == nil`) 기존과 동일하게 기본 4개 모드만 허용한다.

---

## 3. CLI 라우팅 (`cmd/guard`)

* 워크스페이스의 `guard-policy.yaml`(또는 내장 기본 정책)에 `mode: "REVIEW"` 같은 커스텀 모드 규칙이 정의되어 있을 때:
  * `guard review` (인자 없음): `.agents/current_mode`를 `REVIEW`로 전환하고 종료 코드 `0`을 반환한다.
  * `guard review <cmd...>`: `REVIEW` 모드로 런처 세션을 시작하고 `<cmd...>`를 실행한다.
  * `guard handoff send --to review -m "..."`: `To.Mode = "review"`인 핸드오프 티켓을 큐에 등록한다.
  * 정책에 정의되지 않았고 `exec.LookPath`로도 찾을 수 없는 이름(`guard unknowncmd`)은 기존과 동일하게 종료 코드 `1`을 반환한다.
