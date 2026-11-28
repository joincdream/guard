# [hotl_guard_top_plan.md] `guard top` HOTL UX 개선 작업 계획서

- **작성일**: 2026-11-28
- **목표**: `guard top`을 **HOTL(Human-on-the-Loop)** 관제탑으로 고도화하여, 사람이 여러 AI 세션을 관리할 때 **"상태 확인 → 버튼(단축키) 조작 → 설정 확인"**을 최소한의 인지 부하로 수행할 수 있게 함
- **관련 문서**:
  - [03_tui_control_tower.md](../03_tui_control_tower.md)
  - [agent-orchestrators-comparison.md](../research/agent-orchestrators-comparison.md)
- **주요 대상 코드**:
  - `internal/adapters/tui/` (`model.go`, `update.go`, `view.go`, `handoff_modal.go`, `modal.go`, `styles.go`)
  - `internal/hub/session/` (세션 상태 및 작업 요약 메타데이터)
  - `internal/hub/handoff/` (승인 대기 티켓 및 Scope Lock 조회)
  - `internal/hub/policy/` (세션별 활성 정책 요약 조회)

---

## 1. 현황 및 해결할 인지 부하 문제

1. **세션 맥락 파악 불가**: 세션 테이블(`view.go`)에 `ID`, `MODE`, `WORKSPACE`, `LAST_ACTIVE`, `DENY`만 표시되어, 동일 워크스페이스의 여러 세션 중 어떤 세션이 무슨 작업 중인지 즉시 알 수 없음.
2. **주의 필요 세션 구분 불가**: `DENY` 누적 횟수만 보이고, `handoff approve` 대기 중이거나 연속 차단으로 정체된 세션이 시각적으로 구분되지 않음.
3. **`[ALLOW]` 로그 노이즈 및 Handoff 가시성**: 정상 `[ALLOW]` 로그가 패널을 채워 `[DENY]`를 찾기 어렵고, 대기 중인 Handoff를 보려면 `[H]` 토글을 눌러야 함.
4. **승인 모달의 판단 근거 부족**: `handoff_modal.go`에 `Summary`, `References`, `Targets`, `ParentID`만 표시되어, 승인 전 검증 근거(위반 횟수, `git diff --stat`, 테스트 통과 여부)를 별도 터미널에서 확인해야 함.
5. **설정 확인 뷰 부재**: 선택한 세션에 현재 적용된 정책 파일 경로, 차단/허용 패턴, Scope Lock 대상을 `guard top` 내에서 바로 확인할 수 없음.

---

## 2. 세부 작업 계획 (Tasks)

### Task 1: 세션 식별 및 상태 배지 도입 (`renderSessionsPanel`)
* **작업 내용**
  * 세션 메타데이터 또는 연결된 활성 Handoff 티켓에서 작업 요약(`Summary` / 라벨)을 읽어 세션 테이블에 표시 (`TASK` 컬럼 추가 또는 `WORKSPACE` 축약 후 배치).
  * 세션 상태 판별 로직 추가:
    * `WAIT` (승인 대기 Handoff 존재)
    * `BLOCKED` (최근 이벤트 연속 `deny` 발생)
    * `ACTIVE` (최근 활동 기준 정상 실행 중)
    * `IDLE` (유휴 상태)
  * `WAIT` / `BLOCKED` 세션을 시각적으로 강조하고 목록 상단에 우선 배치하는 정렬 옵션 제공.
* **완료 기준**: 동일 워크스페이스에서 2개 이상의 세션을 띄웠을 때 `guard top` 테이블만 보고 각 세션의 작업 내용과 개입 필요 여부를 즉시 식별 가능.

### Task 2: `[DENY]` 필터 및 Handoff 대기 알림 배너 (`renderEventsPanel` / `renderHeader`)
* **작업 내용**
  * 하단 이벤트 패널에 **`[F]` (`DENY` 전용 필터 토글)** 추가 (`ALL` ↔ `DENY ONLY`).
  * 승인 대기 중인 Handoff 티켓이 1건 이상일 경우, `[H]`로 큐를 열지 않아도 헤더 또는 상단 배너에 `🔔 N건 승인 대기 ([H] 확인)` 형태로 상시 노출.
* **완료 기준**: 수십 건의 `[ALLOW]` 이벤트 중에서도 `[F]` 한 번으로 `[DENY]` 내역만 즉시 조회 가능하며, 대기 중인 Handoff를 놓치지 않음.

### Task 3: Handoff 승인 모달 판단 근거 보강 (`renderHandoffSummaryModal`)
* **작업 내용**
  * 승인 모달(`handoff_modal.go`)에 결정론적 검증 요약 섹션 추가:
    * 발신 세션의 `DENY` 발생 횟수
    * 워크스페이스 `git diff --stat` 요약 (변경 파일 수 / +/- 라인 수, 실패 시 생략)
    * (선택) 최근 검증/테스트 실행 요약 정보
  * 사람이 코드를 일일이 열지 않고도 요약 근거를 확인한 뒤 `[Enter]` 승인 버튼을 누를 수 있도록 레이아웃 정리.
* **완료 기준**: 승인 모달 렌더링 시 Handoff 메타정보와 함께 세션 위반 수 및 변경 파일 요약이 표시됨.

### Task 4: 세션 활성 설정/정책 확인 모달 (`[I]` Policy/Scope Info)
* **작업 내용**
  * `[I]` 단축키 입력 시 현재 선택된 세션의 가드레일 설정 요약 모달을 표시:
    * 로드된 정책 파일 소스 (`policy.LoadDetailed` 결과: 경로 또는 `builtin`)
    * 현재 모드(`PLAN`/`TEST`/`DEV`/`REVIEW`/`ALL`)에서 활성화된 `read`/`write`/`command` 규칙의 `patterns` 및 `exclude_patterns`
    * 현재 세션에 적용 중인 Handoff Scope Lock (`Targets`) 목록
* **완료 기준**: `guard top`을 종료하거나 YAML 파일을 따로 열지 않고도 `[I]` 키로 현재 세션의 적용 설정을 즉시 확인 가능.

### Task 5: 로컬 알림 및 Git 경계 연동 (후속 연동)
* **작업 내용**
  * `WAIT` / `BLOCKED` 상태 전환 시 터미널 벨 또는 OS 로컬 알림(`notify-send`) 옵션 제공 (외부 메신저 연동 제외).
  * 로컬 작업이 원격 저장소로 넘어가는 시점(`git push`)에 미승인 Handoff 또는 미검증 상태를 점검하는 Git Hook(`pre-push`) 연계 설계 및 구현.
* **완료 기준**: 로컬 오케스트레이션 범위 내에서 승인 대기 알림 및 `pre-push` 안전 게이트 동작 확인.

---

## 3. 진행 순서 및 검증 전략

1. **구현 순서**: `Task 1` → `Task 2` → `Task 4` → `Task 3` → `Task 5`
   * TUI 레이아웃과 상태 모델(`Task 1`, `Task 2`)을 먼저 정리한 뒤 모달(`Task 4`, `Task 3`)을 확장하고, 마지막으로 외부 알림/Git Hook(`Task 5`)을 연결합니다.
2. **TDD / Cleanroom 검증**
   * `TEST` 모드에서 `internal/adapters/tui/` 단위·렌더링 테스트(`blackbox_phase4_test.go`, `tui_test.go` 확장) 선작성
   * `DEV` 모드에서 기능 구현 후 `go build ./... && go vet ./...` 확인
   * `TEST` 모드에서 `go test ./...` 전체 회귀 테스트 통과 확인
