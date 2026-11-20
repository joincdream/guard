# QA Phase 4 블랙박스 테스트 결과 및 보고서

- **작성일**: 2026-11-20
- **작성자**: QA
- **기준 시나리오**: [test-scenario-plan.md](./test-scenario-plan.md) (`Phase 4`)
- **실행 명령**: `go test -v -race -count=1 ./internal/i18n/... ./internal/adapters/tui/...`
- **테스트 환경**: Linux / Go Test (`-race` 포함) / guard `TEST` 모드

---

## 1. 실행 요약

| 단계 | 패키지 | 결과 | 소요 시간 | 비고 |
| :--- | :--- | :---: | :---: | :--- |
| **Phase 4** | `guard/internal/i18n` | **FAIL** | 0.01s | 기존 4건 + `TC-P4-002`~`004` 통과 / **1건 실패 (`DEF-005`)** |
| **Phase 4** | `guard/internal/adapters/tui` | **FAIL** | 0.075s | 기존 1건 + `TC-P4-005`~`007` 통과 / **1건 실패 (`DEF-006`)** |

---

## 2. 발견 이슈 요약

| 결함 ID | 대상 패키지 | 실패 테스트 | 심각도 | 귀책 판정 |
| :--- | :--- | :--- | :---: | :--- |
| **DEF-005** | `guard/internal/i18n` | `TestPhase4_I18N_LocaleDetectionPriorityAndParsing` | Low | **프로덕션 코드 결함 추정** (`DetectLocaleFromLookup` 미지원 언어 폴백 누락) |
| **DEF-006** | `guard/internal/adapters/tui` | `TestPhase4_TUI_NarrowTerminalSingleLineAndNoPIDOrActive` | Low | **QA 테스트 Assertion 범위 과잉** (프로덕션 렌더링은 정상) |

---

## 3. 이슈 상세 내역

### DEF-005: `DetectLocaleFromLookup()`가 미지원 언어(`fr_FR.UTF-8`)를 `"en"`으로 폴백하지 않고 `"fr"`을 그대로 반환함

- **대상 패키지**: `guard/internal/i18n`
- **관련 테스트**: `internal/i18n/blackbox_phase4_test.go` (`L50-L76`, `TC-P4-001`)
- **단독 재현 명령**:
  ```bash
  go test -v -count=1 ./internal/i18n -run TestPhase4_I18N_LocaleDetectionPriorityAndParsing
  ```
- **입력 조건**:
  `GUARD_LANG="fr_FR.UTF-8"`, `LC_ALL="POSIX"`, `LC_MESSAGES="C.UTF-8"`, `LANG="ja_JP.UTF-8"`
- **기대 결과 (Expected Result)**:
  계약서(`doc/spec/03_adapters_cli_contract.md` §4.2: `C, POSIX, 미지원 언어는 "en" 폴백`)에 따라 `"en"` 반환.
- **실제 결과 (Actual Result)**:
  ```text
  blackbox_phase4_test.go:75: DetectLocaleFromLookup() = "fr", want "en"
  ```
- **원인 추정 및 확인 요청**:
  - `SetLang("fr")` 호출 시에는 `"en"`으로 정상 폴백되나(`TestPhase4_I18N_TranslateFallbackAndInit` 통과), `DetectLocaleFromLookup()` 자체는 `ParseLocale()` 결과(`"fr"`)가 지원 언어 목록(`SupportedLangs()`)에 포함되는지 검증하지 않고 그대로 반환하는 것으로 추정됩니다.
  - 또한 상위 환경변수(`GUARD_LANG`)가 미지원 언어(`fr`)이고 하위 환경변수(`LANG`)가 지원 언어(`ko`)인 경우 폴백 동작도 함께 확인 부탁드립니다.

---

### DEF-006: TUI 좁은 화면 테스트의 `Active` 문자열 금지 Assertion이 패널 제목(`Active Sessions`) 및 영문 컬럼 헤더(`Last Active`)와 충돌

- **대상 패키지**: `guard/internal/adapters/tui`
- **관련 테스트**: `internal/adapters/tui/blackbox_phase4_test.go` (`L219-L269`, `TC-P4-008`)
- **단독 재현 명령**:
  ```bash
  go test -v -count=1 ./internal/adapters/tui -run TestPhase4_TUI_NarrowTerminalSingleLineAndNoPIDOrActive
  ```
- **실제 렌더링 출력 확인**:
  - `en`:
    ```text
    │ Active Sessions (1)                                                          │
    │   ID        Mode   Workspace                          Last Active  Deny      │
    │ ● sess-nar  [PLAN] ...narrow/terminal/rendering/guard just now     0         │
    ```
  - `ko`:
    ```text
    │ 활성 세션 목록 (Active Sessions: 1)                                          │
    │   ID        모드   워크스페이스                       최근 활동    차단      │
    │ ● sess-nar  [PLAN] ...narrow/terminal/rendering/guard 방금 전      0회       │
    ```
- **원인 분석 (QA 테스트 Assertion 과잉)**:
  - 프로덕션 TUI의 5컬럼 테이블 구조(`PID`·`상태` 컬럼 제거) 및 `width=80` 1줄 렌더링 불변성은 `en`/`ko` 모두 **정상 동작**하고 있습니다.
  - QA 테스트가 세션 데이터 행이 아닌 **전체 `View()` 문자열**에 대해 `strings.Contains(view, "Active")`를 검사하면서, 정상 UI 문구인 패널 제목(`Active Sessions`)과 영문 헤더(`Last Active`)에 걸려 실패했습니다.
- **QA 후속 조치 (승인 후 진행 예정)**:
  - `TC-P4-008`의 `Active` 미노출 검증을 전체 `View()`가 아닌 **세션 데이터 행(`sess-nar` 행)**에 `Active` 상태 텍스트가 없는지, 그리고 테이블 헤더에 `PID`/`Status`/`상태` 컬럼이 없는지로 정밀화할 예정입니다.

---

## 4. 참고 — 정상 검증 완료된 주요 항목 (Phase 4)
- **`internal/i18n`**: `GUARD_LANG` > `LC_ALL` > `LC_MESSAGES` > `LANG` 우선순위, `ParseLocale` 경계, `SupportedLangs()`, 번역·키 폴백·`Init()` 연동, `RelativeTime` 경계(`zero`/미래/초/분/시간/일) 통과
- **`internal/adapters/tui`**: 빈 목록 전체 키 입력 패닉 방어, 커서 이동·경계 클램핑·대문자 키 모드 전환/리셋 디스크 영속 반영, `Space`/`?`/`Esc` 모달 토글, `r` 새로고침, `q`/`Q`/`Ctrl+C` 종료 명령 반환 통과

---

## 5. 개발자 검토 및 수정 회신 (2026-11-20)

### `DEF-005` — 수정 완료 (`internal/i18n/detect.go`)
- **원인 확인**: QA 판정대로 `DetectLocaleFromLookup()`이 `ParseLocale()` 결과의 지원 언어 여부를 확인하지 않고 미지원 언어(`"fr"`)를 그대로 반환하고 있었습니다.
- **수정 내용**: `DetectLocaleFromLookup()`이 `GUARD_LANG` → `LC_ALL` → `LC_MESSAGES` → `LANG` 순으로 검사하며 **내장 지원 카탈로그에 존재하는 첫 번째 언어**를 반환하고, 모두 미지원·`C`·`POSIX`·빈 값이면 `DefaultLang ("en")`으로 폴백하도록 수정했습니다.
- **상위 미지원 + 하위 지원 폴백 정책**: 상위 환경변수(`GUARD_LANG=fr`)가 미지원이고 하위 환경변수(`LANG=ko`)가 지원 언어이면, 지원되는 하위 언어(`ko`)로 폴백합니다.
- **QA 확인 요청**: `TestPhase4_I18N_LocaleDetectionPriorityAndParsing` (`TC-P4-001`) 재검증을 부탁드립니다.

### `DEF-006` — 프로덕션 정상 확인 (QA 테스트 Assertion 정밀화 동의)
- `en`/`ko` 모두 `width=80` 5컬럼 1줄 렌더링이 정상 동작함을 확인했습니다. 제안해 주신 대로 `TC-P4-008`의 Assertion을 세션 데이터 행 및 테이블 헤더 기준으로 정밀화해 재검증해 주시기 바랍니다.

---

## 6. QA 재검증 및 종료 보고 (2026-11-20)

- **반영 내용**:
  1. `internal/i18n/blackbox_phase4_test.go` (`TC-P4-001`): 상위 미지원(`GUARD_LANG=fr_FR.UTF-8`) + 하위 지원(`LANG=ko_KR.UTF-8`) → `"ko"` 폴백 검증 케이스 추가.
  2. `internal/adapters/tui/blackbox_phase4_test.go` (`TC-P4-008`): 전체 `View()` 문자열이 아닌 테이블 헤더(`PID`/`Status`/`상태` 컬럼 부재) 및 세션 데이터 행(`Active` 상태 텍스트 부재, `PLAN` 모드 동일 행 표시) 기준으로 Assertion 정밀화.
- **재검증 명령**: `go test -v -race -count=1 ./internal/i18n/... ./internal/adapters/tui/...`
- **재검증 결과**: **PASS** (종료 코드 `0`, 데이터 레이스 0건)
  - `guard/internal/i18n`: `ok` (`1.023s`)
  - `guard/internal/adapters/tui`: `ok` (`1.074s`)
- **결함 최종 상태**:
  - **DEF-005**: **Closed** (프로덕션 수정 및 하위 지원 언어 폴백 검증 완료)
  - **DEF-006**: **Closed** (QA 테스트 Assertion 정밀화 검증 완료)
- **Phase 4 최종 상태**: `i18n`, `adapters/tui` 전수 통과 완료
