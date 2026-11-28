# Contract 02: TUI 핸드오프 모달 및 모드 순환 UX (`internal/adapters/tui`)

## 1. 패키지 및 진입점 (`guard/internal/adapters/tui`)

```go
func NewModel(sessMgr *session.Manager) Model
func NewModelWithHandoff(sessMgr *session.Manager, handoffMgr *handoff.Manager, workspaceRoot string) Model
func (m Model) Init() tea.Cmd
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd)
func (m Model) View() string
```

---

## 2. 핸드오프 요약 모달 (`handoffViewSummary`) 계약

### 표시 규격 (`View()`)
* `pending` 상태의 핸드오프 티켓이 활성화되면 요약 모달이 렌더링된다.
* **Summary**: 모달 폭에 맞게 자동 줄바꿈된다.
* **Payload**: `strings.TrimSpace(item.Payload) != ""`이면 상세 내용(`Details` / `상세 내용`) 섹션이 모달 본문에 표시된다.
  * 줄바꿈 후 **12줄을 초과**하면 처음 12줄만 표시하고 `... (+N lines)` 형식의 생략 표시를 출력한다.
* **Footer 키 안내**: `[Enter]`(승인), `[V]`(참조 문서 보기), `[X/C]`(반려), `[Esc]`(닫기/보류)가 표시된다.

### 키 입력 규격 (`Update(tea.KeyMsg)`)
| 키 | 동작 | 기대 결과 |
| --- | --- | --- |
| `enter` | 승인 및 전달 | `ApproveAndRoute` 실행 후 모달 닫힘, 토스트 메시지 표시 |
| `x`, `X`, `c`, `C` | 반려(Reject) | `UpdateStatus(..., StatusRejected, "rejected via TUI")` 실행 후 모달 닫힘 |
| `esc` | 닫기 / 보류(Dismiss) | 티켓 상태를 변경하지 않고(`StatusPending` 유지) 모달만 닫음. 해당 티켓 ID는 보류 목록에 기록되어 이후 폴링(`tickMsg`)에서 자동으로 다시 팝업되지 않음 |
| `v`, `V` | 참조 문서 열기 | `References`가 비어 있으면 에러 토스트 표시, 있으면 첫 번째 문서(`References[0]`)를 뷰어로 엶 |

---

## 3. 다중 참조 문서 뷰어 (`handoffViewRefViewer`) 계약

### 표시 규격 (`View()`)
* `len(item.References) > 1`일 때 뷰어 헤더에 `[1/N]` 형태로 현재 문서 인덱스와 전체 개수가 표시된다.
* Footer에 `[Tab / ←/→]` 문서 전환 키 안내가 표시된다.

### 키 입력 규격 (`Update(tea.KeyMsg)`)
| 키 | 동작 |
| --- | --- |
| `tab`, `right`, `l` | 다음 참조 문서로 순환 전환 (`(idx + 1) % len(References)`) |
| `shift+tab`, `left`, `h` | 이전 참조 문서로 순환 전환 (`(idx - 1 + len) % len`) |
| `esc`, `v`, `V` | 핸드오프 요약 모달(`handoffViewSummary`)로 복귀 |
| `up`, `down`, `j`, `k`, `pgup`, `pgdown` | 뷰포트 스크롤 |

---

## 4. 세션 모드 순환 전환 (`m` / `M` 키) 계약

* 일반 대시보드 화면(모달·도움말이 닫혀 있을 때)에서 `m` 또는 `M` 키를 누르면:
  * `workspaceRoot`의 정책(`policy.LoadDetailed(workspaceRoot)`)에서 `core.ExtractModes`로 추출한 모드 목록을 기준으로 현재 선택된 세션의 다음 모드로 전환한다(`sessMgr.UpdateMode`).
  * 선택된 세션이 없으면 에러 토스트를 표시한다.
