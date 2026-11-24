## 🧑‍💻 User · `2026-11-24 16:34:47` {#step-25}

@[chat/guard.md] 를 대화 채널로 열어주세요.

---

## 🤖 Antigravity · `2026-11-24 16:35:09` {#step-32}

[chat/guard.md](file:///Users/stagefive/kms/guard/chat/guard.md)를 현재 세션(`1f83892f-fb6d-40ff-afd6-f7a102698832`)의 대화 채널로 전환했습니다. 이번 턴부터 대화가 자동으로 기록됩니다.

이제 [11_review_mode_design.md](file:///Users/stagefive/kms/guard/doc/11_review_mode_design.md) 딥 다이브를 이어가시겠습니까?

---

## 🧑‍💻 User · `2026-11-24 16:47:51` {#step-33}

현재 guard 프로그램은 DEV, TEST, PLAN, ALL 외에 임의로 모드를 만들 수 있나요 ? 예를 들어 REVIEW 모드를 생성 할수도 있나요 ?

---

## 🤖 Antigravity · `2026-11-24 16:48:25` {#step-53}

**결론부터 말씀드리면 "반만 가능합니다."**  
코어 정책 엔진과 `guard run`은 임의의 커스텀 모드(예: `REVIEW`)를 이미 받아들이도록 열려 있지만, **CLI 단축 명령 · Handoff 라우팅 · TUI**에는 `DEV`, `TEST`, `PLAN`, `ALL` 4개 모드가 하드코딩되어 있습니다.

---

### 1. 현재 코드에서 **이미 동작하는 것** (임의 모드 가능)
- **정책 평가 엔진 ([evaluator.go](file:///Users/stagefive/kms/guard/internal/core/evaluator.go#L15-L18), [matcher.go](file:///Users/stagefive/kms/guard/internal/core/matcher.go#L79-L85))**:
  - `NormalizeMode`는 화이트리스트 검사 없이 대문자 변환만 수행하고, `evaluatePathRules`는 `guard-policy.yaml`의 `rule.mode`와 현재 모드를 단순 문자열 비교(`strings.EqualFold`)합니다.
  - 따라서 `guard-policy.yaml`에 `mode: "REVIEW"` 규칙을 직접 적으면 엔진 레벨에서는 즉시 동작합니다.
- **세션 및 런처 실행 ([main.go](file:///Users/stagefive/kms/guard/cmd/guard/main.go#L77-L84), [manager.go](file:///Users/stagefive/kms/guard/internal/hub/session/manager.go#L396-L402))**:
  - `guard run REVIEW agy`로 임의의 모드 세션을 바로 띄울 수 있으며, 세션 JSON에도 `REVIEW`가 정상 저장됩니다.

---

### 2. 현재 코드에서 **막혀 있는 것** (4개 모드 하드코딩)

| 계층 | 위치 | 현재 제약 사항 |
| :--- | :--- | :--- |
| **Handoff 라우팅 (가장 치명적)** | [handoff/model.go](file:///Users/stagefive/kms/guard/internal/hub/handoff/model.go#L94-L159) | `ValidAgentMode()`가 `plan`, `dev`, `test`, `all`만 허용합니다.<br>1) `--to review`가 모드가 아닌 **세션 ID `"review"`로 오인**되어 라우팅 불가<br>2) `REVIEW` 세션에서 `--to dev --type fix`를 보내려 해도 `ValidateAsSender()`가 **`unsupported sender mode "REVIEW"` 에러로 차단** |
| **CLI 단축 실행** | [cmd/guard/main.go](file:///Users/stagefive/kms/guard/cmd/guard/main.go#L68-L75) | `case "dev", "test", "plan", "all":`만 분기되어 있어 `guard review agy` 불가 (`guard run REVIEW agy`만 가능) |
| **TUI 모드 전환** | [tui/update.go](file:///Users/stagefive/kms/guard/internal/adapters/tui/update.go#L138-L149) | `D`/`T`/`P`/`A` 4개 키만 하드코딩되어 있어 TUI에서 `REVIEW`로 전환 불가 |
| **`guard init` 프리셋** | `internal/hub/setup/templates.go` | 기본 `guard-policy.yaml`과 워크플로우/리뷰 규칙 템플릿에 `REVIEW` 미포함 |

*(참고: [ShouldApplyScopeLock](file:///Users/stagefive/kms/guard/internal/hub/handoff/model.go#L227-L239)은 `dev`/`all`일 때만 `true`를 반환하도록 짜여 있어, `REVIEW`에서는 자동으로 Scope Lock이 꺼집니다.)*

---

### 딥 다이브 관점에서의 아키텍처 선택지
이 지점에서 [11_review_mode_design.md](file:///Users/stagefive/kms/guard/doc/11_review_mode_design.md) 설계를 확장할 때 두 가지 방향을 고민해 볼 수 있습니다:
1. **`REVIEW`를 5번째 빌트인 1급 모드로 추가** (현재 `11_review_mode_design.md` 방향 — `guard review agy`, TUI `R`키, 기본 정책·리뷰 템플릿 제공)
2. **정책(`guard-policy.yaml`) 기반 커스텀 모드 범용 지원 + `REVIEW` 기본 프리셋 제공** (Handoff의 `ValidAgentMode`가 정책 파일에 정의된 모드까지 동적 인식하도록 열어두면 `REVIEW`뿐 아니라 `SEC`, `AUDIT` 같은 팀별 커스텀 모드도 확장 가능)

어느 방향이 현재 생각하시는 `guard`의 철학에 더 가까우신가요?

---

## 🧑‍💻 User · `2026-11-24 16:49:26` {#step-54}

2번이 모범 사례 아닌가요 ?

---

## 🤖 Antigravity · `2026-11-24 16:49:26` {#step-55}

맞습니다. **2번(정책 기반 커스텀 모드 범용 지원 + `REVIEW` 1급 프리셋 제공)**이 아키텍처 관점에서 명확한 모범 사례입니다.

이미 [evaluator.go](file:///Users/stagefive/kms/guard/internal/core/evaluator.go)와 [matcher.go](file:///Users/stagefive/kms/guard/internal/core/matcher.go)가 `guard-policy.yaml` 기반의 데이터 주도(Data-Driven) 엔진으로 설계되어 있기 때문에, 나머지 계층에 남아있는 4개 모드 하드코딩만 걷어내면 **OCP(개방-폐쇄 원칙)**에 딱 맞는 구조가 됩니다.

---

### 2번 구조로 갈 때 반드시 같이 설계해야 할 3가지 핵심 포인트

단순히 `ValidAgentMode` 체크를 풀어버리면 **오타나 모호성으로 인한 보안 허점(Fail-Open)**이 생기므로, 아래 3가지를 함께 묶어 설계하는 것이 정석입니다.

#### 1. 단일 진실 원천(SSOT) 기반의 동적 모드 레지스트리
- **유효 모드 집합** = `빌트인 모드 (PLAN, TEST, DEV, REVIEW, ALL)` ∪ **`guard-policy.yaml`에 선언된 `rules[].mode` 목록**
- **오타로 인한 Fail-Open 방지**: 만약 임의 문자열을 무조건 허용하면 `guard run REVEIW agy`처럼 오타를 냈을 때 매칭되는 룰이 0개가 되어 **아무 가드레일도 작동하지 않는 위험**이 생깁니다. 위 집합에 없는 모드는 즉시 경고/차단해야 합니다.

#### 2. Handoff `--to` 대상의 모호성 해소 (`Mode` vs `SessionID`)
- 현재 [ParseTarget](file:///Users/stagefive/kms/guard/internal/hub/handoff/model.go#L113-L132)은 `ValidAgentMode`에 걸리면 `Mode`, 아니면 `SessionID`로 해석합니다.
- 동적 모드 레지스트리(`Builtin ∪ Policy`)를 `ParseTarget` / `ValidateAsSender` / `ValidateAsTarget`에 주입하면:
  - `guard-policy.yaml`에 `mode: "SEC"`가 정의되어 있을 때 `--to sec` → 자동으로 **모드 라우팅**
  - 정의되지 않은 `a1b2c3d4` → 기존대로 **세션 ID 라우팅**
  - (필요 시 `mode:sec` 같은 명시적 접두사도 선택 지원 가능)

#### 3. 1급 프리셋 UX vs 커스텀 모드 UX의 계층화
| 구분 | 빌트인 1급 모드 (`PLAN`, `TEST`, `DEV`, `REVIEW`, `ALL`) | 사용자 정의 커스텀 모드 (예: `SEC`, `AUDIT`, `DOCS`) |
| :--- | :--- | :--- |
| **실행 CLI** | 단축 명령어 (`guard review agy`) + `guard run` | `guard run SEC agy` |
| **정책 및 템플릿** | `guard init`이 기본 정책 + 전용 규칙 템플릿 자동 생성 | 사용자가 `guard-policy.yaml`에 `mode: "SEC"` 규칙 추가 |
| **Handoff** | `--to review` 등 자동 인식 | 정책에 선언되면 `--to sec` 자동 인식 |
| **TUI (`guard top`)** | 전용 단축키(`P`/`T`/`D`/`R`/`A`) + 전용 배지 색상 | 범용 모드 전환키(예: `m`으로 등록된 모드 순환/선택) + 기본 커스텀 배지 |

---

이 방향(커스텀 모드 범용 확장 + `REVIEW` 1급 프리셋)을 [11_review_mode_design.md](file:///Users/stagefive/kms/guard/doc/11_review_mode_design.md)의 핵심 아키텍처 축으로 채택하고, 이어서 **OCR 리서치에서 도출된 리뷰 프로토콜 심화 포인트**(결정론적 전처리 · 비대칭 2-Pass 반증 필터 · 심각도 라우팅)까지 함께 딥 다이브해 볼까요?

---

