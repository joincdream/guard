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

## 🧑‍💻 User · `2026-11-24 20:29:47` {#step-4}

로컬 변경 취소 후 pull

---

## 🤖 Antigravity · `2026-11-24 20:31:27` {#step-7}

로컬 변경 사항을 취소하고 `git pull`을 완료했습니다.

**반영 결과 (Fast-forward):**
- `.obsidian/workspace.json`
- `an71_mcp_knowledge_rag_completion_report.pdf`
- [chat/guard.md](file:///mnt/data/mytask/kms/guard/chat/guard.md)
- [doc/10_prompt_guardrail_design.md](file:///mnt/data/mytask/kms/guard/doc/10_prompt_guardrail_design.md)
- [doc/11_review_mode_design.md](file:///mnt/data/mytask/kms/guard/doc/11_review_mode_design.md)
- [doc/research/open-code-review.md](file:///mnt/data/mytask/kms/guard/doc/research/open-code-review.md)
- [tutor/00-live-chat.md](file:///mnt/data/mytask/kms/guard/tutor/00-live-chat.md)

---

## 🧑‍💻 User · `2026-11-25 21:03:10` {#step-0}

@[doc/11_review_mode_design.md] 딥 다이브 하겠습니다.

제 생각에 4개 모드는 디폴트로 하지만, 사용자가 자유롭게 모드를 만들 수 있게 해야 할 것 같은데요. 제 생각 검토해주세요.

---

## 🤖 Antigravity · `2026-11-25 21:03:52` {#step-20}

사용자님의 생각에 **적극 동의하며, Guard의 확장성을 한 단계 끌어올리는 매우 훌륭한 방향**입니다.

코드베이스를 딥 다이브하여 현재 아키텍처가 사용자 정의 모드를 어떻게 수용할 수 있는지, 어디에 병목이 있는지 검토한 결과를 공유해 드립니다.

---

### 1. 현행 아키텍처 분석 (데이터 기반 검토)

놀랍게도 Guard의 핵심 평가 엔진은 이미 **임의의 모드 문자열을 처리할 수 있도록 유연하게 설계**되어 있습니다.

1. **엔진 레벨의 유연성 ([matcher.go](file:///mnt/data/mytask/kms/guard/internal/core/matcher.go#L80-L82))**:
   ```go
   if rule.Mode != "" && !strings.EqualFold(rule.Mode, currentMode) {
       continue
   }
   ```
   정책 평가기(`evaluatePathRules`, `evaluateCommandRules`)는 모드 이름을 하드코딩 검증하지 않고, 인입된 세션의 `currentMode`와 `guard-policy.yaml`의 `rule.Mode`를 단순히 대소문자 무시 비교합니다. 즉, 정책 파일에 `mode: "DOC"`나 `mode: "SECURITY"`를 적으면 **엔진은 지금도 즉시 해당 모드를 인식하고 가드레일을 적용**합니다.

2. **런처의 유연성 ([main.go](file:///mnt/data/mytask/kms/guard/cmd/guard/main.go#L77-L84))**:
   `guard run <any-mode> <cmd...>` 명령어를 통해 이미 임의의 모드 이름으로 세션을 띄울 수 있는 인터페이스가 열려 있습니다.

---

### 2. 현재 사용자 정의 모드를 가로막는 3가지 병목

엔진과 런처는 준비되어 있으나, **핸드오프(Handoff) 및 부가 시스템에 하드코딩된 제약**이 존재합니다:

| 컴포넌트 | 현재 구현 및 병목 지점 | 문제점 |
| :--- | :--- | :--- |
| **핸드오프 라우팅** | [`internal/hub/handoff/model.go`](file:///mnt/data/mytask/kms/guard/internal/hub/handoff/model.go#L94-L102)<br>`ValidAgentMode()`에 `plan, dev, test, all`만 하드코딩 | 사용자가 `guard handoff send --to security` 또는 `doc`으로 보낼 때, 이를 모드가 아닌 세션 ID로 오인식하거나 거부합니다. |
| **Scope Lock 바인딩** | [`internal/hub/handoff/model.go`](file:///mnt/data/mytask/kms/guard/internal/hub/handoff/model.go#L162-L168)<br>`ShouldApplyScopeLock()`이 `mode == "DEV"`만 검사 | 커스텀 모드 중 특정 모드(예: `DOC`, `FIX`)에만 타겟 쓰기 제한(Scope Lock)을 걸고 싶어도 설정할 방법이 없습니다. |
| **정책 및 모드 선언** | [`guard-policy.yaml`](file:///mnt/data/mytask/kms/guard/guard-policy.yaml)<br>모드 목록을 정의하는 메타데이터 섹션 부재 | 사용자가 어떤 커스텀 모드를 쓰고 있는지 TUI, CLI 도움말, 핸드오프 검증기가 동적으로 파악하기 어렵습니다. |

---

### 3. 권장 아키텍처: "4대 기본 모드 + Custom Mode 확장"

사용자님의 방향(`PLAN`, `TEST`, `DEV`, `REVIEW` 4개를 디폴트로 하되 자유 정의 허용)을 반영한 최적의 설계안입니다:

#### A. `guard-policy.yaml`에 모드 메타데이터 섹션 도입 (선택적 확장)
```yaml
version: "1"

# [선택] 프로젝트 맞춤형 모드 정의 (미정의 시 기본 4개 모드 자동 로드)
modes:
  - name: "SECURITY"
    description: "취약점 분석 및 의존성 보안 감사 전용 세션"
    scope_lock: false # handoff targets를 쓰기 제한으로 쓸지 여부
  - name: "DOC"
    description: "스펙 및 기술 문서 작성 전용 세션"
    scope_lock: true  # 타겟 문서 외에는 쓰기 제한

rules:
  # 기본 모드 규칙들...
  - name: "guard-security-read-only"
    mode: "SECURITY"
    type: "write"
    patterns: ["**"]
    message: "SECURITY 모드는 소스코드를 수정할 수 없습니다."
```

#### B. 핸드오프의 동적 모드 유효성 검사
- `ValidAgentMode(m string)`를 고정 상수가 아닌 **"기본 모드(`PLAN`, `DEV`, `TEST`, `REVIEW`, `ALL`) + 정책 파일(`guard-policy.yaml`)에 등록된 커스텀 모드"**를 검사하도록 개선.
- 이를 통해 `guard handoff send --to security -m "보안 감사 요청"`이 자연스럽게 지원됩니다.

#### C. CLI 런처 단축어 디스패치
- 기본 4개 모드: `guard dev`, `guard test`, `guard plan`, `guard review` (기본 내장 Zero-config)
- 커스텀 모드: `guard run <custom_mode> <cmd>` 지원 및, `guard <custom_mode> <cmd>` 실행 시 정책 파일의 `modes`를 확인하여 런처로 자동 라우팅.

---

### 4. 검토 결론

1. **사용자님의 생각은 완벽히 타당하며 권장됩니다.**
   현업에서는 `REVIEW` 외에도 `SECURITY`(보안 감사), `DOC`(문서 전용), `OPS`(인프라/배포) 등 팀 컨벤션에 따른 다양한 격리 세션이 필요합니다.
2. **[11_review_mode_design.md](file:///mnt/data/mytask/kms/guard/doc/11_review_mode_design.md)와의 연계**:
   - `REVIEW` 모드는 4번째 기본 제공 모드로서 **"Read-Only 하이브리드 리뷰"의 표준 레퍼런스 구현체** 역할을 맡습니다.
   - 동시에 이번 작업을 진행하면서 `internal/hub/handoff/model.go` 등의 하드코딩된 모드 검증 로직을 **동적 모드 등록 구조**로 함께 리팩토링하면, `REVIEW` 모드 구현과 사용자 정의 모드 확장이 한 번에 자연스럽게 완성됩니다.

이 방향에 동의하신다면, [doc/11_review_mode_design.md](file:///mnt/data/mytask/kms/guard/doc/11_review_mode_design.md)에 **"사용자 정의 모드(Custom Modes) 확장 아키텍처"** 섹션을 추가하여 설계를 구체화하겠습니다.

---

## 🧑‍💻 User · `2026-11-25 21:49:35` {#step-21}

승인

---

## 🤖 Antigravity · `2026-11-25 21:50:58` {#step-37}

[doc/11_review_mode_design.md](file:///mnt/data/mytask/kms/guard/doc/11_review_mode_design.md)에 사용자 정의 모드(Custom Modes) 확장 설계를 반영 완료했습니다.

### 반영된 핵심 내용 요약

1. **4대 기본 모드 + Custom Mode 공존 모델 ([4.1절](file:///mnt/data/mytask/kms/guard/doc/11_review_mode_design.md#L87-L99))**:
   - `PLAN`, `DEV`, `TEST`, `REVIEW` 4개 모드는 내장 프리셋으로 Zero-Config 제공.
   - 사용자는 프로젝트 필요에 따라 `DOC`, `SECURITY`, `AUDIT`, `OPS` 등 임의의 모드를 자유롭게 정의.

2. **`guard-policy.yaml` 정책 스키마 확장 ([4.2절](file:///mnt/data/mytask/kms/guard/doc/11_review_mode_design.md#L101-L136))**:
   - `modes:` 섹션을 통해 모드 명칭, 설명, Scope Lock(`scope_lock: true/false`) 여부 선언 지원.

3. **동적 Handoff 라우팅 및 선언적 Scope Lock ([4.3절](file:///mnt/data/mytask/kms/guard/doc/11_review_mode_design.md#L138-L146))**:
   - `ValidAgentMode`를 고정 상수에서 정책 파일 연동 동적 검증으로 확장하여 `guard handoff send --to security` 등 커스텀 모드 티켓 발행 지원.
   - `DEV` 모드 외에도 `scope_lock: true`로 설정된 커스텀 모드에 자동으로 쓰기 영역 제한 바인딩.

4. **구현 청사진 및 체크리스트 동기화 ([7절, 8절](file:///mnt/data/mytask/kms/guard/doc/11_review_mode_design.md#L261-L297))**:
   - Core, Session, Handoff, Policy 모듈의 변경 범위 및 Phase 1~3 검증 체크리스트에 커스텀 모드 검증 항목 반영.

---

## 🧑‍💻 User · `2026-11-26 23:20:04` {#step-0}

특정 명령을 수행하지 못하게 할 수 있을까요 ? 예를 들어서 DEV 모드에서 저는 go test 등을 수행하지 못하게 하고 싶습니다. go test 멸영을 수행하면, 이 결과를 보고 AI 해당 테스트를 통과시킬려고 땜빵 코드를 넣는 경우가 생길 수 있어서 입니다.

---

## 🤖 Antigravity · `2026-11-26 23:21:20` {#step-26}

결론부터 말씀드리면 **가능합니다.** 다만 **현재 `guard`의 정책 파일(`guard-policy.yaml`) 설정만으로는 바로 차단되지 않으며**, 아래 두 가지 방법 중 하나로 적용할 수 있습니다.

---

### 1. 현재 `guard` 엔진 동작 확인 (코드 기준)

현재 코드베이스를 확인한 결과, `guard`는 아직 임의 셸 명령어 실행 자체를 막는 정책 타입을 지원하지 않습니다.

- **지원되는 규칙 `type`**: [evaluator.go](file:///mnt/data/mytask/kms/guard/internal/core/evaluator.go#L27-L34) 및 [doc.go](file:///mnt/data/mytask/kms/guard/cmd/guard/doc.go#L13-L17) 기준 현재 `type`은 `"read"`와 `"write"` 두 가지만 존재합니다.
- **셸 명령어 검사 범위**: [command.go](file:///mnt/data/mytask/kms/guard/internal/core/command.go#L115-L155)의 `evaluateCommandRules`는 셸 세그먼트를 파싱해 **파일 열람/수정 우회**(`cat`, `head`, `git show/diff`, 리다이렉션 `>`/`<`, `tee`, `sed -i`, `python -c`/`bash -c` 등)만 차단합니다.
- **특정 명령어 하드코딩 차단 사례**: [command.go](file:///mnt/data/mytask/kms/guard/internal/core/command.go#L39-L88)의 `evaluateProtectedGuardCommand`에서 `guard handoff approve` 실행을 막고 있어, 동일한 구조로 `go test` 같은 명령어 차단을 정책화할 수 있는 파서(`splitCommandSegments`, `tokenizeSegment`, 서브셸 `bash -c` 검사)는 이미 갖춰져 있습니다.
- 또한 현재 [guard-policy.yaml](file:///mnt/data/mytask/kms/guard/guard-policy.yaml#L16)의 DEV 모드 안내 문구에도 *"요구사항 명세서와 'go test' 실행 결과(stdout)만을 참조하여 개발"*이라고 되어 있어, 차단 적용 시 이 문구도 함께 수정하는 것이 좋습니다(에이전트가 차단된 명령을 계속 시도하는 혼선 방지).

---

### 2. 적용 방법 2가지

#### 방법 A (권장): `guard` 정책 엔진에 `type: "command"` 규칙 기능 추가
현재 워크스페이스가 `guard` 프로젝트 자체이므로, 코어 엔진에 명령어 차단 규칙 타입을 추가하면 모드 전환(`DEV` / `TEST` / `PLAN` / `ALL`) 및 `guard top` 모니터링과 완벽히 연동됩니다.

- **정책 예시 (`guard-policy.yaml`)**:
  ```yaml
  - name: "guard-dev-no-test-exec"
    mode: "DEV"
    type: "command"
    patterns:
      - "go test*"
      - "make test*"
    message: "[POLICY VIOLATION] DEV 모드에서는 테스트(go test 등)를 직접 실행할 수 없습니다. 컴파일 검증(go build, go vet)만 수행하고 검증은 TEST 세션으로 핸드오프하십시오."
  ```
- **구현 범위**:
  1. [evaluator.go](file:///mnt/data/mytask/kms/guard/internal/core/evaluator.go#L27-L34) / [doc.go](file:///mnt/data/mytask/kms/guard/cmd/guard/doc.go#L13-L23): `type: "command"` 스키마 추가
  2. [command.go](file:///mnt/data/mytask/kms/guard/internal/core/command.go#L115-L155): 체이닝(`&&`, `;`, `|`), 환경변수 prefix(`env`, `CGO_ENABLED=0 go test`), 서브셸(`bash -c "go test ./..."`)까지 정규화하여 `patterns`와 매칭하는 검사 로직 추가 (`go build`, `go vet`은 기존처럼 허용 유지)
  3. [guard-policy.yaml](file:///mnt/data/mytask/kms/guard/guard-policy.yaml#L3-L16): DEV 모드 규칙 추가 및 기존 안내 메시지 정리

#### 방법 B: `guard` 수정 없이 `.agents/hooks.json`에 별도 `PreToolUse` 스크립트 추가
[hooks.json](file:///mnt/data/mytask/kms/guard/.agents/hooks.json#L2-L14)과 [handler.go](file:///mnt/data/mytask/kms/guard/internal/adapters/hook/handler.go#L111-L117)에서 보듯, Antigravity의 `PreToolUse` 훅은 표준 출력으로 `{"decision":"deny","reason":"..."}`를 반환하면 명령 실행을 즉시 차단합니다. 별도 셸 스크립트를 훅에 등록해 임시로 막을 수도 있지만, 세션별 실시간 모드(`~/.guard/sessions/<id>.json`) 판별이나 `bash -c` 우회 방지를 중복 구현해야 하므로 **방법 A**가 훨씬 깔끔합니다.

---

원하시면 **방법 A**로 `guard`에 `type: "command"` 기능을 추가하고 현재 워크스페이스의 `guard-policy.yaml`에 DEV 모드 `go test` 차단 규칙까지 적용해 드릴까요? (핸드오프/계획 절차를 거치실 경우 `/plan` 모드로 설계부터 진행할 수도 있습니다.)

---

## 🧑‍💻 User · `2026-11-26 23:23:32` {#step-27}

당신 생각에 어느 방법이 더 좋을 것 같나요 ?

---

## 🤖 Antigravity · `2026-11-26 23:23:32` {#step-28}

제 생각에는 **방법 A (`guard` 코어에 `type: "command"` 정책 지원 추가)**가 압도적으로 좋습니다. 현재 코드 구조와 운영 방식을 기준으로 그 이유는 네 가지입니다.

1. **멀티 세션 및 `guard top` 실시간 모드 전환과 완벽히 호환됨**
   - [handler.go](file:///mnt/data/mytask/kms/guard/internal/adapters/hook/handler.go#L122-L143)를 보면 `guard`는 단순히 전역 파일만 보는 것이 아니라, 세션 ID별로 `~/.guard/sessions/<id>.json`의 모드를 조회합니다.
   - 별도 훅 스크립트(방법 B)를 쓰면 **한 터미널에서는 `DEV` 세션, 다른 터미널에서는 `TEST` 세션을 동시에 띄워 운영하는 환경**(`guard handoff` 워크플로우)이나 `guard top`에서 `D`/`T`/`A` 키로 모드를 즉시 전환하는 기능과 어긋나기 쉽습니다.

2. **AI의 우회 실행 패턴을 기존 파서로 즉시 차단 가능**
   - AI는 단순 문자열 매칭으로 `go test`만 막으면 `CGO_ENABLED=0 go test`, `/usr/local/go/bin/go test`, `go build ./... && go test ./...`, `bash -c "go test"` 같은 변형을 시도할 수 있습니다.
   - [command.go](file:///mnt/data/mytask/kms/guard/internal/core/command.go#L43-L86)와 [command.go](file:///mnt/data/mytask/kms/guard/internal/core/command.go#L351-L515)에는 이미 체이닝 분리(`&&`, `;`, `|`), 앞단 환경변수/`env`/`sudo` 제거(`findCommandIndex`), 경로 제거(`path.Base`), 서브셸(`bash -c`) 재귀 검사가 구현되어 있어 이를 재사용하면 우회 방어가 훨씬 견고해집니다.

3. **정책 관리 일원화 및 재사용성**
   - `guard` 자체 기능으로 넣어두면 앞으로 다른 워크스페이스에서도 별도 스크립트 복사 없이 [guard-policy.yaml](file:///mnt/data/mytask/kms/guard/guard-policy.yaml)에 `go test*`, `make test*`, `pytest*`, `npm test*`만 선언해 바로 재사용할 수 있고, 차단 이력도 `guard top`의 `DENY` 카운트 및 이벤트 로그([handler.go](file:///mnt/data/mytask/kms/guard/internal/adapters/hook/handler.go#L90-L109))에 통합 기록됩니다.

4. **`guard handoff` 설계 철학과 정확히 맞물림**
   - 말씀하신 *"테스트 출력(실패 케이스 입력/기댓값)을 보고 하드코딩·땜빵 코드를 넣는 문제"*는 테스트 파일 열람만 막아서는 해결되지 않는 대표적인 **출력 기반 테스트 치팅(Output-driven Overfitting)**입니다.
   - `DEV`에서는 `go build` / `go vet`(컴파일·정적 검증)만 허용하고, 실행 검증은 `guard handoff`를 통해 `TEST` 세션에서만 수행하도록 물리적으로 분리하면 `guard`가 지향하는 클린룸 아키텍처가 훨씬 완성도 높아집니다.

구현 복잡도도 높지 않아서(기존 [command.go](file:///mnt/data/mytask/kms/guard/internal/core/command.go) 파서 위에 50~80줄 내외 추가), 임시방편인 방법 B보다 **방법 A로 정식 기능화하는 것**을 추천합니다.

---

## 🧑‍💻 User · `2026-11-26 23:24:21` {#step-29}

네 command 를 막는 기능을 추가하겠습니다.

---

## 🤖 Antigravity · `2026-11-26 23:28:56` {#step-57}

현재 모드(`ALL`)와 정책 평가기([evaluator.go](file:///mnt/data/mytask/kms/guard/internal/core/evaluator.go), [command.go](file:///mnt/data/mytask/kms/guard/internal/core/command.go), [matcher.go](file:///mnt/data/mytask/kms/guard/internal/core/matcher.go)) 구조를 확인했습니다. 이어서 코어 엔진에 `type: "command"` 차단 규칙 및 우회 방지 로직을 구현하고 정책 파일과 테스트를 업데이트합니다.

---

## 🤖 Antigravity · `2026-11-26 23:32:50` {#step-67}

코어 엔진의 `type: "command"` 평가 로직, 단위 테스트, `guard doc` 명세, 그리고 워크스페이스 [guard-policy.yaml](file:///mnt/data/mytask/kms/guard/guard-policy.yaml) 반영을 완료했습니다. 이제 전체 테스트 실행 및 바이너리 빌드·설치와 DEV 모드 훅 검증을 한 번에 수행합니다.

---

## 🤖 Antigravity · `2026-11-26 23:33:15` {#step-70}

전체 테스트 통과, `~/.local/bin/guard` 설치, 그리고 `DEV` 모드 훅 차단/허용 검증을 완료했습니다.

---

## 🤖 Antigravity · `2026-11-26 23:33:23` {#step-72}

`guard`에 특정 명령어를 모드별로 차단하는 `type: "command"` 기능을 추가하고, 현재 워크스페이스 정책 및 `~/.local/bin/guard` 바이너리까지 반영했습니다.

### 변경 내역

1. **정책 스키마 확장** ([evaluator.go](file:///mnt/data/mytask/kms/guard/internal/core/evaluator.go#L15-L34), [doc.go](file:///mnt/data/mytask/kms/guard/cmd/guard/doc.go#L13-L23))
   - `Rule.Type`에 `"command"`(별칭 `"exec"` 포함) 타입을 추가했습니다.
   - `exclude_patterns`도 동일하게 지원하여 필요 시 특정 하위 플래그(예: `go test -list*`)만 예외 허용할 수 있습니다.

2. **명령어 패턴 매칭 및 우회 차단 구현** ([matcher.go](file:///mnt/data/mytask/kms/guard/internal/core/matcher.go#L183-L298), [command.go](file:///mnt/data/mytask/kms/guard/internal/core/command.go#L115-L594))
   - 단순 명령어(`go test`, `make test`)뿐 아니라 다음과 같은 우회 패턴까지 정규화하여 차단합니다:
     - 체이닝 및 서브셸 그룹: `go build ./... && go test ./...`, `(cd internal/core && go test ./...)`
     - 환경변수·절대경로·래퍼: `CGO_ENABLED=0 /usr/local/go/bin/go test`, `timeout 30s go test`, `env`/`sudo`/`nohup`
     - 서브커맨드 앞단 플래그: `go -C internal/core test`, `make -C . test`
     - 셸·스크립트 실행기 및 명령 치환: `bash -lc "go test"`, `$(go test ./...)`, `python3 -c "subprocess.run(['go', 'test'])"`
   - 반면 `git commit -m "fix go test"`나 `guard handoff send -m "run go test"`처럼 **다른 명령어의 문자열 인자로 언급된 경우**에는 오탐 없이 통과됩니다.

3. **워크스페이스 정책 반영** ([guard-policy.yaml](file:///mnt/data/mytask/kms/guard/guard-policy.yaml#L3-L26))
   - `DEV` 모드에 `guard-test-command-exec` 규칙(`go test*`, `make test*` 차단)을 추가했습니다.
   - 기존 `guard-test-files-read` 위반 메시지에서도 `'go test' 실행 결과를 참조하라`는 안내를 컴파일 검증(`go build`, `go vet`) 안내로 정리했습니다.

4. **단위 테스트 추가 및 설치 검증** ([evaluator_test.go](file:///mnt/data/mytask/kms/guard/internal/core/evaluator_test.go#L518-L715))
   - 전체 패키지 테스트(`go test ./...` — unit + e2e) 통과 확인
   - `make build && make install`로 `~/.local/bin/guard` 갱신 완료
   - `DEV` 모드 훅 시뮬레이션 결과:
     - `go test ./...` → `{"decision":"deny", ...}` 차단 확인
     - `go build ./...` → `{"decision":"allow"}` 허용 확인

---

## 🧑‍💻 User · `2026-11-26 23:34:04` {#step-73}

ai 가 규칙을 만들때 도움을 주기 위한 doc 에도 도움말을 추가했나요.

---

## 🤖 Antigravity · `2026-11-26 23:34:04` {#step-74}

네, AI 스킬([SKILL.md](file:///home/yundream/.gemini/antigravity-cli/skills/guard/SKILL.md#L11-L12))이 정책 작성 시 호출하는 `guard doc` 명령어의 명세([doc.go](file:///mnt/data/mytask/kms/guard/cmd/guard/doc.go#L13-L24)) 중 **`## 2. Rule Schema & Pattern Syntax` 섹션에는 반영되어 있습니다.**

다만 확인해보니 **반영된 부분과 보강하면 더 좋은 부분**이 나뉩니다:

1. **현재 반영된 내용** ([doc.go](file:///mnt/data/mytask/kms/guard/cmd/guard/doc.go#L16-L22))
   - `type` 설명에 `"command" (blocks matching shell commands, e.g. "go test*")` 추가 완료
   - `patterns` 문법에 `Command match : "go test*", "make test*", "pytest" (for type: "command")` 추가 완료

2. **아직 빠져 있는 내용** ([doc.go](file:///mnt/data/mytask/kms/guard/cmd/guard/doc.go#L25-L82))
   - `## 3. Sample guard-policy.yaml (Few-Shot Template)`: `read` / `write` 샘플만 있고 `type: "command"` YAML 샘플 블록은 아직 없습니다.
   - `## 4. Quick Verification`: `read_file` / `write_file` 검증 예시만 있고, `run_command`(`CommandLine`) 차단 검증 `echo '{...}' | AGENT_MODE=DEV guard` 예시는 아직 없습니다.

AI는 스키마 설명보다 **Few-Shot YAML 샘플과 검증 명령어 예시**를 그대로 참고해 생성하는 경향이 강하므로, 원하시면 [doc.go](file:///mnt/data/mytask/kms/guard/cmd/guard/doc.go)의 `## 3` 샘플과 `## 4` 검증 예시까지 마저 보강해 드릴까요?

---

## 🧑‍💻 User · `2026-11-26 23:35:34` {#step-75}

승인. 그런데 현재 애플리케이션은 i18n 이 적용되어 있지 않나요. 모든 문서는 한글과 영어를 포함해야 합니다.

---

## 🤖 Antigravity · `2026-11-26 23:37:04` {#step-107}

`internal/i18n`(`en.json`, `ko.json`) 구조와 한/영 문서([README.md](file:///mnt/data/mytask/kms/guard/README.md), [README.ko.md](file:///mnt/data/mytask/kms/guard/README.ko.md), [01_requirements.md](file:///mnt/data/mytask/kms/guard/doc/01_requirements.md), [02_architecture.md](file:///mnt/data/mytask/kms/guard/doc/02_architecture.md)) 현황을 확인했습니다. 이제 `guard doc` 출력에 한/영 i18n 및 `type: "command"` Few-Shot·검증 예시를 적용하고 모든 한/영 문서를 동기화합니다.

---

## 🤖 Antigravity · `2026-11-26 23:41:21` {#step-121}

`guard doc` 명세([doc.go](file:///mnt/data/mytask/kms/guard/cmd/guard/doc.go))의 한/영 i18n 분기 및 Few-Shot·검증 예시 보강, 로케일 카탈로그(`en.json`, `ko.json`), 그리고 한/영 문서([README.md](file:///mnt/data/mytask/kms/guard/README.md), [README.ko.md](file:///mnt/data/mytask/kms/guard/README.ko.md), 설계 문서) 업데이트를 마쳤습니다. 이제 전체 테스트 실행 및 빌드·설치 후 `GUARD_LANG=en`과 `GUARD_LANG=ko` 출력을 검증합니다.

---

## 🤖 Antigravity · `2026-11-26 23:42:37` {#step-124}

전체 단위·E2E·i18n 정합성 테스트 통과, `~/.local/bin/guard` 재설치, 그리고 `GUARD_LANG=en` / `GUARD_LANG=ko` 출력 검증까지 완료했습니다.

---

## 🤖 Antigravity · `2026-11-26 23:43:24` {#step-126}

`guard doc`에 `internal/i18n` 기반 한/영 분리를 적용하고, 모든 한/영 문서와 로케일 카탈로그에 `type: "command"` 명세·샘플·검증 예시를 반영했습니다.

### 반영 내역

1. **`guard doc` i18n 한/영 분리 및 Few-Shot·검증 예시 보강** ([doc.go](file:///mnt/data/mytask/kms/guard/cmd/guard/doc.go#L7-L197), [main.go](file:///mnt/data/mytask/kms/guard/cmd/guard/main.go#L242-L244), [doc_test.go](file:///mnt/data/mytask/kms/guard/cmd/guard/doc_test.go#L1-L45))
   - 기존에 영문 제목과 한글 메시지가 혼재된 단일 상수였던 `policyDocSpec`을 `i18n.Lang()`(`GUARD_LANG`, `LC_ALL`, `LANG` 연동)에 따라 **완전 영문(`en`)**과 **완전 국문(`ko`)**으로 분기하도록 변경했습니다.
   - 두 언어 모두 `## 3. Sample guard-policy.yaml`에 `type: "command"`(`go test*`, `make test*`) Few-Shot 샘플을 추가하고, `## 4. Quick Verification`에 `run_command`(`CommandLine: "go test ./..."`) 훅 시뮬레이션 명령어를 추가했습니다.

2. **i18n 로케일 카탈로그 동기화** ([en.json](file:///mnt/data/mytask/kms/guard/internal/i18n/locales/en.json#L19), [ko.json](file:///mnt/data/mytask/kms/guard/internal/i18n/locales/ko.json#L19))
   - `ALL` 모드 전환 안내 문구(`cli.switch.desc.all`)에 파일 읽기/쓰기뿐 아니라 명령어 제한 해제도 한/영 대칭으로 명시했습니다 (`TestCatalogKeyAndFormatParity` 통과 확인).

3. **한/영 사용자 문서 및 설계 문서 업데이트**
   - **영문 README**: [README.md](file:///mnt/data/mytask/kms/guard/README.md#L12-L20), [README.md](file:///mnt/data/mytask/kms/guard/README.md#L142-L160) (핵심 기능 및 `guard-policy.yaml` 예시 추가)
   - **국문 README**: [README.ko.md](file:///mnt/data/mytask/kms/guard/README.ko.md#L12-L20), [README.ko.md](file:///mnt/data/mytask/kms/guard/README.ko.md#L142-L160) (핵심 기능 및 `guard-policy.yaml` 예시 추가)
   - **요구사항·아키텍처 명세**: [01_requirements.md](file:///mnt/data/mytask/kms/guard/doc/01_requirements.md#L54-L70) (`FR-2.2`, `FR-2.3`, `FR-3.7`), [02_architecture.md](file:///mnt/data/mytask/kms/guard/doc/02_architecture.md#L85-L86) (`Path Matcher`, `Command Analyzer` 역할 반영)

4. **검증 및 배포**
   - `go test ./...` 전체 통과 및 `make build && make install`로 `~/.local/bin/guard` 재배포 완료
   - `GUARD_LANG=en guard doc`(영문) 및 `GUARD_LANG=ko guard doc`(국문) 출력 정상 동작 확인

---

## 🧑‍💻 User · `2026-11-26 23:49:10` {#step-127}

commit & push 해주세요.

---

## 🤖 Antigravity · `2026-11-27 00:07:57` {#step-133}

`origin/main`으로 커밋 및 푸시를 완료했습니다.

- **커밋**: `9df9a91` — `feat: add command execution guardrail rules and i18n policy docs` (14개 파일, `+1034 / -32`)
- **참고**: 이번 작업과 무관한 기존 로컬 변경 파일([11_review_mode_design.md](file:///mnt/data/mytask/kms/guard/doc/11_review_mode_design.md), [guard.md](file:///mnt/data/mytask/kms/guard/chat/guard.md), [workspace.json](file:///mnt/data/mytask/kms/guard/.obsidian/workspace.json))은 섞이지 않도록 커밋에서 제외했습니다.

---

## 🧑‍💻 User · `2026-11-27 00:08:25` {#step-134}

guard top 에서 이벤트 로그들이 "2개씩 중복되서 출력" 이 되는 것 같습니다. 확인해주세요.

---

## 🤖 Antigravity · `2026-11-27 00:11:06` {#step-166}

확인 결과, `guard top`의 화면 렌더링 버그가 아니라 **실제 세션 파일에 `[ALLOW]` 이벤트가 약 14~16ms 간격으로 2번씩 중복 기록**되고 있었습니다.

---

### 1. 데이터 확인 결과

현재 활성 세션 파일([358ef6ee.json](file:///home/yundream/.guard/sessions/358ef6ee.json#L305-L335), 워크스페이스: `/mnt/data/mytask/stagefive/aicc-architecture`)을 확인하면 다음과 같은 패턴이 나타납니다:

- **`[ALLOW]` 이벤트는 약 14~16ms 차이로 2건씩 기록됨**
  - `00:01:04.972` — `allow` / `read_file` → `.../guard-policy.yaml`
  - `00:01:04.986` — `allow` / `read_file` → `.../guard-policy.yaml` *(+14ms 뒤 동일 기록)*
  - `00:03:05.465` — `allow` / `run_command` → `git status --short && ...`
  - `00:03:05.482` — `allow` / `run_command` → `git status --short && ...` *(+17ms 뒤 동일 기록)*
- **`[DENY]` 이벤트는 1건만 기록됨**
  - `00:01:05.030` — `deny` / `run_command` → `cd src && go test ./internal/auth/...` *(1건만 기록)*

---

### 2. 근본 원인: `.agents/hooks.json` 내 구버전·신버전 `guard` 훅 중복 등록

해당 워크스페이스의 [hooks.json](file:///mnt/data/mytask/stagefive/aicc-architecture/.agents/hooks.json#L2-L55)에 `guard`를 실행하는 `PreToolUse` 설정이 **서로 다른 키 이름으로 2개** 들어 있습니다:

1. **`"guard"`** ([hooks.json](file:///mnt/data/mytask/stagefive/aicc-architecture/.agents/hooks.json#L2-L14)): `matcher: "*"` → `"command": "guard hook"` (`guard init` / 런처 부트스트랩이 추가한 현재 표준 훅)
2. **`"guard-cleanroom-mode"`** ([hooks.json](file:///mnt/data/mytask/stagefive/aicc-architecture/.agents/hooks.json#L15-L55)): `read_file`, `edit_file`, `write_file`, `run_command` → `"command": "guard"` (이전에 등록되어 있던 레거시 훅)

Antigravity는 `hooks.json`의 최상위 키가 다르면 두 `PreToolUse` 훅을 순차적으로 모두 실행합니다.
- 첫 번째 훅이 `deny`를 반환하면 즉시 차단되어 1번만 찍히고,
- 첫 번째 훅이 `allow`를 반환하면 두 번째 훅까지 이어서 실행되어 **`guard`가 총 2번 호출되면서 이벤트 로그가 2개씩 쌓인 것**입니다.

---

### 3. 해결 방법 제안

1. **해당 워크스페이스 즉시 정리**: [hooks.json](file:///mnt/data/mytask/stagefive/aicc-architecture/.agents/hooks.json#L15-L55)에서 레거시 `"guard-cleanroom-mode"` 항목 제거
2. **`guard` 자체 재발 방지 패치** (권장):
   - **`guard init` / `guard doctor --fix` / 런처 부트스트랩 보완**: 기존 `hooks.json` 병합 시 `"guard"` 외의 다른 키(`"guard-cleanroom-mode"` 등)에 중복 등록된 레거시 `guard` `PreToolUse` 훅을 자동 감지·정리하도록 개선
   - **이벤트 중복 기록 방지 (Idempotency)**: `PreToolUse` 페이로드의 `toolCall.id`(또는 짧은 시간 창 내 동일 호출)를 기준으로 [handler.go](file:///mnt/data/mytask/kms/guard/internal/adapters/hook/handler.go#L99-L108) / [manager.go](file:///mnt/data/mytask/kms/guard/internal/hub/session/manager.go#L299-L327)에서 중복 적재를 무시하도록 방어 로직 추가

`aicc-architecture`의 `hooks.json` 정리와 함께 `guard` 재발 방지 패치까지 진행할까요?

---

## 🧑‍💻 User · `2026-11-27 20:57:28` {#step-0}

git pull 해주세요.

---

## 🧑‍💻 User · `2026-11-27 20:57:36` {#step-1}

git pull 해주세요.

---

## 🤖 Antigravity · `2026-11-27 20:57:47` {#step-4}

`git pull`을 완료했습니다. (`main` 브랜치 Fast-forward: `9df9a91..5d6aba6`)

### 변경된 파일 (9개, +251 / -18)
- [handler.go](file:///home/yundream/mytask/kms/guard/internal/adapters/hook/handler.go)
- [payload.go](file:///home/yundream/mytask/kms/guard/internal/adapters/hook/payload.go)
- [manager.go](file:///home/yundream/mytask/kms/guard/internal/hub/session/manager.go)
- [model.go](file:///home/yundream/mytask/kms/guard/internal/hub/session/model.go)
- [doctor.go](file:///home/yundream/mytask/kms/guard/internal/hub/setup/doctor.go)
- [hooks_merge.go](file:///home/yundream/mytask/kms/guard/internal/hub/setup/hooks_merge.go)
- [en.json](file:///home/yundream/mytask/kms/guard/internal/i18n/locales/en.json)
- [ko.json](file:///home/yundream/mytask/kms/guard/internal/i18n/locales/ko.json)
- [00-live-chat.md](file:///home/yundream/mytask/kms/guard/tutor/00-live-chat.md)

---

## 🧑‍💻 User · `2026-11-28 12:03:33` {#step-0}

DEV 세션을 위한 가드레일 규칙을 만들어주세요. DEV 세션은 테스트 코드에 대한 읽기/쓰기를 할 수 없어야 합니다.

---

## 🤖 Antigravity · `2026-11-28 12:45:54` {#step-12}

[guard-policy.yaml](file:///home/yundream/mytask/kms/guard/guard-policy.yaml)에 DEV 세션의 테스트 코드 쓰기/수정을 차단하는 규칙을 추가하여 읽기·쓰기 모두 차단되도록 구성했습니다.

### 변경 사항
- **기존 유지**: `guard-test-files-read` (`mode: "DEV"`, `type: "read"`) — 테스트 파일(`*_test.go`, `*.test.ts`, `*.test.js`, `*.spec.ts`, `*.spec.js`, `**/testutil/**`, `**/tests/**`, `**/000/**`) 열람 차단
- **신규 추가**: `guard-test-files-write` (`mode: "DEV"`, `type: "write"`) ([guard-policy.yaml](file:///home/yundream/mytask/kms/guard/guard-policy.yaml#L18-L31)) — 동일 패턴의 테스트 파일 생성·수정 차단

### 검증 결과
- `./guard status`: `/home/yundream/mytask/kms/guard/guard-policy.yaml` (규칙 6개) 정상 로드 확인
- `AGENT_MODE=DEV ./guard hook` 시뮬레이션:
  - `read_file` (`internal/core/evaluator_test.go`) → `deny`
  - `write_file` (`internal/core/evaluator_test.go`) → `deny`
  - `edit_file` (`test/e2e/handoff_cli_test.go`) → `deny`
  - `read_file` / `write_file` (`internal/core/evaluator.go`) → `allow`

---

## 🧑‍💻 User · `2026-11-28 12:54:00` {#step-0}

기본 모드 외에 모드를 자유롭게 추가하는게 어렵지 않을 것 같은데 확인해 주세요. 사실상 @[guard-policy.yaml] 만 업데이트 하면 되는 거 아닌가요 ?

---

## 🧑‍💻 User · `2026-11-28 17:11:32` {#step-17}

기본 모드 외에 모드를 자유롭게 추가하는게 어렵지 않을 것 같은데 확인해 주세요. 사실상 @guard-policy.yaml 만 업데이트 하면 되는 거 아닌가요 ?

---

## 🤖 Antigravity · `2026-11-28 17:11:59` {#step-33}

결론부터 말씀드리면, **단순 가드레일(파일·명령어 차단) 용도로만 쓸 때는 [guard-policy.yaml](file:///mnt/data/mytask/kms/guard/guard-policy.yaml)만 업데이트해도 동작**합니다.  
다만 **`guard <모드>` 단축 실행, `handoff` 연동, TUI 모드 전환**까지 자연스럽게 쓰려면 몇 군데 하드코딩된 부분을 함께 열어야 합니다.

---

### 1. `guard-policy.yaml`만 수정해도 바로 동작하는 부분

* **정책 엔진 (`internal/core`)**
  * [evaluator.go](file:///mnt/data/mytask/kms/guard/internal/core/evaluator.go#L23-L25)의 `NormalizeMode`는 허용 모드 목록을 검사하지 않고 공백 제거 + 대문자 변환만 수행합니다.
  * [matcher.go](file:///mnt/data/mytask/kms/guard/internal/core/matcher.go#L83)는 `strings.EqualFold(rule.Mode, currentMode)`로 문자열 비교만 하므로, `guard-policy.yaml`에 `mode: "REVIEW"`처럼 새 이름을 넣으면 즉시 매칭됩니다.
* **모드 전달 및 실행**
  * `guard run REVIEW <cmd...>` ([main.go](file:///mnt/data/mytask/kms/guard/cmd/guard/main.go#L77-L83))
  * 환경변수 `AGENT_MODE=REVIEW` 또는 `.agents/current_mode` 파일 ([loader.go](file:///mnt/data/mytask/kms/guard/internal/hub/policy/loader.go#L101-L116))
  * 런처 배너([banner.go](file:///mnt/data/mytask/kms/guard/internal/adapters/launcher/banner.go#L58-L62))와 TUI 모드 배지([styles.go](file:///mnt/data/mytask/kms/guard/internal/adapters/tui/styles.go#L114-L120))는 `default:` 분기가 구현되어 있어 커스텀 모드명도 그대로 표시됩니다.

---

### 2. 코드 수정이 필요한 제약 사항 (기본 4개 모드 하드코딩 지점)

1. **CLI 단축 명령 (`guard <모드>`, `guard <모드> <cmd...>`)**
   * [main.go](file:///mnt/data/mytask/kms/guard/cmd/guard/main.go#L68-L75)에 `case "dev", "test", "plan", "all":`가 고정되어 있습니다.
   * 커스텀 모드는 `guard review`나 `guard review agy`로 실행할 수 없고, `guard run review agy`로만 실행해야 합니다.
   * `switchMode`([main.go](file:///mnt/data/mytask/kms/guard/cmd/guard/main.go#L169-L178))의 모드별 안내 문구도 4개 모드만 분기되어 있습니다.

2. **핸드오프 (`guard handoff`) — 가장 큰 제약**
   * [model.go](file:///mnt/data/mytask/kms/guard/internal/hub/handoff/model.go#L95-L102)의 `ValidAgentMode`가 `plan`, `dev`, `test`, `all`만 `true`를 반환합니다.
   * 따라서 커스텀 모드를 대상으로 `guard handoff send --to review ...`를 호출하거나, 커스텀 모드 세션에서 핸드오프를 발신하려 하면 `unsupported target mode` / `unsupported sender mode` 검증 오류가 발생합니다.

3. **TUI 키보드 단축키 전환**
   * [update.go](file:///mnt/data/mytask/kms/guard/internal/adapters/tui/update.go#L138-L149)에서 세션 모드 전환 단축키가 `D`, `T`, `P`, `A` 4개로만 매핑되어 있어, TUI 화면에서 커스텀 모드로 전환하는 기능은 지원되지 않습니다.

---

### 요약
* **임시·테스트용 / `guard run <모드>` + 정책 차단만 필요할 때**: [guard-policy.yaml](file:///mnt/data/mytask/kms/guard/guard-policy.yaml)만 수정하면 됩니다.
* **정식 모드처럼 `guard <모드>`와 `guard handoff`까지 연동하려면**: 최소한 [model.go](file:///mnt/data/mytask/kms/guard/internal/hub/handoff/model.go#L95-L102)(`ValidAgentMode`)와 [main.go](file:///mnt/data/mytask/kms/guard/cmd/guard/main.go#L68-L75) 수정이 필요합니다. (원하시면 범용 커스텀 모드를 허용하도록 확장하는 수정도 진행해 드릴 수 있습니다.)

---

## 🧑‍💻 User · `2026-11-28 17:13:39` {#step-34}

커스텀 모드 기능을 확장하려면 어떤 수정이 필요한가요. 현재 dev, test, plan, all 은 기본 모드로 하고 사용자가 필요하면 생성하도록 하는게 목표입니다.

---

## 🤖 Antigravity · `2026-11-28 17:13:39` {#step-35}

현재 구조(`DEV`, `TEST`, `PLAN`, `ALL` 기본 모드 유지 + 필요 시 사용자가 커스텀 모드 추가)로 확장하려면, **정책 파일(`guard-policy.yaml`)을 "모드 레지스트리"의 단일 진실 공급원(SSOT)으로 삼고** 아래 3~4곳의 하드코딩된 분기를 동적 조회로 바꾸면 됩니다.

---

### 1. 정책에서 사용 가능한 모드 목록 추출 헬퍼 추가 (`internal/core` 또는 `internal/hub/policy`)
현재 [evaluator.go](file:///mnt/data/mytask/kms/guard/internal/core/evaluator.go#L44-L47)의 `Policy` 구조체에는 `Version`과 `Rules`만 있습니다.
* **수정 내용**: 기본 4개 모드(`DEV`, `TEST`, `PLAN`, `ALL`) + `Policy.Rules`에 등장하는 `rule.Mode`들을 중복 제거·대문자 정규화해서 반환하는 헬퍼(예: `KnownModes(p *Policy) map[string]bool`)를 추가합니다.
* **(선택)** `guard-policy.yaml` 최상위에 `modes:` 메타 필드(설명, 바이패스 여부 등)를 선택적으로 둘 수도 있지만, 기존 호환성을 유지하려면 `rules[].mode`만으로도 충분히 자동 인식됩니다.

---

### 2. CLI 라우팅 확장 ([cmd/guard/main.go](file:///mnt/data/mytask/kms/guard/cmd/guard/main.go))
현재 `case "dev", "test", "plan", "all":`([main.go](file:///mnt/data/mytask/kms/guard/cmd/guard/main.go#L68-L75))에 기본 모드만 하드코딩되어 있습니다.
* **수정 내용**:
  1. `default:` 분기([main.go](file:///mnt/data/mytask/kms/guard/cmd/guard/main.go#L125-L136))에서 `exec.LookPath` 전에(또는 함께) 현재 워크스페이스의 정책(`policy.LoadDetailed`)을 확인해 **입력된 `cmd`가 `KnownModes`에 포함되는 커스텀 모드인지 검사**합니다.
     * 해당하면 `dev/test/plan/all`과 동일하게 `len(args) > 1`이면 `l.Run(mode, ...)`, 인자가 없으면 `a.switchMode(mode)`를 호출합니다.
     * 정책에 없는 이름이면 기존처럼 `unknown_subcommand` 에러를 내므로 오타 검증도 유지됩니다.
  2. `switchMode`([main.go](file:///mnt/data/mytask/kms/guard/cmd/guard/main.go#L169-L178))에 `default:` 분기를 추가해 커스텀 모드 전환 시 기본 안내 메시지를 출력하도록 합니다.
  3. (보완) 별도 명령어 `guard mode <name>`을 함께 두면, 정책 작성 전에도 임시 모드로 전환하거나 실행 명령어(`exec.LookPath`)와 이름이 겹치는 커스텀 모드를 명확히 지정할 수 있습니다.

---

### 3. 핸드오프 모드 검증 확장 ([internal/hub/handoff/model.go](file:///mnt/data/mytask/kms/guard/internal/hub/handoff/model.go))
가장 중요한 수정 지점입니다. 현재 `ValidAgentMode`([model.go](file:///mnt/data/mytask/kms/guard/internal/hub/handoff/model.go#L95-L102))가 4개 모드만 허용하고 있고, `ParseTarget`([model.go](file:///mnt/data/mytask/kms/guard/internal/hub/handoff/model.go#L113-L132))은 `ValidAgentMode`가 `false`면 무조건 **세션 ID**로 간주합니다.
* **주의할 점**: `ValidAgentMode`를 무조건 모든 문자열에 대해 `true`로 열면 `guard handoff send --to <sessionId>`를 모드명으로 오인하게 됩니다.
* **수정 내용**:
  * `ParseTarget`, `ValidateAsTarget`, `ValidateAsSender`가 **기본 4개 모드 + 현재 정책의 `KnownModes`**를 기준으로 모드 여부를 판별하도록 변경합니다.
  * (또는 `--to mode:<name>` 같은 명시적 접두사 문법을 추가로 지원해 정책 파일에 없는 모드로도 라우팅할 수 있게 합니다.)

---

### 4. TUI 지원 (선택 사항, [internal/adapters/tui](file:///mnt/data/mytask/kms/guard/internal/adapters/tui))
* **뱃지 표시**: [styles.go](file:///mnt/data/mytask/kms/guard/internal/adapters/tui/styles.go#L114-L120)의 `renderModeBadge`는 이미 `default:` 분기가 있어 커스텀 모드도 `[REVIEW]`처럼 정상 출력됩니다.
* **모드 전환**: [update.go](file:///mnt/data/mytask/kms/guard/internal/adapters/tui/update.go#L138-L149)는 `D/T/P/A` 키만 매핑되어 있으므로, TUI에서도 커스텀 모드로 전환하려면 예컨대 `M` 키로 `KnownModes` 목록을 순환하거나 선택 팝업을 띄우는 기능을 추가하면 됩니다.

---

### 수정하지 않아도 되는 부분
* **코어 판정 엔진**: [evaluator.go](file:///mnt/data/mytask/kms/guard/internal/core/evaluator.go#L23-L25), [matcher.go](file:///mnt/data/mytask/kms/guard/internal/core/matcher.go#L83)는 이미 임의의 모드 문자열을 대소문자 구분 없이 매칭합니다.
* **워크스페이스 모드 해석**: [loader.go](file:///mnt/data/mytask/kms/guard/internal/hub/policy/loader.go#L101-L116)의 `ResolveWorkspaceMode`는 `AGENT_MODE`, `AGENT_ROLE`, `.agents/current_mode` 값을 그대로 정규화해 반환합니다.
* **런처 배너**: [banner.go](file:///mnt/data/mytask/kms/guard/internal/adapters/launcher/banner.go#L58-L62)에 이미 `default:` 분기가 구현되어 있습니다.

---

## 🧑‍💻 User · `2026-11-28 17:14:56` {#step-36}

그냥 전부다 동일하게 시작하게 하면 되는 거 아닌가요 ? 4개는 빌트인 모드로 취급하면 되는 거잖아요.

---

## 🤖 Antigravity · `2026-11-28 17:14:56` {#step-37}

맞습니다. 정확히 그렇게 단순화할 수 있습니다.

구조적으로 빌트인 모드와 커스텀 모드를 나눌 필요 없이, **모든 모드를 동일한 개념으로 취급**하고 `DEV`, `TEST`, `PLAN`, `ALL`만 기본 목록(Built-in)으로 미리 포함해 두면 됩니다.

실제 구현도 아주 간단해집니다:

1. **통합 모드 집합 하나만 정의**
   * `모드 목록 = {DEV, TEST, PLAN, ALL} ∪ {guard-policy.yaml의 rules[].mode}`
   * Core 평가, 런처 실행(`guard <mode> ...`), 모드 전환(`guard <mode>`), 배너/TUI 표시 모두 이 집합 기준으로 **동일한 단일 경로**로 처리하면 됩니다.

2. **유일하게 판별이 필요한 곳 1군데 (`handoff --to`)**
   * `guard handoff send --to <값>`에서 `<값>`이 **모드명**인지 **세션 ID**인지 구분해야 하므로([model.go](file:///mnt/data/mytask/kms/guard/internal/hub/handoff/model.go#L113-L132)), 여기서만 위 통합 모드 목록에 있으면 모드로, 없으면 세션 ID로 해석하면 됩니다.

즉, **`KnownModes(policy)`로 빌트인 4개 + YAML 모드를 하나로 합쳐두고**, CLI 라우터([main.go](file:///mnt/data/mytask/kms/guard/cmd/guard/main.go#L68-L75))와 핸드오프 파서([model.go](file:///mnt/data/mytask/kms/guard/internal/hub/handoff/model.go#L95-L102))가 그 목록만 보도록 바꾸면 빌트인·커스텀 구분 없이 똑같이 동작합니다.

---

## 🧑‍💻 User · `2026-11-28 17:16:01` {#step-38}

그렇게 하면, 4개 모드도 하드코딩 할 필요 없는 거 아닌가요 ? guard 설정파일 자체가 모드 정보를 담고 있는 거니까요.

---

## 🤖 Antigravity · `2026-11-28 17:16:01` {#step-39}

네, 맞습니다. 실제로 그렇게 설계할 수 있고, 현재 코드에도 이미 그 기반이 있습니다.

1. **`DEV`, `PLAN`, `TEST`는 설정에서 자동 추출 가능**
   * 이미 [model.go](file:///mnt/data/mytask/kms/guard/internal/hub/policy/model.go)에 `guard-policy.yaml`이 없을 때 쓰는 내장 기본 정책(built-in policy)이 존재하며, 여기에 `DEV`, `PLAN`, `TEST` 규칙이 정의되어 있습니다.
   * 따라서 정책이 `guard-policy.yaml`에서 로드되든 내장 기본 정책으로 폴백되든, **`Policy.Rules`의 `mode`만 모으면 `DEV`, `PLAN`, `TEST`는 코드에 하드코딩하지 않아도 자동으로 나옵니다.**

2. **유일한 예외: `ALL` 같은 "규칙이 없는 모드"**
   * `ALL`(과 `QA`)은 차단 규칙이 하나도 없는 바이패스 모드([evaluator.go](file:///mnt/data/mytask/kms/guard/internal/core/evaluator.go#L28-L31))라서 `rules:` 목록만 훑어서는 존재 여부를 알 수 없습니다.
   * 만약 `ALL`까지 포함해 코드에서 모드 하드코딩을 완전히 없애고 싶다면:
     * `guard-policy.yaml`에 `modes:` (예: `bypass: true` 옵션 포함) 같은 섹션을 추가해 모드 정의까지 설정 파일로 내리거나,
     * 기존 `guard-policy.yaml` 포맷을 그대로 유지한다면 **"모든 모드는 설정(`rules[].mode`)에서 읽고, `ALL`만 규칙 없는 기본 바이패스 모드로 취급"**하면 최소 변경으로 깔끔하게 달성됩니다.

---

## 🧑‍💻 User · `2026-11-28 17:16:18` {#step-40}

개발 계획 수립해주세요.

---

## 🤖 Antigravity · `2026-11-28 17:17:45` {#step-47}

`guard-policy.yaml`(및 내장 기본 정책)에 정의된 모드를 기준으로 CLI, 핸드오프, TUI가 모두 동일하게 동작하도록 확장하는 개발 계획입니다.

---

## 개발 목표
* `DEV`, `TEST`, `PLAN`, `ALL`을 코드상 특수 분기가 아닌 **정책에서 로드되는 기본 모드**로 통합합니다.
* 사용자가 `guard-policy.yaml`의 `rules[].mode`에 새 모드를 추가하면 별도 빌드나 코드 수정 없이 **CLI 실행(`guard <mode>`), 모드 전환, 핸드오프(`guard handoff`), 가드레일 판정**이 모두 동작하도록 합니다.

---

## 단계별 구현 계획

### Phase 1. 코어 모드 추출 헬퍼 추가 (`internal/core`)
* **대상 파일**: [evaluator.go](file:///mnt/data/mytask/kms/guard/internal/core/evaluator.go)
* **작업 내용**:
  1. `ExtractModes(p *Policy) []string` 및 `HasMode(p *Policy, mode string) bool` 함수를 추가합니다.
     * `p.Rules`의 각 `rule.Mode`를 `NormalizeMode`로 정규화·중복 제거하여 수집합니다.
     * 규칙이 존재하지 않는 바이패스 모드(`ModeAll`)는 기본 포함합니다.
  2. 기존 `ModeDev`, `ModeTest`, `ModePlan`, `ModeAll`, `ModeQA` 상수는 내장 정책(`DefaultPolicy`)과 `IsBypassMode` 호환용으로만 유지합니다.
* **단위 테스트**: 커스텀 모드(`REVIEW`, `SECURITY` 등)가 포함된 `Policy`에서 `ExtractModes`, `HasMode`, `EvaluateRequest` 동작 검증.

---

### Phase 2. 핸드오프 동적 모드 지원 (`internal/hub/handoff`, `cmd/guard`)
* **대상 파일**:
  * [model.go](file:///mnt/data/mytask/kms/guard/internal/hub/handoff/model.go)
  * [handoff.go](file:///mnt/data/mytask/kms/guard/cmd/guard/handoff.go)
* **핵심 주의점**: `ParseTarget`([model.go](file:///mnt/data/mytask/kms/guard/internal/hub/handoff/model.go#L113-L132))은 대상이 **모드명**이 아니면 **세션 ID**로 간주하므로, 무작정 모든 문자열을 모드로 허용하면 세션 ID 라우팅이 깨집니다.
* **작업 내용**:
  1. `ParseTargetWithModes(raw string, knownModes map[string]bool)`(또는 유사 시그니처)를 도입해 `knownModes`에 포함된 이름만 `HandoffNode{Mode: ...}`로, 나머지는 `HandoffNode{SessionID: ...}`로 파싱합니다.
  2. `ValidateAsTarget` / `ValidateAsSender`([model.go](file:///mnt/data/mytask/kms/guard/internal/hub/handoff/model.go#L135-L160))에서 하드코딩된 `ValidAgentMode` 체크 대신 안전한 식별자(`ValidSafeID`) 또는 `knownModes` 기반 검증을 사용하도록 확장합니다.
  3. [handoff.go](file:///mnt/data/mytask/kms/guard/cmd/guard/handoff.go)의 `runHandoffSend` 및 `resolveSenderNode`([handoff.go](file:///mnt/data/mytask/kms/guard/cmd/guard/handoff.go#L322-L343))에서 `policy.LoadDetailed(a.workspaceRoot)`로 추출한 모드 목록을 전달하도록 연결합니다.
* **단위 테스트**: 커스텀 모드(`--to review`)와 세션 ID(`--to sess-abc123`)가 충돌 없이 각각 모드/세션으로 파싱되는지 검증.

---

### Phase 3. CLI 동적 라우팅 (`cmd/guard/main.go`)
* **대상 파일**: [main.go](file:///mnt/data/mytask/kms/guard/cmd/guard/main.go)
* **작업 내용**:
  1. 현재 고정된 `case "dev", "test", "plan", "all":`([main.go](file:///mnt/data/mytask/kms/guard/cmd/guard/main.go#L68-L75)) 대신 `default:` 분기([main.go](file:///mnt/data/mytask/kms/guard/cmd/guard/main.go#L125-L136))에서 `policy.LoadDetailed(a.workspaceRoot)`로 정책을 로드해 `core.HasMode(polRes.Policy, cmd)`를 확인합니다.
     * 등록된 모드이면 `len(args) > 1`일 때 `l.Run(mode, ...)`, 인자가 없으면 `a.switchMode(mode)`를 실행합니다.
     * 등록된 모드가 아니면 기존처럼 `exec.LookPath` → `unknown_subcommand` 순서로 처리하여 오타 명령어를 계속 차단합니다.
  2. `switchMode`([main.go](file:///mnt/data/mytask/kms/guard/cmd/guard/main.go#L169-L178))에 `default:` 분기를 추가해 커스텀 모드 전환 시 일반 안내 메시지를 출력합니다.
  3. *(선택)* 실행 파일 이름과 겹치거나 정책 작성 전 임시 모드를 쓰고 싶을 때를 위해 `guard mode <name> [cmd...]` 명시적 서브커맨드 추가도 고려할 수 있습니다.

---

### Phase 4. TUI 모드 전환 보완 (`internal/adapters/tui`)
* **대상 파일**: [update.go](file:///mnt/data/mytask/kms/guard/internal/adapters/tui/update.go)
* **작업 내용**:
  1. [styles.go](file:///mnt/data/mytask/kms/guard/internal/adapters/tui/styles.go#L114-L120)의 `renderModeBadge`는 이미 `default:`로 커스텀 모드를 `[NAME]` 형태로 표시하므로 수정 불필요.
  2. [update.go](file:///mnt/data/mytask/kms/guard/internal/adapters/tui/update.go#L138-L149)의 `D/T/P/A` 단축키는 유지하되, 정책의 `ExtractModes` 목록을 순환 전환하는 키(예: `m`/`M`)를 추가합니다.

---

## 검증 계획
1. **기존 회귀 테스트**: `go test ./...` 전체 통과 확인.
2. **커스텀 모드 E2E 시나리오**:
   * `guard-policy.yaml`에 `mode: "REVIEW"` 규칙 추가
   * `guard review` (모드 전환) 및 `guard review <cmd>` (런처 실행 + 정책 차단) 동작 확인
   * `guard handoff send --to review -m "..."` → `approve` → `inbox --mode review` 전달 확인
   * 오타 명령(`guard invalidcmd`) 에러 반환 및 세션 ID 대상(`guard handoff send --to <sessionId>`) 정상 파싱 확인

---

이 계획대로 진행을 원하시면 말씀해 주세요. (또는 문서 파일로 `doc/`에 저장할지, 바로 Phase 1부터 구현에 들어갈지 알려주시면 진행하겠습니다.)

---

## 🧑‍💻 User · `2026-11-28 17:49:10` {#step-48}

문서로 저장해주세요.

---

## 🤖 Antigravity · `2026-11-28 17:49:23` {#step-51}

커스텀 모드 확장 개발 계획을 [12_custom_mode_plan.md](file:///mnt/data/mytask/kms/guard/doc/12_custom_mode_plan.md)에 저장했습니다.

---

## 🧑‍💻 User · `2026-11-28 17:49:49` {#step-52}

개발 진행해주세요.

---

## 🤖 Antigravity · `2026-11-28 17:56:55` {#step-103}

[12_custom_mode_plan.md](file:///mnt/data/mytask/kms/guard/doc/12_custom_mode_plan.md)의 Phase 1 ~ Phase 4 프로덕션 코드 구현을 완료했습니다.

### 변경 사항

1. **Phase 1 — 코어 모드 추출 헬퍼 ([evaluator.go](file:///mnt/data/mytask/kms/guard/internal/core/evaluator.go#L33-L70))**
   * `ExtractModes(policy *Policy) []string`: 정책 규칙(`rules[].mode`)에 정의된 모드들과 기본 바이패스 모드(`ALL`)를 정규화·중복 제거하여 반환합니다.
   * `HasMode(policy *Policy, mode string) bool`: 해당 모드가 정책 규칙에 정의되어 있거나 바이패스 모드(`ALL`/`QA`)인지 판별합니다.

2. **Phase 2 — 핸드오프 동적 모드 지원 ([model.go](file:///mnt/data/mytask/kms/guard/internal/hub/handoff/model.go#L104-L216), [queue.go](file:///mnt/data/mytask/kms/guard/internal/hub/handoff/queue.go#L32-L38), [handoff.go](file:///mnt/data/mytask/kms/guard/cmd/guard/handoff.go#L33-L48))**
   * `IsKnownMode`, `ParseTargetWithModes`, `ValidateAsTargetWithModes`, `ValidateAsSenderWithModes`, `ValidateWithModes`를 추가해 정책에 정의된 커스텀 모드도 대상/발신자 모드로 인식하고 세션 ID와 구분해 파싱합니다.
   * `Manager.SetKnownModes` 및 `Enqueue` 검증 연동을 추가했습니다.
   * `cmd/guard/handoff.go`에서 `policy.LoadDetailed`로 추출한 모드 목록을 핸드오프 송신·발신자 해석·큐 매니저에 전달합니다.

3. **Phase 3 — CLI 동적 라우팅 ([main.go](file:///mnt/data/mytask/kms/guard/cmd/guard/main.go#L68-L190))**
   * `default:` 분기에서 `core.HasMode(polRes.Policy, cmd)`를 먼저 확인해, `guard-policy.yaml`에 정의된 커스텀 모드도 `guard <모드>`(모드 전환) 및 `guard <모드> <cmd...>`(런처 실행)로 동일하게 동작하도록 `runModeCommand`로 통합했습니다.
   * `switchMode`에 `default:` 분기를 추가해 커스텀 모드 전환 안내를 출력합니다.

4. **Phase 4 — TUI 모드 순환 전환 ([update.go](file:///mnt/data/mytask/kms/guard/internal/adapters/tui/update.go#L151-L250))**
   * `m`/`M` 키로 정책(`core.ExtractModes`)에 등록된 모드 목록을 순환 전환하는 `cycleSelectedMode`를 추가했습니다.

---

### 검증 결과 및 다음 안내
* `go build ./... && go vet ./...` 실행 결과 오류 없이 통과했습니다.
* 현재 세션이 **DEV 모드**이므로 Guard 정책상 `*_test.go` 파일 작성 및 `go test` 실행은 차단됩니다. 단위·통합 테스트 작성 및 전체 테스트 실행은 **TEST 모드**에서 진행해 주세요.

---

## 🧑‍💻 User · `2026-11-28 18:04:29` {#step-0}

TEST 모드는 문서와 test 코드에만 접근 할 수 있어야 합니다. 목적에 맞게 가드레일 설정이 되어있는지 확인해주세요.

---

## 🤖 Antigravity · `2026-11-28 18:04:31` {#step-3}

현재 세션에서 가드레일 설정 파일([guard-policy.yaml](file:///mnt/data/mytask/kms/guard/guard-policy.yaml))을 직접 열람하려 했으나, pre-tool hook에 의해 접근이 차단되어 설정 파일 내부를 확인할 수 없습니다.

### 확인된 사항
- **차단 메시지**: `[POLICY VIOLATION] TEST 모드에서는 doc/ 디렉토리와 테스트 파일(*_test.go)만 열람할 수 있습니다. 다른 파일에는 접근할 수 없습니다.`
- 현재 실행 중인 TEST 모드에서 `doc/` 디렉토리와 `*_test.go` 외 파일(`guard-policy.yaml` 포함)의 **읽기 접근이 실제로 차단되는 동작**은 확인되었습니다.

### 한계 및 안내
- `guard-policy.yaml` 자체에 대한 열람이 차단되어 있어, 쓰기(`write`/`edit`) 제한, 명령 실행(`run_command`) 제한, 다른 확장자/경로에 대한 세부 규칙 등이 목적에 맞게 설정되어 있는지는 이 세션에서 검증할 수 없습니다.
- 정책 파일 전체를 점검하려면 `guard-policy.yaml` 접근이 허용된 모드(예: 일반/DEV 모드 등)에서 확인해 주시기 바랍니다.

---

## 🧑‍💻 User · `2026-11-28 18:07:37` {#step-0}

TEST 모드는 doc와 test 코드만 접근해야 합니다. 프로덕션 코드에 접근할 수 있으면 안됩니다. 가드레일 설정이 올바르게 됐는지 확인해 주세요.

---

## 🤖 Antigravity · `2026-11-28 18:24:15` {#step-24}

[guard-policy.yaml](file:///mnt/data/mytask/kms/guard/guard-policy.yaml#L53-L72)과 `guard` 평가 엔진([evaluator.go](file:///mnt/data/mytask/kms/guard/internal/core/evaluator.go), [matcher.go](file:///mnt/data/mytask/kms/guard/internal/core/matcher.go), [command.go](file:///mnt/data/mytask/kms/guard/internal/core/command.go))을 확인한 결과, **기본적인 파일 읽기/쓰기 차단은 설정되어 있으나 몇 가지 허점과 누락된 예외가 있습니다.**

---

### 1. 현재 정상 동작하는 부분
[guard-policy.yaml](file:///mnt/data/mytask/kms/guard/guard-policy.yaml#L54-L72)의 `guard-test-mode-read`, `guard-test-mode-write` 규칙은 다음과 같이 동작합니다.
- `patterns: ["**"]`로 모든 경로를 차단 대상으로 잡고, `exclude_patterns`로 `doc/**`와 `*_test.go`만 예외 허용합니다.
- `read_file`, `write_file`, `edit_file` 호출 시 `cmd/guard/*.go`, `internal/**/*.go`(테스트 파일 제외), `go.mod`, `Makefile` 등 프로덕션 코드와 일반 파일의 읽기/쓰기가 차단됩니다.
- `run_command`에서도 `cat`/`head`/`tail` 등 `fileViewers`, 입력 리다이렉션(`<`), `tee`, `sed -i`, 인라인 스크립트(`python -c`, `bash -c` 등)에 프로덕션 코드 경로가 인자로 포함되면 차단됩니다([command.go](file:///mnt/data/mytask/kms/guard/internal/core/command.go#L693-L806)).

---

### 2. 확인된 문제점 및 보완 필요 사항

1. **셸 명령어 기반 프로덕션 코드 열람 우회 가능성 (`type: "command"` 규칙 부재)**
   - [command.go](file:///mnt/data/mytask/kms/guard/internal/core/command.go#L693-L755)의 `inspectReadSegment`는 `fileViewers`(`cat`, `head`, `tail` 등), `<`, `git show`/`diff`/`checkout`/`log -p`, 인라인 스크립트(`-c`, `-e`)만 검사합니다.
   - 현재 `TEST` 모드에 `type: "command"` 규칙이 없기 때문에 다음 명령어들은 차단되지 않습니다:
     - `rg`, `grep`, `awk`, `sed`(출력용), `find` 등 검색/출력 명령어로 프로덕션 코드 내용 조회
     - **경로 인자 없는 `git diff` / `git show`** (`args[1:]`를 순회하며 경로 규칙을 검사하므로, 인자 없이 실행하면 프로덕션 코드 변경분이 그대로 출력됨)

2. **`test/` 디렉토리 내 비-`*_test.go` 파일 차단**
   - `exclude_patterns`에 `doc/**`와 `*_test.go`만 있어 `test/e2e/*_test.go`는 허용되지만, [test/handoff-user-test-guide.md](file:///mnt/data/mytask/kms/guard/test/handoff-user-test-guide.md)처럼 `test/` 하위에 있는 테스트 문서·픽스처·헬퍼 파일은 접근이 차단됩니다.
   - 테스트 코드/리소스 전체를 허용하려면 `test/**` 추가를 검토해야 합니다.

3. **현재 워크스페이스 모드 상태 참고**
   - 현재 [.agents/current_mode](file:///mnt/data/mytask/kms/guard/.agents/current_mode#L1)는 `ALL`(바이패스 모드)로 설정되어 있습니다. 실제 `TEST` 가드레일을 적용하려면 `guard test` 또는 `AGENT_MODE=TEST`로 실행해야 합니다.

4. **(참고) `doc/**` 매칭 방식 및 내장 기본 정책**
   - [matcher.go](file:///mnt/data/mytask/kms/guard/internal/core/matcher.go#L62-L70)는 `doc/**`를 경로 중간 포함(`/<prefix>/`)으로도 매칭하므로, 향후 프로덕션 코드 하위 디렉토리 이름에 `doc`이 들어가면 예외로 허용될 수 있습니다.
   - [model.go](file:///mnt/data/mytask/kms/guard/internal/hub/policy/model.go#L48-L65)의 `DefaultPolicy()`에는 `src/internal/**`, `src/cmd/**` 등 과거 경로가 남아 있으나, 현재 루트에 [guard-policy.yaml](file:///mnt/data/mytask/kms/guard/guard-policy.yaml)이 존재하므로 [loader.go](file:///mnt/data/mytask/kms/guard/internal/hub/policy/loader.go#L59-L64) 우선순위에 따라 루트 YAML 파일이 사용됩니다.

---

원하시면 `guard-policy.yaml`의 `TEST` 모드에 `test/**` 예외 추가 및 `rg`/`grep`/`git diff` 등 셸 우회 차단용 `command` 규칙을 보완해 드릴 수 있습니다.

---

## 🧑‍💻 User · `2026-11-28 18:24:50` {#step-25}

승인

---

## 🤖 Antigravity · `2026-11-28 18:31:07` {#step-43}

[guard-policy.yaml](file:///mnt/data/mytask/kms/guard/guard-policy.yaml#L53-L88)의 `TEST` 모드 가드레일 설정을 보완했습니다.

### 변경 내용
1. **허용 예외에 `test/**` 추가** ([guard-policy.yaml](file:///mnt/data/mytask/kms/guard/guard-policy.yaml#L59-L73))
   - `guard-test-mode-read`, `guard-test-mode-write`의 `exclude_patterns`에 `test/**`를 추가하여 `doc/**`, `test/**`, `*_test.go`만 읽기/쓰기가 허용되도록 수정했습니다.
2. **셸 명령어 열람 우회 차단 규칙 추가** ([guard-policy.yaml](file:///mnt/data/mytask/kms/guard/guard-policy.yaml#L76-L88))
   - `guard-test-mode-command` (`mode: "TEST"`, `type: "command"`)를 추가하여 인자 없는 `git diff`·`git show`, `git log -p`/`--patch`/`-u`, `rg`, `grep`, `awk`, `sed` 실행을 차단했습니다.

### 검증 결과 (`GUARD_SESSION_ID="" AGENT_MODE=TEST ./guard hook`)
- `read_file` (`internal/core/evaluator.go`): `deny`
- `write_file` (`internal/core/evaluator.go`): `deny`
- `read_file` (`doc/01_requirements.md`, `test/handoff-user-test-guide.md`, `internal/core/evaluator_test.go`): `allow`
- `write_file` (`test/e2e/sample_helper.go`): `allow`
- `run_command` (`git diff`, `rg EvaluateRequest internal/`): `deny`
- `run_command` (`go test ./...`): `allow`

---

## 🧑‍💻 User · `2026-11-28 18:32:38` {#step-44}

향후 기능 개선을 위한 조사를 진행하겠습니다. 시장에는 amux, Conductor, Emdash 등의 툴이 있는 것으로 알고 있습니다. 확인해주세요.

---

## 🤖 Antigravity · `2026-11-28 18:32:57` {#step-49}

시장의 주요 AI 코딩 에이전트 오케스트레이션 툴인 **amux**, **Conductor**, **Emdash**를 조사한 결과입니다. 세 툴 모두 **"여러 AI 코딩 에이전트(Claude Code, Codex, Gemini CLI 등)를 병렬로 실행·관리하는 컨트롤 플레인"**이라는 공통 목표를 가지지만, 구현 형태와 초점에 차이가 있습니다.

---

### 1. 툴별 핵심 특징

#### ① amux (Agent Multiplexer)
동일 이름으로 성격이 다른 3가지 주요 오픈소스 프로젝트가 있습니다.
* **`amuxhq/amux`** (가장 대표적인 웹/모바일 컨트롤 플레인)
  * **구조**: Rust + SQLite 기반 셀프호스팅 컨트롤 플레인 + 웹 대시보드 + macOS/iOS 컴패니언 앱
  * **특징**: Claude Code, Codex, Gemini CLI를 병렬 실행하며 **공유 태스크 보드**, **사용량 한도(Usage-limit) 복구**, Tailscale 등을 통한 **원격 모니터링·제어**를 지원합니다.
* **`elyesghazel/amux`** (Rust TUI 멀티플렉서 + MCP)
  * **구조**: `ratatui` + `portable-pty` 기반 터미널 멀티플렉서
  * **특징**: 각 패인(pane)마다 **MCP 서버**를 붙여 `spawn_agent`, `wait_for_agents`, `send_to_agent` 도구를 제공하고, 사용자 입력/권한이 필요할 때 시각적 알림을 표시합니다.
* **`hewigovens/amux`** (경량 tmux 래퍼)
  * `tmux` 세션 위에서 로컬 에이전트를 `start`, `attach`, `status`로 관리하는 경량 CLI입니다.

#### ② Conductor (Melty Labs)
* **형태**: 네이티브 **macOS 데스크톱 앱** (상용/클로즈드)
* **핵심 아키텍처**: **Git Worktree 격리**가 기본 단위입니다. 작업(프롬프트)마다 독립된 worktree, 브랜치, 터미널, dev 서버, 리뷰 흐름을 자동 생성해 디스크 충돌 없이 병렬 개발을 수행합니다.
* **주요 기능**:
  * Claude Code, Codex, Cursor, OpenCode 등 기존 에이전트 래핑(Multi-harness)
  * **Diff 중심 코드 리뷰 UI** (계획 검토, 변경분 확인, PR/머지 워크플로우에 최적화)
  * GitHub / Linear 이슈 트래커 연동

#### ③ Emdash (Agentic Development Environment, ADE)
* **형태**: 오픈소스 **로컬 퍼스트 Electron 데스크톱 앱** (로컬 SQLite DB)
* **핵심 아키텍처**: Conductor와 마찬가지로 **작업별 독립 Git Worktree + 전용 브랜치**를 자동 프로비저닝합니다.
* **주요 기능**:
  * **25개 이상의 CLI 에이전트 지원** (Claude Code, Codex, Copilot, Cursor, OpenCode, Qwen Code 등) 및 **동일 태스크 멀티 모델 비교 실행**
  * **이슈 트래커 광범위 연동**: Linear, GitHub, Jira, GitLab, Asana, YouTrack 티켓을 에이전트 세션으로 바로 전달하고 Diff/CI 확인 후 머지
  * **원격 SSH/SFTP 지원**: 로컬 머신뿐 아니라 원격 서버·클라우드 Dev Box의 에이전트 워크로드도 오케스트레이션

---

### 2. 한눈에 보는 비교

| 구분 | amux (`amuxhq` / `elyesghazel`) | Conductor | Emdash |
| :--- | :--- | :--- | :--- |
| **플랫폼/형태** | CLI / TUI / 웹 + 모바일 | macOS 네이티브 앱 | 크로스플랫폼 데스크톱 (Electron) |
| **오픈소스 여부** | 오픈소스 | 클로즈드 (Melty Labs) | 오픈소스 (로컬 퍼스트) |
| **격리 방식** | 세션 / PTY 패인 / 태스크 보드 | Git Worktree 자동 생성 | Git Worktree 자동 생성 |
| **핵심 강점** | 원격 제어, 사용량 복구, MCP 기반 서브에이전트 | Diff-first 리뷰 UX, macOS 완성도 | 25+ 에이전트 지원, Jira/Linear 등 연동, SSH 원격 실행 |

---

### 3. `guard` 향후 기능 개선을 위한 시사점

현재 시장의 툴들은 주로 **"병렬 실행(Worktree) + 이슈 연동 + Diff 리뷰 UX"**에 집중하고 있으며, `guard`가 가진 **모드별 Cleanroom 격리(`PLAN`/`TEST`/`DEV`/`REVIEW`) 및 훅 기반 정책 강제** 영역은 이 툴들이 직접 다루지 않는 차별화 포인트입니다.

향후 `guard` 개선 시 다음 기능들을 참고할 수 있습니다.
1. **세션별 Git Worktree 자동 바인딩 (`Conductor` / `Emdash`)**: `guard dev`·`guard test` 세션 실행 시 독립 worktree를 자동 생성/매핑하면 동일 저장소 내 병렬 핸드오프 시 파일 충돌을 원천 방지할 수 있습니다.
2. **외부 이슈 트래커 → `guard handoff` 연동 (`Emdash` / `Conductor`)**: GitHub Issue나 Linear 티켓을 바로 `guard handoff send` 티켓(`--ref`, `--targets`)으로 변환하는 어댑터.
3. **권한/차단 알림 및 원격 모니터링 (`amux`)**: `guard top` TUI 외에, 에이전트가 정책 위반(`deny`)이나 사용자 승인(`handoff approve`) 대기에 걸렸을 때 데스크톱/웹/모바일 알림을 보내는 기능.
4. **멀티 에이전트 하네스 표준화 (`Emdash`)**: Claude Code, Codex, Gemini CLI 등 여러 에이전트 런처의 훅 포맷·세션 ID 추적을 플러그인/어댑터 계층으로 더 넓게 지원.

---

## 🧑‍💻 User · `2026-11-28 18:36:02` {#step-50}

@[doc] 디렉토리에 조사 내용을 기반으로 guard 와 다른 유사 솔루션간 목표, 기능, 장점/단점 등을 검토하고 향후의 방향을 제안해주세요.

---

## 🤖 Antigravity · `2026-11-28 18:36:53` {#step-58}

[doc/research/agent-orchestrators-comparison.md](file:///mnt/data/mytask/kms/guard/doc/research/agent-orchestrators-comparison.md) 문서를 작성했습니다.

### 문서 주요 구성
1. **개요 및 목표 비교**: 병렬 실행·UX 중심(`amux`, `Conductor`, `Emdash`) vs Cleanroom 무결성·결정론적 정책 집행 중심(`guard`)
2. **기능 비교 매트릭스**: 실행 형태, 에이전트 지원, 격리 방식(Worktree vs `PreToolUse` 훅), 치팅 방어, 핸드오프/HITL, 관찰성, 원격/이슈 연동 비교
3. **솔루션별 장단점 상세 검토**: `guard`의 강점(Cleanroom TDD 격리, 자체 승인 차단, 데몬리스 경량성)과 한계(Worktree 격리 부재, Diff 리뷰 UX 부족, 다중 에이전트/외부 연동 한계) 분석
4. **`guard` 향후 발전 방향 제안**
   - **단기**: `--worktree` 세션 자동 바인딩, 다중 에이전트 훅 어댑터 확장, 승인 대기/차단 푸시 알림
   - **중기**: `REVIEW`·커스텀 모드 정식 통합, 이슈 트래커 → `guard handoff` 브리지, Worktree 기반 Diff/정적 검증 요약 게이트
   - **장기**: 타 오케스트레이터(`Emdash`, `amux` 등) 위에서도 결합 가능한 **범용 Cleanroom·정책 거버넌스 레이어**로 포지셔닝

---

## 🧑‍💻 User · `2026-11-28 18:38:43` {#step-59}

git worktree 는 어떤 기능인가요.

---

## 🤖 Antigravity · `2026-11-28 18:38:43` {#step-60}

`git worktree`는 **하나의 Git 저장소(`.git`)를 공유하면서, 서로 다른 브랜치를 체크아웃한 여러 개의 독립된 작업 디렉터리를 동시에 띄워둘 수 있는 Git 기본 기능**입니다.

---

### 1. 왜 쓰나요? (기존 방식과의 차이)
보통 작업 중에 다른 브랜치(예: 긴급 버그 수정이나 다른 에이전트 작업)로 넘어가려면 다음 중 하나를 해야 했습니다.
* `git stash` 후 `git checkout`으로 브랜치를 전환하거나 (진행 중인 빌드·에디터 상태가 깨짐)
* `git clone`을 하나 더 받아 별도 폴더에 복제해야 함 (`.git` 히스토리를 중복 저장해 디스크 낭비, 커밋/브랜치 공유 안 됨)

`git worktree`는 **`.git` 객체 데이터베이스(커밋 히스토리, 리모트, 스태시 등)는 하나만 공유**하고, **작업 디렉터리(Working Tree)와 HEAD/스테이징 영역(Index)만 별도 폴더로 분리**해 줍니다.

---

### 2. 핵심 동작 원리
* **메인 워크트리**: 처음 `git clone` 또는 `git init` 한 기본 폴더 (실제 `.git/` 디렉터리가 위치)
* **링크드 워크트리(Linked Worktree)**: `git worktree add`로 추가한 폴더 (내부에 `.git` 디렉터리 대신 메인 저장소의 `.git/worktrees/<이름>`을 가리키는 작은 `.git` 파일만 존재)
* **제약 사항**: 같은 브랜치를 두 개의 worktree에서 동시에 체크아웃할 수는 없습니다(파일/브랜치 충돌 방지). 각 worktree는 서로 다른 브랜치를 가져야 합니다.

---

### 3. 기본 사용법
```bash
# 1. 새 브랜치(feat/test)를 만들면서 상위 폴더(../guard-test)에 worktree 생성
git worktree add ../guard-test -b feat/test

# 2. 현재 연결된 worktree 목록 확인
git worktree list

# 3. 작업 완료 후 worktree 제거 (폴더 삭제 및 연결 정리)
git worktree remove ../guard-test
```

---

### 4. AI 코딩 에이전트 환경에서 주목받는 이유
`Conductor`, `Emdash` 같은 멀티 에이전트 툴이 `git worktree`를 핵심 기반으로 삼는 이유는 다음과 같습니다.
1. **디스크 충돌 원천 차단**: 에이전트 A(DEV)와 에이전트 B(TEST)가 같은 저장소에서 동시에 파일을 수정하고 `go test`·빌드를 돌려도 서로의 파일을 덮어쓰지 않습니다.
2. **즉시 생성/저비용**: 대용량 저장소라도 `git clone` 없이 순식간에 독립 작업 공간을 만들 수 있고, 커밋하면 즉시 메인 저장소와 히스토리가 공유됩니다.
3. **병렬 검증 & 리뷰**: 한쪽 worktree에서는 기능을 구현하고, 다른 쪽 worktree에서는 기존 브랜치로 대조 테스트나 리뷰를 동시에 수행할 수 있습니다.

---

## 🧑‍💻 User · `2026-11-28 18:54:48` {#step-61}

제가 만든 guard 는 개인이 사용하는 툴이라서 굳이 git worktree 를 연결하지는 않았는데요. 팀단위로 작업할 때 필요한 기능일까요.

---

## 🤖 Antigravity · `2026-11-28 18:54:48` {#step-62}

결론부터 말씀드리면, **"사람으로 구성된 팀" 때문이라면 굳이 필수는 아니지만, "한 머신에서 여러 AI 에이전트를 동시에 돌리는 병렬 작업"을 하려 한다면 필요해지는 기능**입니다.

---

### 1. "팀 단위 협업" 관점에서는 왜 필수가 아닐까?
* 사람 팀원들은 각자의 PC(각자의 `git clone`)에서 작업하므로 애초에 디스크가 물리적으로 분리되어 있습니다.
* 팀 단위로 `guard`를 도입할 때 실제로 더 중요한 것은 `git worktree`보다는 다음과 같은 요소들입니다.
  * **팀 표준 정책 공유**: 레포지토리에 `guard-policy.yaml`을 커밋해 모든 팀원이 동일한 Cleanroom/TDD 규칙을 적용받게 하는 것
  * **에이전트 다양성 지원**: 팀원마다 쓰는 툴(`agy`, Claude Code, Codex 등)이 다를 수 있으므로 훅 어댑터 호환성
  * **감사/리뷰 연계**: 누가 어떤 모드에서 `handoff approve`를 했는지, 정책 위반 없이 테스트가 작성됐는지 PR/CI 단계에서 확인하는 흐름

---

### 2. 그렇다면 `guard`에 `git worktree`가 필요한 순간은 언제일까?
개인 사용자든 팀원이든, **한 명의 개발자가 내 PC에서 여러 에이전트 세션을 동시에 병렬 실행할 때**입니다.

* **순차 작업 (`PLAN → TEST → DEV → REVIEW`) 중심이라면 → 없어도 충분 (오히려 현재 구조가 단순)**
  * 현재 `guard`처럼 한 작업 내에서 역할을 바통 터치(`handoff`)하며 순차적으로 진행할 때는, **같은 디렉터리**에 있어야 `TEST`가 작성한 `*_test.go`를 `DEV`가 곧바로 빌드/연동해 구현하기 편합니다. (브랜치 합치는 수고도 없음)
* **동시 병렬 작업 중심이라면 → 매우 유용**
  * 예: 기능 A를 `DEV` 에이전트가 구현하는 동안, 같은 레포지토리에서 기능 B의 `TEST` 에이전트나 `REVIEW` 에이전트를 **동시에** 돌리고 싶을 때.
  * worktree가 없으면 두 에이전트가 같은 파일이나 `git status`/브랜치를 건드려 작업이 엉킵니다.

---

### 3. 정리: 현재 `guard`에 맞는 선택
* 지금처럼 **개인이 한 작업 흐름을 안전하게 격리(`PLAN`/`TEST`/`DEV`/`REVIEW`)하며 진행하는 용도**라면 `git worktree`를 억지로 붙일 필요가 없습니다. (오히려 브랜치 생성·머지·정리라는 복잡도만 늘어납니다.)
* 만약 향후 **"여러 이슈를 동시에 에이전트에게 던져두고 병렬로 돌리는 오케스트레이터"**로 확장하고 싶을 때만 선택 옵션(`--worktree`)으로 붙이는 것이 가장 합리적입니다.

---

## 🧑‍💻 User · `2026-11-28 18:57:35` {#step-63}

잇 점을 아직 잘 모르겠습니다. PLAN -> DEV -> TEST -> REVIEW 는 워크플로우 입니다. 워크플로우는 기본적으로 각 단계가 독립적으로 병렬해서 돌아갈 수 있습니다. 저는 지금도 DEV 하면서, 새로운 기능의 기획이 필요하면 PLAN 에서 작업을 수행합니다. 파일들은 애초에 guard 로 격리되니 충돌할 일도 사실상 없다고 봐야 할 겁니다.

---

## 🤖 Antigravity · `2026-11-28 18:57:35` {#step-64}

말씀하신 관점이 정확합니다. **`guard`처럼 역할(`PLAN` / `TEST` / `DEV`)별로 접근 가능한 파일 집합을 서로 겹치지 않게(Disjoint) 격리해 둔 구조에서는, 역할 간 병렬 작업 시 파일 수정 충돌이 거의 발생하지 않습니다.**

실제로 `PLAN`(문서만 수정)과 `DEV`(프로덕션 코드만 수정)를 동시에 돌리는 경우라면 `guard`의 권한 격리가 이미 논리적인 작업 공간 분리 역할을 해주고 있으므로 `git worktree`의 실익이 크지 않습니다.

---

### 그럼에도 `git worktree`가 의미를 가지는 예외적인 3가지 경우

현재 워크플로우에서 굳이 `git worktree`가 필요해지는 상황은 딱 다음 3가지입니다.

1. **같은 모드를 2개 이상 동시에 돌릴 때 (Multi-Feature 병렬화)**
   * 예: "기능 A를 구현하는 `DEV` 에이전트"와 "기능 B를 구현하는 `DEV` 에이전트"를 동시에 실행할 때.
   * 둘 다 같은 `DEV` 권한을 가지므로 `guard`만으로는 서로의 코드 수정을 막을 수 없습니다.

2. **미완성 코드로 인한 빌드/테스트 간섭**
   * `DEV` 에이전트가 리팩터링 중이라 잠시 컴파일이 깨진 상태일 때, 같은 디렉터리에서 `TEST` 에이전트가 `go test`를 실행하거나 `REVIEW`가 정적 검증을 돌리면 `DEV`의 미완성 변경분 때문에 실패하게 됩니다.
   * Worktree로 브랜치가 분리되어 있으면 서로의 중간 작업 상태(Dirty working tree)에 영향을 받지 않습니다.

3. **작업 단위별 커밋 / PR 분리**
   * `PLAN`에서 작성한 새 기획 문서와 `DEV`에서 수정한 기존 기능 코드를 **서로 다른 브랜치/커밋**으로 깔끔하게 분리해 올리고 싶을 때, 워킹 디렉터리가 하나면 `git add` 시 수동으로 골라 담아야 합니다.

---

### 정리
* **현재 방식 (단일/소수 작업에서 `PLAN`·`DEV`·`TEST` 역할 병렬 실행)**: `guard`의 파일 격리만으로 충분하며, `git worktree`를 도입하면 오히려 브랜치 생성·머지·정리 비용만 늘어납니다.
* **`git worktree`가 유효한 시점**: "역할 간 병렬"을 넘어 **"독립된 브랜치·빌드 상태를 유지한 채 여러 기능(Feature)을 동시에 구현"**할 필요가 생길 때입니다.

따라서 아까 작성한 [agent-orchestrators-comparison.md](file:///mnt/data/mytask/kms/guard/doc/research/agent-orchestrators-comparison.md)의 방향성에서도 `git worktree`는 필수 구조가 아니라, 향후 다중 기능 병렬화가 필요해질 때만 선택적으로 고려하는 옵션으로 보는 것이 맞습니다.

---

## 🧑‍💻 User · `2026-11-28 19:00:05` {#step-65}

그리고 "AI Agent 병렬 수행의 범위에 대해서" 합의가 필요하겠는데요. 예를 들어 PLAN 과 DEV를 동시에 돌린다고 하면, 사람이 병목이 되어서 병렬작업을 수행하는게 어렵습니다. 예를 들어서 PLAN 과 DEV 를 동시에 돌린다고 해보면, 결국 HITL 방식이 되어야 하는데, 여기 저기 번갈아 가면서 병렬로 기획 하고 개발을 하는데 쉽지 않습니다.

---

## 🤖 Antigravity · `2026-11-28 19:00:05` {#step-66}

정확한 지적입니다. **"에이전트가 병렬로 돌 수 있다"는 것과 "사람이 병렬로 조율할 수 있다"는 것은 완전히 다른 문제**입니다.

결국 **HITL(Human-in-the-Loop)이 개입하는 순간, 전체 속도의 상한선은 에이전트의 처리 속도가 아니라 '사람의 컨텍스트 스위칭 한계'**로 결정됩니다.

---

### 1. 병렬 실행이 실패하는 패턴 vs 작동하는 패턴

| 구분 | 동기적 상호작용 병렬 (비효율) | 비동기 파이프라인 병렬 (현실적) |
| :--- | :--- | :--- |
| **예시** | `PLAN`(기획 논의) + `DEV`(구현 지시)를 동시에 번갈아 대화 | 확정된 `DEV`/`TEST` 티켓을 백그라운드 실행에 두고, 사람은 `PLAN` 또는 `REVIEW` 1개만 전경(Foreground)에서 집중 |
| **사람의 역할** | 여러 창을 오가며 실시간 운전 (컨텍스트 스위칭 폭증) | **체크포인트(`handoff approve`, 리뷰)에서만 개입** |
| **인지 부하** | 매우 높음 (사람이 병목이 됨) | 낮음 (한 번에 하나의 의사결정에만 집중) |

즉, 현실적으로 유효한 병렬 수행의 범위는 **"사람이 동시에 여러 개를 대화하며 조종하는 것"이 아니라, "사람이 하나의 인지 작업(기획 또는 리뷰)을 하는 동안, 이미 명세가 확정된 실행 작업(`TEST`, `DEV`)이 백그라운드에서 자율 주행하는 것"**에 가깝습니다.

---

### 2. 단계별 병렬 적합성 정리

* **병렬에 부적합 (인간 집중 필요 — 직렬/전경 작업)**
  * **`PLAN`**: 요구사항 정의, 트레이드오프 결정 등 사람의 판단이 계속 개입해야 함. 한 번에 하나만 하는 것이 효율적.
  * **`REVIEW`**: 최종 품질과 설계 의도를 확인하는 단계. 역시 사람의 집중력이 필요.
* **병렬(백그라운드 위임)에 적합 (완료 조건이 명확함)**
  * **`TEST`**: 확정된 명세서(`doc/spec/*`)를 보고 테스트 케이스를 작성·실행하는 작업.
  * **`DEV`**: 확정된 명세서와 실패하는 테스트를 통과시킬 때까지 구현·빌드하는 작업.

---

### 3. `guard`의 방향성에 주는 시사점

이 관점에서 보면, 시장의 다른 툴(`Conductor`, `Emdash`)이 **"수십 개 에이전트를 띄워놓고 사람이 바쁘게 리뷰·머지하는 모델"**을 지향할 때, `guard`는 오히려 다음 방향이 훨씬 잘 맞습니다.

1. **동시성보다 "자율 주행의 신뢰성"이 핵심**: 사람이 백그라운드로 던져놓고 신경을 끄려면, 에이전트가 엉뚱한 파일(테스트/프로덕션/승인)을 건드리지 않는다는 **가드레일 신뢰**가 필수입니다. (`guard`의 핵심 강점)
2. **Handoff = 비동기 체크포인트**: `PLAN`에서 확정된 스펙을 `guard handoff send`로 넘기면 `TEST`/`DEV`가 알아서 돌고, 사람은 **모든 게 끝난 뒤 `guard top` 알림이나 `handoff` 인박스에서 `approve` 여부만 판단**하는 구조가 인지 부하를 가장 줄여줍니다.

원하시면 이 **"HITL 병목과 적정 병렬 수행 범위(전경 1개 + 백그라운드 자율 실행)"**에 대한 합의 내용을 앞서 작성한 [agent-orchestrators-comparison.md](file:///mnt/data/mytask/kms/guard/doc/research/agent-orchestrators-comparison.md)에도 반영해 두겠습니다.

---

## 🧑‍💻 User · `2026-11-28 19:01:32` {#step-67}

수십 개 에이전트를 띄워 놓는 것이 유효한 경우에는 "수십 개의 주제에 대한 세부 탐색을 동시에 하는" 경우가 되지 않나요 ?

---

## 🤖 Antigravity · `2026-11-28 19:01:32` {#step-68}

네, 정확합니다. 수십 개의 에이전트를 동시에 띄우는 것이 실제로 높은 효율을 내는 경우는 대부분 **Fan-out / Map-Reduce 성격의 탐색·분석·검증 작업**입니다.

---

### 1. 다수 병렬 에이전트가 유효한 경우 (Read-heavy / Map-Reduce)
중간 과정에서 사람(HITL)의 판단이 필요 없고, **각 에이전트가 독립적으로 탐색한 뒤 요약된 최종 결과만 사람이 한 번에 취합(Reduce)**하는 작업들입니다.

* **코드베이스 다각도 탐색/리뷰**: 수십 개 모듈이나 PR 파일을 쪼개서 동시에 보안 취약점·동시성 버그·성능 이슈를 스캔 (예: [open-code-review.md](file:///mnt/data/mytask/kms/guard/doc/research/open-code-review.md)의 번들별 병렬 SubAgent)
* **기술 리서치 / 대안 비교**: 여러 라이브러리, 아키텍처 옵션, API 스펙을 동시에 조사해 비교표로 취합
* **독립적 재현·테스트**: 서로 간섭 없는 수십 개의 버그 재현 스크립트나 벤치마크를 동시 실행

이 영역에서는 에이전트가 많아질수록 탐색 시간이 선형으로 줄어들며, **사람은 마지막에 취합된 보고서 1개만 읽으면 되므로 인지 병목이 생기지 않습니다.**

---

### 2. 반대로 다수 병렬이 독이 되는 경우 (Write-heavy / HITL)
* **서로 얽히는 코드 수정·설계**: 10개의 에이전트가 각자 브랜치에서 코드를 수정해 PR 10개를 동시에 올리면, 결국 **사람이 10개의 Diff와 설계 의도를 전부 읽고 머지 충돌을 해결**해야 합니다.
* 이때는 에이전트가 10배 빨라져도 **사람의 리뷰·의사결정 처리량이 1배로 고정**되어 있어 전체 흐름이 막힙니다.

---

### 3. 정리
* **수십 개 병렬(Fan-out)**: **탐색·리서치·정적/동적 리뷰**처럼 읽기 중심이고 결과를 하나로 요약할 수 있는 작업에 유효 (이 경우 `guard`의 `read-only` 격리나 서브에이전트 패턴이 잘 맞음)
* **소수 파이프라인(1~2개 전경 + 백그라운드 위임)**: **기획(`PLAN`)·구현(`DEV`)·테스트(`TEST`)**처럼 상태를 바꾸고 사람의 최종 승인이 필요한 작업에 유효

즉, **"탐색은 넓게(다수 병렬), 변경은 좁고 안전하게(격리된 소수 파이프라인)"**로 역할을 나누는 것이 현업에서 가장 현실적인 기준이 됩니다.

---

## 🧑‍💻 User · `2026-11-28 19:05:39` {#step-69}

그리고 코드 베이스 다각도 탐색/리뷰에 대해서도 결국 HITL이 되게 되는데, 동시 탐색이 의미가 있나요 ? 단계적 탐색이 오히려 더 효과적일 거 같은데요.

map reduce 방식으로 동시 탐색이 되려면, reduce 가 가능하도록 "초반에 매우 높은 수준의 기획/설계"가 필요하게 됩니다. 그렇게 한다고 하더라도 기획/설계를 통해서 정보가 갱신되면, 탐색 방식과 주제가 바뀌는 경우가 매우 많습니다. 특수한 경우가 아니면 효과를 보는 경우가 있을까요 ?

저는 대량의 AI Agent 병렬 수행이라는게, 마케팅을 위한 사례이지 일반적인 사례가 되기는 힘들다고 보는 입장입니다. Autonouse AI 조직 환경에서는 의미가 있을 수 있는데, AI Autonomouse 라는 것 자체가 중장기적인 관점에서도 의미가 있을지 모르는 상황인데, 큰 효과가 있을까요.

---

## 🤖 Antigravity · `2026-11-28 19:05:39` {#step-71}

말씀하신 통찰은 실제 소프트웨어 엔지니어링 현장의 본질을 정확히 찌르고 있습니다. **"탐색 자체가 가설과 주제를 바꾼다"**는 점을 고려하면, 일반적인 개발·설계에서 대규모 병렬 탐색은 효율보다 낭비가 큰 경우가 많습니다.

---

### 1. 왜 단계적(직렬) 탐색이 대부분 더 효과적인가?

* **탐색은 정적 집계가 아니라 '가설 수정 루프'입니다**
  * 첫 번째 파일을 읽어보니 애초에 가설이 틀렸거나, 아키텍처의 진짜 원인이 전혀 다른 곳에 있는 경우가 흔합니다.
  * 이때 처음부터 20개 에이전트를 동시에 띄워두면, **나머지 18~19개는 이미 폐기된 가설을 붙잡고 토큰과 시간만 태우는 결과**가 됩니다.
  * 반면 단계적 탐색은 1번 결과를 보고 즉시 2번 질문을 좁혀 들어가므로 훨씬 적은 비용으로 핵심에 도달합니다.
* **Map-Reduce의 전제 조건(고정된 스키마)의 모순**
  * 20개 결과를 깔끔하게 Reduce하려면 초반에 **"무엇을 어떤 기준으로 찾고 어떤 포맷으로 보고할지"** 매우 정교하게 설계해야 합니다.
  * 그런데 그 정도로 문제 구조가 이미 명확히 파악된 상태라면, 사실상 **모호한 '탐색' 단계는 이미 끝난 것**에 가깝습니다.

---

### 2. 그렇다면 다중 병렬이 실제로 효과를 내는 '특수한 조건'은?

대규모 병렬 에이전트가 마케팅 사례를 넘어 실제로 의미가 있는 경우는 **탐색 주제가 중간에 절대 바뀌지 않는, 기준이 고정된 반복 작업**으로 매우 좁혀집니다.

* **고정 체크리스트 기반 대규모 검사**: 이미 규칙(예: NPE, SQLi, 사내 컨벤션)이 확정되어 있고, 변경된 파일 50개에 그 동일한 잣대만 대입해 결함을 뽑아내는 경우 (이것도 사실상 LLM에게 "정적 분석기의 확장판" 역할을 시키는 것)
* **단순 반복 기계적 변환**: 100개의 독립된 컴포넌트/테스트 파일에 동일한 마이그레이션 패턴을 적용하는 경우

그 외에 **"이 문제를 어떻게 풀까?", "이 코드베이스가 왜 이렇게 동작하지?"** 같은 맥락 추론과 설계가 섞인 작업에서는 단계적 실행이 압도적으로 유리합니다.

---

### 3. `guard` 관점에서의 결론

사용자께서 말씀하신 **"중장기적으로도 완전 자율 AI 조직(Autonomous AI)보다 인간이 통제권을 쥔 상태에서 깊고 정확하게 일하는 환경이 현실적"**이라는 관점에 서면, `guard`의 방향성은 더욱 분명해집니다.

* `Conductor`나 `Emdash`처럼 "에이전트를 10개, 20개씩 띄워놓고 병렬로 돌리는 화려한 쇼케이스"를 무리하게 따라갈 필요가 없습니다.
* 오히려 **`guard`는 "소수의 에이전트가 단계적으로 작업할 때, 각 단계가 앞선 단계의 원칙을 훼손하지 않도록 신뢰 경계를 지키고, 사람이 최소한의 피로도로 상태를 확인·승인하게 해주는 도구"**라는 본질에 집중하는 것이 실무 효용성이 훨씬 높습니다.

---

## 🧑‍💻 User · `2026-11-28 19:07:43` {#step-72}

지금 대화 내용을 기반으로 @[doc/research/agent-orchestrators-comparison.md] 문서를 보강해주세요. 4.guard 발전 방향 부분을 보강하면 되지 않을까요.

---

## 🤖 Antigravity · `2026-11-28 19:10:06` {#step-80}

[agent-orchestrators-comparison.md](file:///mnt/data/mytask/kms/guard/doc/research/agent-orchestrators-comparison.md#L67-L111)의 **4장(`guard`의 향후 발전 방향 제안)**을 대화 내용 기반으로 보강했습니다.

### 주요 보강 내용
1. **4.1 병렬 수행 범위와 `git worktree`에 대한 현실적 전제** 신설
   - **HITL 병목 기준**: 동기적 멀티 창 운전이 아닌 **전경 1개(`PLAN`/`REVIEW`) + 백그라운드 자율 실행(`TEST`/`DEV`)** 비동기 파이프라인 모델 정리
   - **대규모 병렬 탐색(Map-Reduce)의 한계**: 가설이 계속 변하는 일반 개발·탐색에서는 **단계적 탐색**이 우위이며, 대규모 병렬은 고정 룰셋/반복 작업에만 한정됨을 명시
   - **`git worktree`의 위치**: `guard`의 Disjoint 파일 격리로 역할 간 병렬은 충돌이 없으므로, `worktree`는 기본 필수가 아닌 동일 모드 다중 실행·빌드 격리 시에만 쓰는 선택 옵션으로 재정의
2. **4.2~4.4 로드맵 및 포지셔닝 조정**
   - 단기 과제의 우선순위를 **다중 에이전트 훅 어댑터**, **비동기 승인/위반 알림**, **선택적 `--worktree`** 순으로 재정렬
   - 장기 전략을 대규모 스웜 경쟁이 아닌 **단계적·고신뢰 AI 개발을 위한 경량 거버넌스 레이어**로 명확화

---

## 🧑‍💻 User · `2026-11-28 19:15:27` {#step-81}

4.2 단기 과제를 보면 Slack/Discord 로 알리는 경량 알리미 추가와 같은 기능이 있습니다. 이것은 guard를 "로컬 AI 개발 환경"으로 하느냐 "실시간 협업 환경"으로  격상시키느냐의 이슈로 보이는데, 기본은 로컬에서의 AI 개발 오케스트레이트가 되어야 하는 거 아닐까요.

현재의 원격 개발 협력 작업은 이미 "비동기적" 입니다. git 에 merge 되는 순간에만 PR 리뷰를 하거나 알림이 가는 방식아닌가요. 이런 관점에서 보면 slack 이건 부요 기능이라기 보다는 GIT 모드를 하나 만들던지 해서 git hook 과 연동하는게 더 나은 방법으로 보입니다. 아니면 DEV 모드에서 git hook 연동하거나.

---

## 🤖 Antigravity · `2026-11-28 19:15:27` {#step-82}

맞습니다. **`guard`의 본질은 "로컬 AI 개발 오케스트레이션"에 두는 것이 훨씬 일관성 있습니다.**

Slack/Discord 웹훅은 `guard`를 불필요하게 팀 메신저·실시간 협업 영역으로 확장시켜, 데몬리스 로컬 도구라는 정체성과도 어긋납니다. 말씀하신 대로 원격 협업은 이미 **Git(`push` / PR / `merge`)을 경계로 비동기화**되어 있으므로, 외부 알림 대신 **로컬과 원격의 경계인 Git Hook과 `guard`를 연결하는 것이 훨씬 자연스럽고 실용적**입니다.

---

### Git Hook 연동 방식 비교: `DEV` 모드 내장 vs 별도 `GIT` 모드

| 접근 방식 | 동작 방식 | 장점 | 단점 |
| :--- | :--- | :--- | :--- |
| **1. `DEV`/`TEST` 모드 + Git Hook (`pre-commit` / `pre-push`)** | 에이전트가 `git commit`·`git push`를 실행할 때 Git Hook이 `guard` 상태(미해결 `handoff` 유무, 정책 위반, 테스트 통과)를 검사 | 별도 모드 전환 없이 로컬 작업이 원격으로 나가는 관문을 자동 통제 | `DEV` 에이전트가 중간 커밋을 자주 하는 워크플로우라면 `pre-commit`이 번거로울 수 있음 (`pre-push`가 더 적합) |
| **2. 별도 `GIT` (또는 `RELEASE`) 모드 분리** | `PLAN`/`TEST`/`DEV`/`REVIEW`에서는 `git push`·PR 생성을 차단하고, `GIT` 모드(또는 `REVIEW` 승인 후)에서만 원격 반영 허용 | 에이전트가 실수로 미검증 코드를 `push`하는 사고를 모드 단위로 원천 차단 | 모드 개수가 하나 늘어나 전환 비용이 생김 |

---

### 가장 현실적인 결합 형태
* **로컬 알림은 OS/터미널 수준으로 축소**: Slack/Discord 웹훅 대신 `guard top` 상태 표시나 OS 로컬 알림(`notify-send` 등)만 유지해 로컬 집중도를 해치지 않습니다.
* **로컬 ↔ 원격 경계는 Git Hook(`pre-push`)으로 통합**:
  * 로컬에서 `DEV`/`TEST`가 자유롭게 중간 커밋을 하더라도, **`git push` 시점(Git Hook 또는 `guard` 정책)**에 미승인 `handoff` 티켓이나 미검증 변경이 있으면 원격 반영을 막아 **로컬 AI 오케스트레이션의 신뢰가 그대로 Git PR 품질로 이어지게** 만듭니다.

원하시면 [agent-orchestrators-comparison.md](file:///mnt/data/mytask/kms/guard/doc/research/agent-orchestrators-comparison.md)의 **4.2 단기 과제**에서 Slack/Discord 웹훅 내용을 제거하고, **로컬 알림 + Git Hook(`pre-push` / `GIT` 게이트) 연동** 방향으로 수정해 두겠습니다.

---

## 🧑‍💻 User · `2026-11-28 19:16:54` {#step-83}

업데이트 해주세요.

---

## 🤖 Antigravity · `2026-11-28 19:17:09` {#step-91}

[agent-orchestrators-comparison.md](file:///mnt/data/mytask/kms/guard/doc/research/agent-orchestrators-comparison.md#L87-L100)의 로드맵 다이어그램과 **4.2 단기 과제**를 업데이트했습니다.

### 변경 사항
- **외부 메신저(Slack/Discord) 웹훅 제거**: `guard`의 정체성을 **로컬 AI 개발 오케스트레이션**으로 한정해 `guard top` 및 OS 로컬 알림만 유지하도록 정리했습니다.
- **Git Hook(`pre-push` / `GIT` 게이트) 연동 추가**: 로컬 작업이 원격 비동기 협업 경계로 넘어가는 `git push` 시점에 미승인 `handoff` 티켓이나 미검증 변경분 반영을 차단하는 방향으로 보완했습니다.

---

## 🧑‍💻 User · `2026-11-28 19:19:01` {#step-92}

guard 는 HOTL 솔루션이라고 저는 생각 합니다. 사람이 여러 AI 세션과 작업을 할 때 가장 힘든건 "인지 부하" 입니다. 사람은 "확인" 만 하고 "버튼만 누르는" 그리고 "설정을 확인하는" 그러한 환경이 되어야 한다고 저는 생각 합니다. 당신 생각은 어떤가요.

---

## 🤖 Antigravity · `2026-11-28 19:19:01` {#step-93}

전적으로 동의합니다. **HITL(Human-in-the-Loop)**과 **HOTL(Human-on-the-Loop)**의 구분은 `guard`의 정체성을 가장 정확하게 설명하는 개념입니다.

---

### 1. HITL vs HOTL: 왜 HOTL이어야 하는가

* **HITL (루프 안의 인간)**: 사람이 매 단계마다 대화에 끼어들어 맥락을 읽고 지시를 수정해야 합니다. 세션이 2~3개만 넘어가도 **인간의 인지 부하가 한계에 달해 전체 작업이 멈춥니다.**
* **HOTL (루프 위의 인간)**: 에이전트들이 루프 안에서 자율적으로 돌고, 사람은 루프 **위(관제탑)**에서 다음 3가지만 수행합니다.
  1. **설정 확인**: 세션이 올바른 모드(`PLAN`/`TEST`/`DEV`/`REVIEW`)와 정책(`guard-policy.yaml`) 위에 있는지 확인
  2. **상태 확인**: `guard top`에서 위반(`deny`) 없이 정상 진행 중인지, 테스트/정적 검증을 통과했는지 확인
  3. **버튼 누르기**: 모드 전환(`D`/`T`/`P`/`A`)이나 `handoff approve` 같은 게이트 통과 결정만 단축키로 실행

---

### 2. HOTL 환경에서 `guard`가 필수인 이유

일반적인 에이전트 툴들이 HITL에 머무르는 이유는 **에이전트를 루프 안에 풀어놓았을 때 선을 넘지 않는다는 보장이 없기 때문**입니다.
* 사람이 매 턴 지켜보는 HITL에서는 가드레일이 없어도 사람이 말로 제지할 수 있습니다.
* 반면 **사람이 루프 밖으로 빠져나와 "확인하고 버튼만 누르는" HOTL이 성립하려면, 루프 안의 에이전트가 치팅(테스트 열람, 프로덕션 유출, 자체 승인)을 못 하도록 물리적으로 묶어두는 가드레일이 반드시 전제**되어야 합니다.

즉, `guard`의 **훅 엔진(물리적 경계)**과 **`guard top` + `handoff`(버튼형 제어)**는 HOTL을 구현하기 위한 가장 핵심적인 두 기둥입니다.

---

### 3. HOTL 관점에서 본 향후 UX 핵심

"사람이 확인하고 버튼만 누르게" 하려면, 앞으로의 개선도 **"사람이 버튼을 누르기 전에 읽어야 하는 정보량을 최소로 줄이는 방향"**에 집중하는 것이 좋습니다.
* 예: `handoff approve` 대기 화면에서 긴 로그나 코드 전체 대신, **`[정책 위반 0건] [go test PASS] [변경 파일 3개]`**처럼 결정론적 검증 결과만 요약해 띄워주면, 사람은 **"확인 → 승인 버튼"**만으로 안전하게 다음 단계를 넘길 수 있습니다.

---

## 🧑‍💻 User · `2026-11-28 19:20:36` {#step-94}

지금 HOTL 의 핵심은 guard top 이라고 봅니다. guard top 기반으로 전체 AI 작업을 사람이 모니터링 할 때, 가장 큰 인지적 부하 혹은 UX를 헤치는 부분이 뭐라고 보시나요.

---

## 🤖 Antigravity · `2026-11-28 19:47:15` {#step-100}

[view.go](file:///mnt/data/mytask/kms/guard/internal/adapters/tui/view.go)와 [handoff_modal.go](file:///mnt/data/mytask/kms/guard/internal/adapters/tui/handoff_modal.go)의 현재 구현을 기준으로 보면, **"사람은 확인하고 버튼만 누르고 설정을 확인한다"**는 HOTL 관점에서 인지 부하를 유발하는 핵심 지점은 다음 5가지입니다.

---

### 1. 세션 식별 불가: "이 세션이 무슨 작업 중인지" 모름
* **현재 상태**: [renderSessionsPanel](file:///mnt/data/mytask/kms/guard/internal/adapters/tui/view.go#L105-L150)의 컬럼이 `ID`(해시 8자리), `MODE`, `WORKSPACE`, `LAST_ACTIVE`, `DENY`뿐입니다.
* **UX 문제**: 같은 워크스페이스에서 2~3개 세션이 돌면 `ID`만 보고는 어느 세션이 어떤 티켓/작업을 수행 중인지 알 수 없습니다. 결국 사람이 커서를 옮겨 하단 로그를 읽어보며 맥락을 추론해야 합니다.
* **개선 포인트**: 세션과 연결된 **현재 Handoff 요약(`Summary`) 또는 작업 라벨** 컬럼 추가.

### 2. "지금 사람이 필요한 세션"이 한눈에 안 들어옴 (Attention / Status 부재)
* **현재 상태**: `DENY`는 누적 숫자만 표시되고, 세션의 현재 상태(정상 진행 중 / 연속 `DENY`로 루프 정체 / `handoff approve` 대기 중)가 구분되지 않습니다.
* **UX 문제**: HOTL 조율자는 "아무 문제 없는 세션"은 무시하고 **"내 버튼이 필요한 세션"**만 보고 싶어합니다. 지금은 사람이 매번 각 세션을 눈으로 스캔하며 상태를 파악해야 합니다.
* **개선 포인트**: `WAITING_APPROVAL`, `BLOCKED(연속 deny)`, `ACTIVE`, `IDLE` 같은 **상태 배지 및 주의 세션 상단 정렬**.

### 3. 정상 `[ALLOW]` 로그 홍수와 하단 패널 토글(`[H]`)
* **현재 상태**: [renderEventsPanel](file:///mnt/data/mytask/kms/guard/internal/adapters/tui/view.go#L193-L214)은 `[ALLOW]`와 `[DENY]`를 섞어서 최신순으로 출력하며, 하단 영역을 이벤트 로그와 핸드오프 큐가 `[H]` 토글로 나눠 씁니다([view.go](file:///mnt/data/mytask/kms/guard/internal/adapters/tui/view.go#L59-L63)).
* **UX 문제**: 에이전트가 파일을 조금만 읽어도 수십 개의 `[ALLOW]`가 화면을 채워 정작 중요한 `[DENY]`가 밀려납니다. 또한 승인 대기 중인 Handoff가 있는지 보려면 `[H]`를 눌러 화면을 전환해야 합니다.
* **개선 포인트**: **`[DENY]` 전용 필터 토글**과 **승인 대기 Handoff 배너/알림 상시 노출**.

### 4. Handoff 승인 모달에 "버튼을 누를 판단 근거"가 부족함
* **현재 상태**: [renderHandoffSummaryModal](file:///mnt/data/mytask/kms/guard/internal/adapters/tui/handoff_modal.go#L54-L115)에는 `Summary`, `References`, `Targets`, `ParentID`만 표시됩니다.
* **UX 문제**: 사람이 안심하고 `[Enter]`(승인 버튼)를 누르려면 **"에이전트가 치팅 없이 제대로 끝냈는가"**를 바로 확인할 수 있어야 합니다. 지금은 근거가 없어 결국 다른 터미널에서 `git diff`나 `go test`를 쳐보게 됩니다.
* **개선 포인트**: 모달에 **변경 파일 요약(`git diff --stat`), 테스트/정적 검증 결과, 해당 세션 위반 횟수** 같은 결정론적 체크리스트를 함께 표시.

### 5. "설정 확인" 뷰 부재 (모드 이름만 보이고 실제 규칙/Scope Lock이 안 보임)
* **현재 상태**: 세션 테이블에 `DEV`, `TEST` 같은 모드 배지 및 단축키(`[D]/[T]/[P]/[A]`)만 있고, 현재 적용 중인 정책 파일 경로, 차단 패턴, 또는 `Targets` 기반 Scope Lock 상태를 `guard top` 내에서 바로 조회하는 화면이 없습니다.
* **UX 문제**: 말씀하신 HOTL의 3요소 중 **"설정을 확인하는"** 행위를 하려면 결국 `guard top`을 벗어나 `guard-policy.yaml`을 직접 열어야 합니다.
* **개선 포인트**: 선택한 세션의 **활성 정책 요약(허용/차단 패턴 + 적용 중인 Scope Lock 대상)**을 팝업 모달(예: `[I]` Info)로 즉시 확인하는 기능.

---

## 🧑‍💻 User · `2026-11-28 19:50:06` {#step-101}

작업 계획서를 만들고 그에 따라 작업을 하도록하겠습니다. 어느 디렉토리에 만들면 좋을까요.

---

## 🤖 Antigravity · `2026-11-28 19:50:06` {#step-103}

기존 `doc/` 디렉터리 구조를 기준으로 보면 목적에 따라 두 곳 중 하나를 선택하시면 됩니다. (`guard-policy.yaml`상 두 경로 모두 `doc/**`에 포함되어 `PLAN`/`TEST`/`DEV` 모드에서 모두 참조 가능합니다.)

1. **`doc/` 루트** (예: `doc/13_hotl_tui_improvement_plan.md`)
   * 기존 [03_tui_control_tower.md](file:///mnt/data/mytask/kms/guard/doc/03_tui_control_tower.md), [11_review_mode_design.md](file:///mnt/data/mytask/kms/guard/doc/11_review_mode_design.md), [12_custom_mode_plan.md](file:///mnt/data/mytask/kms/guard/doc/12_custom_mode_plan.md)처럼 **기능 배경·UX 설계·전체 로드맵**을 포함하는 상위 계획서일 때 적합합니다.

2. **`doc/dev/`** (예: `doc/dev/hotl_tui_dev_plan.md`)
   * 기존 `doc/dev/handoff_protocol_dev_plan.md`, `doc/dev/init_automation_dev_plan.md`와 같이 **곧바로 `TEST`/`DEV` 핸드오프(`--ref`)에 넘겨 구현할 상세 개발 작업 계획서**일 때 적합합니다.

이번 작업은 `guard top`의 HOTL UX 개선을 실제 구현까지 이어가는 단계이므로, **`doc/dev/`** (또는 상위 설계가 필요하면 `doc/13_...` 작성 후 `doc/dev/`로 상세화)에 두는 것을 추천합니다.

---

## 🧑‍💻 User · `2026-11-28 19:59:01` {#step-104}

doc/task 디렉토리에 만들어주세요.

---

## 🤖 Antigravity · `2026-11-28 19:59:20` {#step-107}

[doc/task/hotl_guard_top_plan.md](file:///mnt/data/mytask/kms/guard/doc/task/hotl_guard_top_plan.md)에 `guard top` HOTL UX 개선 작업 계획서를 작성했습니다.

### 계획서 요약 (5개 Task)
1. **Task 1 (세션 식별 & 상태 배지)**: 세션 테이블에 작업 요약(`Summary`) 및 상태 배지(`WAIT`, `BLOCKED`, `ACTIVE`, `IDLE`) 추가, 개입 필요 세션 우선 정렬
2. **Task 2 (`[DENY]` 필터 & 대기 배너)**: 이벤트 패널 `[F]` 키로 `DENY ONLY` 필터 토글, 승인 대기 Handoff 상시 배너 노출
3. **Task 3 (승인 모달 근거 보강)**: `renderHandoffSummaryModal`에 발신 세션 `DENY` 횟수, `git diff --stat` 변경 요약 등 결정론적 체크리스트 표시
4. **Task 4 (설정 확인 모달 `[I]`)**: `guard top` 내에서 선택 세션의 정책 파일 경로, 활성 규칙 패턴, Scope Lock(`Targets`) 즉시 조회
5. **Task 5 (로컬 알림 & Git 경계 연동)**: `WAIT`/`BLOCKED` OS 로컬 알림 및 `pre-push` Git Hook 게이트 연계

원하시는 단계부터 `guard handoff send`로 `TEST`/`DEV` 모드에 위임하거나 바로 진행하실 수 있습니다.

---

## 🧑‍💻 User · `2026-11-28 20:45:34` {#step-104}

지금 hand off UI/UX를 검토해주세요. 개발자는 guard top 상에서 한번에 이슈를 파악하고 Enter 혹은 Cancel 할 수 있어야 할 겁니다. 검토해주세요.

---

## 🤖 Antigravity · `2026-11-28 20:46:01` {#step-113}

현재 [handoff_modal.go](file:///mnt/data/mytask/kms/guard/internal/adapters/tui/handoff_modal.go), [update.go](file:///mnt/data/mytask/kms/guard/internal/adapters/tui/update.go), [view.go](file:///mnt/data/mytask/kms/guard/internal/adapters/tui/view.go), [model.go](file:///mnt/data/mytask/kms/guard/internal/adapters/tui/model.go) 기준으로 `guard top`의 핸드오프 UI/UX를 검토한 결과입니다.

---

### 현재 동작 방식
* 0.5초 폴링([model.go](file:///mnt/data/mytask/kms/guard/internal/adapters/tui/model.go#L162-L185))으로 `pending` 상태인 가장 오래된 티켓이 감지되면 자동으로 요약 모달(`handoffViewSummary`)이 뜹니다.
* 모달에서 `Enter`로 즉시 승인, `Esc`로 즉시 거절, `V`로 참조 문서 뷰어를 열 수 있습니다([update.go](file:///mnt/data/mytask/kms/guard/internal/adapters/tui/update.go#L81-L94)).

---

### 개선이 필요한 UI/UX 문제점 5가지

1. **`Esc` = "모달 닫기"가 아니라 "즉시 거절(Reject)"로 동작함** ([update.go](file:///mnt/data/mytask/kms/guard/internal/adapters/tui/update.go#L89-L91))
   * 일반 TUI/모달 UX에서는 `Esc`가 "창 닫기/취소"인데, 현재는 `Esc`를 누르면 확인 없이 `rejectActiveHandoff()`가 실행되어 티켓이 거절됩니다.
   * **"잠깐 닫고 세션 상태나 최근 DENY 로그를 확인한 뒤 결정"할 수 있는 보류(Dismiss) 동작이 전혀 없습니다.**
   * **권장**: `Enter`(승인) / `X` 또는 `R`(거절) / `Esc`(모달만 닫고 보류)로 분리하고, 거절 키는 Footer에도 명확히 표기(`[X] Reject`, `[Esc] Dismiss`).

2. **상세 이슈 내용(`Payload`)이 모달에 표시되지 않음** ([handoff_modal.go](file:///mnt/data/mytask/kms/guard/internal/adapters/tui/handoff_modal.go#L67-L98))
   * 현재 모달은 `Summary`, `References`, `Targets`, `ParentID`만 보여주고 `item.Payload`는 렌더링하지 않습니다.
   * 에이전트가 `--payload`에 구체적인 에러 원인이나 수정 지시사항을 담아 보내도 `guard top` 화면에서는 한 번에 파악할 수 없습니다.
   * **권장**: 모달 본문에 `Payload` 섹션(길면 스크롤 또는 자동 줄바꿈)을 함께 표시.

3. **참조 문서(`References`)가 2개 이상일 때 첫 번째 파일만 열람 가능** ([update.go](file:///mnt/data/mytask/kms/guard/internal/adapters/tui/update.go#L202-L221))
   * `openActiveHandoffRef()`가 무조건 `m.activeHandoff.References[0]`만 엽니다.
   * 설계서와 로그 등 여러 파일이 첨부된 경우 나머지 문서를 TUI에서 확인할 수 없습니다.
   * **권장**: 뷰어 내에서 `Tab` / `←` `→` 등으로 첨부 파일 간 전환 지원(헤더에 `[1/3]` 표시).

4. **하단 핸드오프 큐 패널(`H` 키)이 읽기 전용임** ([handoff_modal.go](file:///mnt/data/mytask/kms/guard/internal/adapters/tui/handoff_modal.go#L134-L181))
   * `H` 키로 큐 패널을 열어도 단순 리스트 출력만 되며, 커서 이동·상세 보기·승인/거절 조작이 불가능합니다.
   * `pending` 티켓이 여러 건 쌓였을 때 원하는 티켓을 먼저 선택해 처리하거나, 보류한 티켓을 다시 열어볼 수 없습니다.
   * **권장**: 큐 패널 활성화 시 `↑/↓`로 티켓 선택 후 `Enter`로 상세 모달 열기 지원.

5. **거절 사유 입력 불가 & 긴 텍스트 줄바꿈 미처리**
   * 거절 시 사유가 `"rejected via TUI"`로 고정되어([update.go](file:///mnt/data/mytask/kms/guard/internal/adapters/tui/update.go#L192)) 왜 반려했는지 에이전트에게 전달할 수 없습니다.
   * `Summary`가 길 경우 줄바꿈(`wrapPrefix`) 없이 출력되어 모달 폭을 넘기거나 잘릴 수 있습니다.

---

### 추천 개선 우선순위
* **1순위 (즉시 체감)**:
  1. 모달에 `Payload` 표시 + `Summary`/`Payload` 줄바꿈 처리
  2. `Esc`(닫기/보류)와 `X` 또는 `C`(Cancel/Reject) 키 분리로 오작동 방지
  3. `References` 다중 파일 전환(`Tab` / `←` `→`)
* **2순위**:
  4. 큐 패널(`H`)에서 티켓 선택 후 상세 모달 열기
  5. 거절 시 간단한 사유 입력 프롬프트(빈 입력 후 Enter면 기본 사유 사용)

원하시면 **1순위 항목부터 바로 구현**해 드릴까요?

---

## 🧑‍💻 User · `2026-11-28 20:51:58` {#step-114}

승인

---

## 🤖 Antigravity · `2026-11-28 21:12:11` {#step-137}

1순위 핸드오프 TUI/UX 개선 작업을 완료했습니다.

### 변경 사항

1. **모달 본문에 `Payload` 표시 및 줄바꿈 처리** ([handoff_modal.go](file:///mnt/data/mytask/kms/guard/internal/adapters/tui/handoff_modal.go#L62-L135))
   * `Summary`와 `Payload`를 모달 폭에 맞게 `wrapLines`로 자동 줄바꿈해 출력합니다.
   * `Payload`가 있을 경우 `상세 내용(Details)` 섹션을 표시하며, 12줄을 초과하면 `... (+N lines)`로 생략해 레이아웃 깨짐을 방지합니다.

2. **`Esc`(닫기/보류)와 `X`/`C`(거절) 키 분리** ([model.go](file:///mnt/data/mytask/kms/guard/internal/adapters/tui/model.go#L61-L185), [update.go](file:///mnt/data/mytask/kms/guard/internal/adapters/tui/update.go#L89-L106))
   * `Enter`: 승인 및 전달
   * `X` / `C`: 거절(반려)
   * `Esc`: 모달만 닫고 보류(`dismissActiveHandoff()`). `dismissedHandoffs`에 해당 티켓 ID를 기록해 0.5초 폴링 시 모달이 즉시 다시 뜨지 않도록 처리했습니다.

3. **다중 참조 문서(`References`) 전환 지원** ([handoff_modal.go](file:///mnt/data/mytask/kms/guard/internal/adapters/tui/handoff_modal.go#L172-L195), [update.go](file:///mnt/data/mytask/kms/guard/internal/adapters/tui/update.go#L70-L87))
   * 참조 문서 뷰어(`V`)에서 `Tab` / `→` / `l` (다음 문서), `Shift+Tab` / `←` / `h` (이전 문서)로 첨부 파일 간 전환을 지원하며, 헤더에 `[1/N]` 형태로 현재 문서 번호를 표시합니다.
   * 다국어 리소스([en.json](file:///mnt/data/mytask/kms/guard/internal/i18n/locales/en.json#L185-L197), [ko.json](file:///mnt/data/mytask/kms/guard/internal/i18n/locales/ko.json#L185-L197))도 함께 업데이트했습니다.

---

### 검증 결과
* `go build ./... && go vet ./...` 실행 결과 오류 없이 통과했습니다. (TUI 테스트 실행 및 신규 테스트 케이스 추가는 TEST 모드에서 진행해 주세요.)

---

## 🧑‍💻 User · `2026-11-28 22:44:59` {#step-138}

TEST 모드에스 블랙박스 테스트를 하려면, 기능/함수시그니처/DTO/인터페이스 등이 제공되어야 하지 않나요 ?

---

## 🧑‍💻 User · `2026-11-28 22:47:07` {#step-147}

TEST 모드에스 블랙박스 테스트를 하려면, 기능/함수시그니처/DTO/인터페이스 등이 제공되어야 하지 않나요 ?

---

## 🤖 Antigravity · `2026-11-28 22:47:11` {#step-149}

네, 맞습니다. 정확히 짚으셨습니다.

[guard-policy.yaml](file:///mnt/data/mytask/kms/guard/guard-policy.yaml#L53-L72)의 `guard-test-mode-read` 규칙에 의해 **TEST 모드에서는 `doc/**`와 `*_test.go` 외 모든 프로덕션 소스 코드 열람이 차단**됩니다.

따라서 TEST 모드의 에이전트가 이번에 추가·변경된 기능에 대해 컴파일 가능한 블랙박스 테스트(`*_test.go`)를 작성하려면, `doc/` 아래 문서에 다음 정보가 명시되어 있어야 합니다:

1. **커스텀 모드 관련 공개 API 시그니처 (`internal/core`, `internal/hub/handoff`)**
   * `core.ExtractModes(policy *Policy) []string`, `core.HasMode(policy *Policy, mode string) bool`
   * `handoff.IsKnownMode(m string, knownModes []string) bool`
   * `handoff.ParseTargetWithModes(raw string, knownModes []string) (HandoffNode, error)`
   * `HandoffItem.ValidateWithModes(knownModes []string) error`, `HandoffNode.ValidateAsTargetWithModes`, `ValidateAsSenderWithModes`
   * `(*handoff.Manager).SetKnownModes(modes []string)`
   * 각 함수의 입력/반환값 및 경계 조건(`nil` 정책, 빈 문자열, 세션 ID와 모드명 충돌 구분 등)

2. **TUI 핸드오프 모달 및 모드 순환 동작 규격 (`internal/adapters/tui`)**
   * `tea.KeyMsg` 키 바인딩 변경 사항: `Enter`(승인), `X`/`C`(반려), `Esc`(모달 닫기/보류 — `reloadHandoffs()` 재실행 시 자동 재팝업되지 않음), `Tab`/`→`/`l` & `Shift+Tab`/`←`/`h`(다중 참조 문서 전환), `m`/`M`(모드 순환)
   * 렌더링 규격: `Payload` 표시 및 12줄 초과 시 생략(`... (+N lines)`), 참조 문서 뷰어 헤더 `[i/N]` 표기

현재 [12_custom_mode_plan.md](file:///mnt/data/mytask/kms/guard/doc/12_custom_mode_plan.md)에는 계획만 있고 위 **구현 시그니처와 TUI 규격 명세**가 빠져 있으므로, TEST 모드로 핸드오프하기 전에 `doc/`에 계약 명세 문서를 작성해 두어야 합니다. 원하시면 바로 작성해 드리겠습니다.

---

## 🧑‍💻 User · `2026-11-28 22:49:31` {#step-150}

doc 디렉토리 밑에 contract 디렉토리 만들어서 정리해주세요.
TEST 모드에서 블랙박스 테스트를 할 수 있도록 제공해야 합니다.

---

## 🤖 Antigravity · `2026-11-28 22:49:54` {#step-155}

TEST 모드 블랙박스 테스트 작성을 위한 계약 문서를 `doc/contract/` 디렉토리에 정리했습니다.

* [README.md](file:///mnt/data/mytask/kms/guard/doc/contract/README.md): 계약 문서 개요 및 블랙박스 테스트 작성 원칙
* [01_custom_mode_contract.md](file:///mnt/data/mytask/kms/guard/doc/contract/01_custom_mode_contract.md): `core.ExtractModes`, `core.HasMode`, `handoff.IsKnownMode`, `ParseTargetWithModes`, `ValidateWithModes` 계열, `Manager.SetKnownModes` 시그니처 및 CLI 동적 라우팅 계약
* [02_tui_handoff_ux_contract.md](file:///mnt/data/mytask/kms/guard/doc/contract/02_tui_handoff_ux_contract.md): TUI 핸드오프 모달(`Payload` 표시, `Esc` 보류 vs `X`/`C` 반려, 다중 참조 문서 전환 `Tab`/`←`/`→`) 및 모드 순환(`m`/`M`) 계약

---

## 🧑‍💻 User · `2026-11-28 22:50:34` {#step-156}

TEST 모드에서 테스트를 작성 할 수 있도록 hand off 요청 작성해주세요.

---

## 🤖 Antigravity · `2026-11-28 22:50:50` {#step-159}

TEST 모드로 블랙박스 테스트 작성 및 검증 핸드오프 티켓을 발행했습니다.

* **티켓 ID**: `hnd-20261128-ab925557` (`[VERIFY] -> TEST`)
* **참조 문서 (`--ref`)**:
  * [01_custom_mode_contract.md](file:///mnt/data/mytask/kms/guard/doc/contract/01_custom_mode_contract.md)
  * [02_tui_handoff_ux_contract.md](file:///mnt/data/mytask/kms/guard/doc/contract/02_tui_handoff_ux_contract.md)
  * [12_custom_mode_plan.md](file:///mnt/data/mytask/kms/guard/doc/12_custom_mode_plan.md)
* **대상 (`--targets`)**: `internal/core/*_test.go`, `internal/hub/handoff/*_test.go`, `internal/adapters/tui/*_test.go`, `cmd/guard/*_test.go`
* **요약**: 커스텀 모드 지원 및 TUI 핸드오프 모달 UX 개선 블랙박스 테스트 작성 및 전체 검증

`guard top`에서 `Enter`로 승인하거나 아래 명령으로 승인 후 TEST 모드 세션에서 진행할 수 있습니다:
```bash
guard handoff approve hnd-20261128-ab925557
```

---

## 🧑‍💻 User · `2026-11-28 22:51:40` {#step-4}

DEV 로 부터 테스트를 위한 hand off 요청이 전달됐습니다. 검토해주세요.

---

## 🤖 Antigravity · `2026-11-28 22:52:17` {#step-18}

DEV 세션(`089d6523`)으로부터 전달된 핸드오프 요청(`hnd-20261128-ab925557`)을 검토했습니다.

### 1. 핸드오프 티켓 요약
- **ID / 상태**: `hnd-20261128-ab925557` (`verify`, `approved`)
- **경로**: `DEV(089d6523) -> TEST(760c8379)`
- **요약**: 커스텀 모드 지원 및 TUI 핸드오프 모달 UX 개선 블랙박스 테스트 작성 및 전체 검증
- **허용 대상 (`Targets`)**:
  - `internal/core/*_test.go`
  - `internal/hub/handoff/*_test.go`
  - `internal/adapters/tui/*_test.go`
  - `cmd/guard/*_test.go`
- **요청 내용 (`Payload`)**:
  - [01_custom_mode_contract.md](file:///mnt/data/mytask/kms/guard/doc/contract/01_custom_mode_contract.md) 및 [02_tui_handoff_ux_contract.md](file:///mnt/data/mytask/kms/guard/doc/contract/02_tui_handoff_ux_contract.md) 명세 기준 블랙박스 테스트(`*_test.go`) 작성
  - `go test ./...` 전체 통과 여부 검증 (특히 `Esc` 보류 vs `X/C` 반려 분리, 다중 참조 전환, 커스텀 모드 handoff 및 CLI 라우팅)

---

### 2. 참조 문서 기반 테스트 범위 검토
1. **[01_custom_mode_contract.md](file:///mnt/data/mytask/kms/guard/doc/contract/01_custom_mode_contract.md)** (`internal/core`, `internal/hub/handoff`, `cmd/guard`)
   - `core.ExtractModes` / `core.HasMode`: 정규화(`NormalizeMode`), 중복 제거, 빈 모드 규칙 제외, `ModeAll` 기본 포함, `policy == nil` 및 바이패스 모드(`ALL`, `QA`) 처리.
   - `handoff`: `IsKnownMode`, `ParseTarget` / `ParseTargetWithModes`(커스텀 모드 vs 세션 ID 구분), `Validate...WithModes` 계열, `Manager.SetKnownModes` 연동 및 하위 호환성.
   - `cmd/guard`: 정책에 커스텀 모드(예: `REVIEW`) 정의 시 `guard review`, `guard review <cmd...>`, `guard handoff send --to review`, 미정의 명령(`guard unknowncmd`) 종료 코드 `1` 반환.
2. **[02_tui_handoff_ux_contract.md](file:///mnt/data/mytask/kms/guard/doc/contract/02_tui_handoff_ux_contract.md)** (`internal/adapters/tui`)
   - `handoffViewSummary`: Summary 줄바꿈, Payload 12줄 초과 시 `... (+N lines)` 생략 표시, Footer 키 안내, `enter`(승인) / `x,X,c,C`(반려) / `esc`(상태 `StatusPending` 유지 보류 및 재팝업 방지) / `v,V`(참조 문서 열기).
   - `handoffViewRefViewer`: 다중 참조 시 `[1/N]` 헤더 및 Footer 안내, `tab`/`right`/`l` 및 `shift+tab`/`left`/`h` 순환 전환, `esc`/`v`/`V` 요약 모달 복귀, 스크롤 키 동작.
   - 대시보드 `m`/`M` 키: 정책 모드 목록 기반 세션 모드 순환 전환 및 미선택 시 에러 토스트.
3. **[12_custom_mode_plan.md](file:///mnt/data/mytask/kms/guard/doc/12_custom_mode_plan.md)**
   - Phase 1~4 구현 배경 및 커스텀 모드 E2E 검증 시나리오 일치 확인.

---

### 3. 특이사항 (현재 세션 도구 제한)
- 기존 테스트 파일 목록 확인 및 `go test ./...` 실행을 시도했으나, pre-tool hook에 의해 차단되었습니다.
  - **차단 메시지**: `[POLICY VIOLATION] TEST 모드에서는 doc/, test/ 디렉토리와 테스트 파일(*_test.go)만 수정할 수 있습니다. 다른 파일에는 접근할 수 없습니다.`
- 허용된 `Targets` 경로의 `*_test.go` 작성/수정을 진행할지, 또는 `go test` 실행 권한/명령 제한 확인이 필요한지 지시해 주시면 바로 이어서 진행하겠습니다.

---

## 🧑‍💻 User · `2026-11-28 22:54:33` {#step-108}

TEST 모드는 go test 명령을 실행할 수 있어야 합니다. 현재 실행 금지된거 같은데 확인해주세요.

---

## 🤖 Antigravity · `2026-11-28 22:55:17` {#step-112}

확인 결과, 순수 `go test` 명령 자체는 허용되지만 **리다이렉션(`2>&1`)이나 파이프(`| grep` 등)가 붙는 경우 차단**되고 있습니다.

### 실제 `GUARD_SESSION_ID="" AGENT_MODE=TEST ./guard hook` 테스트 결과

| 실행 명령 | 판정 | 원인 |
| :--- | :---: | :--- |
| `go test ./...` | `allow` | 정상 허용 |
| `go test -v ./internal/core/...` | `allow` | 정상 허용 |
| `go test ./... 2>&1` | **`deny`** | [guard-policy.yaml](file:///mnt/data/mytask/kms/guard/guard-policy.yaml#L65-L74)의 `guard-test-mode-write`(`patterns: ["**"]`)에 걸림. 훅의 쓰기 리다이렉션 검사가 `2>&1`(`&1`)·`/dev/null`도 파일 쓰기 대상으로 판정해 차단됨 |
| `go test ./... \| grep FAIL` | **`deny`** | [guard-policy.yaml](file:///mnt/data/mytask/kms/guard/guard-policy.yaml#L77-L90)의 `guard-test-mode-command`에 `grep`, `awk`, `sed`가 포함되어 있어 파이프 필터링 시 차단됨 |

*(참고: 만약 테스트를 돌린 세션 모드가 `TEST`가 아니라 `DEV`였다면 `guard-test-command-exec` 규칙에 의해 `go test*` 자체가 차단됩니다.)*

---

### 해결 방향
1. **`2>&1` / `/dev/null` 차단 해결**: `internal/core/command.go`의 출력 리다이렉션 검사에서 `&1`, `&2` 같은 파일 디스크립터 복제와 `/dev/null`을 쓰기 대상에서 제외하도록 수정이 필요합니다.
2. **파이프 필터링 허용 여부**: 에이전트가 `go test ./... | grep ...` 등을 써야 한다면 `guard-test-mode-command`에서 `grep`/`awk`/`sed`를 제거하거나 `go test` 파이프라인 허용 방식을 조정해야 합니다.

원하시면 `2>&1`·`/dev/null` 리다이렉션 예외 처리를 바로 수정해 드리겠습니다.

---

## 🧑‍💻 User · `2026-11-28 22:56:25` {#step-113}

TEST 모드에 대해서 수정해 주세요.

---

## 🧑‍💻 User · `2026-11-28 22:57:39` {#step-121}

지금 뭐하는 거죠 ? 룰만 수정하는 거 아니었나요 ?

---

## 🤖 Antigravity · `2026-11-28 22:57:39` {#step-123}

죄송합니다. 코드(`command.go`)는 건드리지 않고 [guard-policy.yaml](file:///mnt/data/mytask/kms/guard/guard-policy.yaml)의 `TEST` 모드 룰만 수정하겠습니다.

`guard-test-mode-write`의 `exclude_patterns`에 `&1`, `&2`, `/dev/null`을 추가하고, `go test ./... | grep ...` 같은 파이프 필터링도 허용되도록 `guard-test-mode-command`를 조정한 뒤 훅 동작을 바로 확인하겠습니다.

---

## 🧑‍💻 User · `2026-11-28 22:58:36` {#step-125}

룰이 너무 장황한거 같은데요. 그냥 go test 로 시작되는 거면 뒤에 뭐가 붙든 성공하도록 하면 되는 거 아닌가요 ?

---

## 🤖 Antigravity · `2026-11-28 22:58:37` {#step-126}

`guard` 엔진의 동작 방식 때문에 `guard-policy.yaml` 룰만으로는 **"go test로 시작하면 뒤는 무조건 통과"**를 표현할 수 없습니다. 이유는 두 가지입니다.

1. **`guard-test-mode-command`에는 애초에 `go test` 차단 규칙이 없습니다.** (`go test ./...` 자체는 이미 통과합니다.)
2. **`2>&1`이나 `| grep`이 막히는 이유는 `go test` 때문이 아니라 뒤쪽 토큰 때문입니다.**
   - `2>&1` → `guard-test-mode-write`(`patterns: ["**"]`)가 `&1`을 파일 경로로 보고 차단 ([command.go](file:///mnt/data/mytask/kms/guard/internal/core/command.go#L759-L763), [evaluator.go](file:///mnt/data/mytask/kms/guard/internal/core/evaluator.go#L143-L171))
   - `| grep` → 파이프(`|`) 뒤 세그먼트의 `grep`이 `guard-test-mode-command`에 걸려 차단

---

따라서 **코드는 건드리지 않고 [guard-policy.yaml](file:///mnt/data/mytask/kms/guard/guard-policy.yaml)만 최소한으로 고쳐서** `go test ... 2>&1`과 `go test ... | grep`이 통과되게 하려면 아래 두 줄 수정이 가장 단순합니다.
1. `guard-test-mode-write`의 `exclude_patterns`에 `"&1"`, `"&2"`, `"/dev/null"` 추가
2. `guard-test-mode-command`에서 `grep`, `awk`, `sed` 제거

이대로 `guard-policy.yaml`만 적용할까요?

---

## 🧑‍💻 User · `2026-11-28 23:00:39` {#step-127}

command 에 파이프와 리다이렉션 등까지 세부적으로 설정할 필요가 있나요 ?

---

## 🤖 Antigravity · `2026-11-28 23:00:39` {#step-128}

원래는 **세부적으로 설정할 필요가 없는 게 맞습니다.** 지금 룰이 복잡해 보였던 이유는 두 가지 성격이 섞여 있기 때문입니다.

1. **리다이렉션(`2>&1`, `/dev/null`)은 원래 룰에서 신경 쓸 영역이 아닙니다.**
   * `guard-test-mode-write`처럼 `patterns: ["**"]`로 전체 쓰기를 막는 규칙을 쓸 때, 현재 `guard` 엔진이 `&1`이나 `/dev/null`도 일반 파일 경로로 오판해 차단하기 때문에 임시로 `exclude_patterns`에 넣어야 하는 것입니다. (원래는 엔진이 무시해야 할 영역)

2. **파이프 도구(`grep`, `awk`, `sed`)까지 막을 필요는 낮습니다.**
   * `grep`·`awk`·`sed`는 프로덕션 코드 훔쳐보기를 막기 위해 넣었지만, `go test ./... | grep FAIL` 같은 정상적인 테스트 출력 필터링까지 막혀 오히려 작업 흐름을 방해합니다.
   * 실용적인 관점에서는 **`git diff`, `git show`, `git log *-p*`** 정도만 막고, 범용 파이프 도구(`grep`, `awk`, `sed`, `rg`)는 굳이 막지 않는 편이 룰도 단순하고 쾌적합니다.

---

요약하면:
- **가장 단순한 룰 구성**: `guard-test-mode-command`는 `git diff`, `git show`, `git log *-p*` (및 `--patch`, `-u`)만 남기고, `guard-test-mode-write`의 `exclude_patterns`에만 `&1`, `&2`, `/dev/null`을 예외로 두면 `go test` 실행 시 리다이렉션과 파이프 모두 걸리지 않습니다.

---

## 🧑‍💻 User · `2026-11-28 23:01:56` {#step-129}

기능을 수정하도록 하겠습니다. DEV 세션에 개능 개발 사항을 hand off 해주세요.

---

## 🤖 Antigravity · `2026-11-28 23:02:35` {#step-132}

`DEV` 모드로 핸드오프 티켓을 발행했습니다.

- **티켓 ID**: `hnd-20261128-fb1f6304` (`TASK` → `DEV`)
- **대상 파일 (`Targets`)**: [internal/core/command.go](file:///mnt/data/mytask/kms/guard/internal/core/command.go)
- **참조 문서 (`References`)**: [doc/01_requirements.md](file:///mnt/data/mytask/kms/guard/doc/01_requirements.md)
- **요약**: `TEST` 모드 등에서 `2>&1`(`&1`, `&2` 등 FD 복제) 및 `/dev/null` 출력 리다이렉션이 `write` 규칙(`**`)에 의해 차단되지 않도록 `hasOutputRedirection` 및 `extractOutputRedirectionTargets` 수정 (`go test ./... 2>&1` 허용)

`guard top` 또는 `guard handoff approve hnd-20261128-fb1f6304`로 승인하시면 `DEV` 세션에서 바로 작업할 수 있습니다.

---

## 🧑‍💻 User · `2026-11-28 23:03:41` {#step-160}

ALL 세션으로 부터 hand off 요청이 전달됐습니다. 검토해주세요.

---

## 🤖 Antigravity · `2026-11-28 23:07:48` {#step-176}

ALL 세션(`2667df37`)으로부터 전달된 핸드오프 티켓(`hnd-20261128-fb1f6304`, 대상: [command.go](file:///mnt/data/mytask/kms/guard/internal/core/command.go))을 검토했습니다.

---

### 1. 원인 분석

* **토크나이저 동작** ([command.go](file:///mnt/data/mytask/kms/guard/internal/core/command.go#L952-L965)):
  * `go test ./... 2>&1` ➔ `["go", "test", "./...", "2>", "&1"]`
  * `cmd >/dev/null` ➔ `["cmd", ">", "/dev/null"]`
* **문제 지점**:
  1. `hasOutputRedirection(tokens)` ([command.go](file:///mnt/data/mytask/kms/guard/internal/core/command.go#L1006-L1013)): 리다이렉션 연산자(`2>`, `>` 등)만 보이면 대상과 무관하게 `true`를 반환해 `isSafeSegment`([command.go](file:///mnt/data/mytask/kms/guard/internal/core/command.go#L653-L657))에서 무조건 탈락합니다.
  2. `extractOutputRedirectionTargets(tokens)` ([command.go](file:///mnt/data/mytask/kms/guard/internal/core/command.go#L1015-L1024)): 뒤따르는 `"&1"`, `"/dev/null"`을 일반 파일 타겟으로 그대로 추출합니다. 그 결과 `inspectWriteSegment`([command.go](file:///mnt/data/mytask/kms/guard/internal/core/command.go#L757-L763))가 `evaluatePathRules(..., "write", "&1")`을 호출해 TEST 모드의 `**` write 규칙에 걸려 차단됩니다.

---

### 2. 수정 방안 (Scope: `internal/core/command.go`)

1. **무해한 출력 대상 판별 헬퍼 추가 (`isBenignOutputTarget(target string) bool`)**
   * `target == "/dev/null"`인 경우 허용
   * FD 복제/닫기 표기(`&0`, `&1`, `&2`, `&-` 등: `&` 뒤가 모두 숫자이거나 `-`인 경우)만 허용 (`&file.go` 같은 파일명 우회는 차단 유지)
2. **`extractOutputRedirectionTargets(tokens)` 수정**
   * `isBenignOutputTarget(tokens[i+1])`가 `true`이면 타겟 목록에서 제외(`i++`로 스킵만 수행).
3. **`hasOutputRedirection(tokens)` 수정**
   * 연산자 뒤에 `isBenignOutputTarget`이 아닌 실제 대상이 있거나 대상이 누락된 경우에만 `true`를 반환하도록 변경.

---

### 3. 결론
타당한 요청이며, 수정 범위도 [command.go](file:///mnt/data/mytask/kms/guard/internal/core/command.go) 내 2개 함수 + 헬퍼 1개로 명확합니다. 바로 수정 진행하고 `guard handoff ack hnd-20261128-fb1f6304` 처리할까요?

---

## 🧑‍💻 User · `2026-11-28 23:11:23` {#step-177}

지금 수정은 command 에 일반적으로 적용될 수 있는 건가요. 아니면 이 경우만을 해결하려고 부분 최적화 하는 건가요.

---

## 🤖 Antigravity · `2026-11-28 23:11:23` {#step-178}

**특정 케이스용 부분 최적화가 아니라, 모든 명령어·모든 모드에 공통 적용되는 일반적인 수정**입니다.

### 이유
1. **적용 범위**: `hasOutputRedirection`과 `extractOutputRedirectionTargets`는 `go test`나 `TEST` 모드 전용이 아니라, 모든 셸 커맨드 파이프라인을 검사하는 `isSafeSegment`와 `inspectWriteSegment`의 공통 파서 계층입니다.
2. **셸 문법상의 보편성**: 파일 쓰기가 발생하지 않는 출력 리다이렉션은 POSIX 셸 전반에서 동일하게 다음 두 형태입니다.
   * **파일 디스크립터(FD) 복제/닫기**: `2>&1`, `1>&2`, `>&1`, `>&-` 등 (`&` + 숫자 또는 `-`)
   * **Null 디바이스**: `/dev/null`
3. **혼합 케이스 처리**:
   * `go test ./... 2>&1` ➔ 쓰기 대상 없음(`targets = []`, `hasOutputRedirection = false`)으로 안전 통과
   * `cmd > out.txt 2>&1` ➔ `out.txt`는 일반 파일이므로 `targets = ["out.txt"]`로 정상 추출되어 쓰기 정책 검사 수행
   * `cmd > &out.txt` ➔ `&` 뒤가 숫자가 아니므로 우회 시도로 간주해 정상 검사

즉, `2>&1`만 예외 처리하는 하드코딩이 아니라 **"실제 파일 쓰기를 유발하지 않는 FD 복제·`/dev/null` 리다이렉션을 리다이렉션 타겟에서 제외하는"** 셸 파서의 누락된 일반 규칙을 보완하는 수정입니다. (진행할까요?)

---

## 🧑‍💻 User · `2026-11-28 23:11:43` {#step-179}

승인

---

## 🤖 Antigravity · `2026-11-28 23:15:53` {#step-188}

핸드오프 티켓 `hnd-20261128-fb1f6304`의 수정을 완료하고 `ack` 처리했습니다.

### 변경 사항 ([command.go](file:///mnt/data/mytask/kms/guard/internal/core/command.go#L1006-L1052))
* `isBenignOutputTarget(target string) bool` 추가: `/dev/null` 및 FD 복제/닫기 표기(`&1`, `&-` 등 `&` + 숫자/`-`)만 무해한 출력 대상으로 판별합니다.
* `hasOutputRedirection(tokens []string) bool`: 연산자 뒤에 `isBenignOutputTarget`이 아닌 실제 파일 대상이 있거나 대상이 누락된 경우에만 `true`를 반환하도록 수정했습니다.
* `extractOutputRedirectionTargets(tokens []string) []string`: 무해한 출력 대상은 write 규칙 검사 대상 목록에서 제외하도록 수정했습니다.

### 검증 및 완료 처리
* `go build ./... && go vet ./...` 통과
* `guard handoff ack hnd-20261128-fb1f6304` 실행 완료 (`Handoff acknowledged: hnd-20261128-fb1f6304`)

---

## 🧑‍💻 User · `2026-11-28 23:18:58` {#step-19}

권한을 풀었습니다. 다시 테스트 진행하세요. 테스트 실패하면, 임의 수정하지 말고, 레포팅 해주세요.

---

## 🤖 Antigravity · `2026-11-28 23:19:10` {#step-25}

`go test ./...` 실행 결과 **1개 패키지(`guard/test/e2e`)에서 테스트 실패**가 발생했으며, 지시대로 임의 수정 없이 결과를 보고합니다.

### 1. 전체 실행 결과 (`go test ./...`, exit code `1`)
- **통과 (`ok`)**:
  - `guard/cmd/guard` (`0.008s`)
  - `guard/internal/adapters/hook` (`0.017s`)
  - `guard/internal/adapters/launcher` (`0.046s`)
  - `guard/internal/adapters/tui` (`0.029s`)
  - `guard/internal/core` (`0.021s`)
  - `guard/internal/hub/policy` (`0.016s`)
  - `guard/internal/hub/session` (`0.095s`)
  - `guard/internal/hub/setup` (`0.038s`)
  - `guard/internal/i18n` (`0.004s`)
- **테스트 파일 없음 (`?`)**:
  - `guard/internal/hub/handoff` (`[no test files]`)
- **실패 (`FAIL`)**:
  - `guard/test/e2e` (`0.622s`)

---

### 2. 실패 상세 내역
- **테스트명**: `TestHandoffTUI01_ModalAndPanelToggle`
- **위치**: [handoff_tui_i18n_test.go](file:///mnt/data/mytask/kms/guard/test/e2e/handoff_tui_i18n_test.go#L108-L132)
- **오류 메시지**:
  ```
  handoff_tui_i18n_test.go:131: expected ticketB status "rejected" after [Esc], got "pending"
  ```
- **원인**:
  - 해당 E2E 테스트는 `[Esc]` 입력 시 티켓 상태가 `rejected`(`handoff.StatusRejected`)로 변경되는 기존 동작을 검증하고 있습니다.
  - 반면 [02_tui_handoff_ux_contract.md](file:///mnt/data/mytask/kms/guard/doc/contract/02_tui_handoff_ux_contract.md#L28-L29) 명세에서는 `esc`가 상태를 `StatusPending`으로 유지한 채 모달만 닫고(보류), 반려(`StatusRejected`)는 `x`/`X`/`c`/`C` 키로 분리되어 실제 구현은 `pending`을 반환하고 있습니다.

---

### 3. 추가 확인 사항
- `guard/internal/hub/handoff`에는 아직 테스트 파일이 존재하지 않습니다(`[no test files]`).
- 실패한 E2E 테스트(`TestHandoffTUI01_ModalAndPanelToggle`)를 새 UX 계약(`Esc` 보류 / `X`/`C` 반려)에 맞게 갱신할지, 그리고 `01_custom_mode_contract.md` / `02_tui_handoff_ux_contract.md` 기반 신규 블랙박스 테스트 작성을 진행할지 알려주시면 반영하겠습니다.

---

## 🧑‍💻 User · `2026-11-28 23:20:36` {#step-0}

hand off 요청에 따라서 Test 진행해주세요.

---

## 🤖 Antigravity · `2026-11-28 23:25:48` {#step-56}

핸드오프 티켓(`hnd-20261128-ab925557`)의 계약 명세([01_custom_mode_contract.md](file:///mnt/data/mytask/kms/guard/doc/contract/01_custom_mode_contract.md), [02_tui_handoff_ux_contract.md](file:///mnt/data/mytask/kms/guard/doc/contract/02_tui_handoff_ux_contract.md))를 기준으로 블랙박스 테스트를 작성하고 전체 테스트 통과 및 핸드오프 완료(`ack`) 처리를 마쳤습니다.

### 작성 및 수정된 테스트 파일
1. **Contract 01 — 동적(커스텀) 모드 지원**
   - [custom_mode_blackbox_test.go](file:///mnt/data/mytask/kms/guard/internal/core/custom_mode_blackbox_test.go): `ExtractModes`, `HasMode` 및 커스텀 모드(`REVIEW`) 규칙에 대한 `EvaluateRequest` 동작 검증
   - [custom_mode_blackbox_test.go](file:///mnt/data/mytask/kms/guard/internal/hub/handoff/custom_mode_blackbox_test.go): `IsKnownMode`, `ParseTarget` / `ParseTargetWithModes`, `Validate...WithModes`, `Manager.SetKnownModes` 검증
   - [custom_mode_blackbox_test.go](file:///mnt/data/mytask/kms/guard/cmd/guard/custom_mode_blackbox_test.go): 커스텀 모드 전환(`guard review`), 런처 실행(`guard review sh -c ...`), 커스텀 모드 핸드오프(`guard handoff send --to review`), 미정의 명령 종료 코드 `1` 검증

2. **Contract 02 — TUI 핸드오프 모달 및 모드 순환 UX**
   - [handoff_ux_blackbox_test.go](file:///mnt/data/mytask/kms/guard/internal/adapters/tui/handoff_ux_blackbox_test.go): 요약 모달 Payload 12줄 초과 생략(`... (+3 lines)`) 및 Footer 안내, `[Enter]` 승인 / `[X/C]` 반려 / `[Esc]` 보류 및 재팝업 방지, 빈 참조 토스트 및 다중 참조 뷰어(`[1/N]` 순환 전환, 스크롤, 복귀), `m`/`M` 정책 모드 순환 전환 검증
   - [handoff_tui_i18n_test.go](file:///mnt/data/mytask/kms/guard/test/e2e/handoff_tui_i18n_test.go#L108-L152): Contract 02 변경 사항에 맞게 `[Esc]` 보류(`pending` 유지) 및 `[X]` 반려(`rejected`)로 기대 동작 현행화

### 검증 및 완료 처리
- `go test -count=1 ./...` 실행 결과 11개 패키지(`guard/cmd/guard`, `guard/internal/adapters/hook`, `guard/internal/adapters/launcher`, `guard/internal/adapters/tui`, `guard/internal/core`, `guard/internal/hub/handoff`, `guard/internal/hub/policy`, `guard/internal/hub/session`, `guard/internal/hub/setup`, `guard/internal/i18n`, `guard/test/e2e`) 모두 통과 (`exit code 0`)
- `guard handoff ack hnd-20261128-ab925557` 실행 완료

---

## 🧑‍💻 User · `2026-11-28 23:32:42` {#step-57}

당신은 QA 입니다. 테스트를 통과시키는게 목적이 아니고, 실패 케이스를 찾는게 목표입니다.
다양한 정상 케이스 뿐만 아니라 예외/엣지 시나리오를 테스트해야 합니다.

예외 / 엣지 시나리오를 충분히 포함했나요 ?

---

## 🤖 Antigravity · `2026-11-28 23:32:42` {#step-58}

아니요, 현재 작성된 테스트는 **정상 동작(Happy Path) 위주**이며 QA 관점에서 요구되는 **경계값·예외·실패 엣지 시나리오가 충분히 포함되어 있지 않습니다.**

현재 컨텍스트의 테스트 코드와 계약서([01_custom_mode_contract.md](file:///mnt/data/mytask/kms/guard/doc/contract/01_custom_mode_contract.md), [02_tui_handoff_ux_contract.md](file:///mnt/data/mytask/kms/guard/doc/contract/02_tui_handoff_ux_contract.md))를 기준으로 누락된 예외/엣지 시나리오는 다음과 같습니다.

---

### 1. Contract 01 (`core` / `handoff` / `cli`) 누락 시나리오
* **`internal/core`**
  * 모든 규칙이 전역 규칙(`Mode: ""` 또는 공백)뿐일 때 `ExtractModes`가 `[]string{"ALL"}`만 반환하는지
  * `policy != nil`이지만 `Rules`가 비어 있을 때 `HasMode(pol, "DEV")`가 `false`를 반환하는지
  * 커스텀 모드(`REVIEW`) 규칙에 `ExcludePatterns`, `command` 타입 차단, 전역 규칙(`Mode: ""`)이 결합된 평가 케이스
* **`internal/hub/handoff`**
  * `Manager.SetKnownModes(nil)`로 재설정한 뒤 커스텀 모드가 다시 거부(`ErrInvalidItem`)되는지
  * `From`·`To` 모두 비어 있거나(`Mode == ""` && `SessionID == ""`), 잘못된 `From.Mode`가 들어온 실패 케이스
  * `knownModes`에 없는 유사 모드명(예: `review-v2`)이 `SessionID`로 안전하게 분기되는지, 커스텀 모드 대상 `Inbox` 필터링이 동작하는지
* **`cmd/guard` CLI**
  * 정책 파일에 `REVIEW`가 **정의되지 않은** 상태에서 `guard review` 실행 시 종료 코드 `1` 반환 여부
  * 정책에 없는 상태에서 `guard handoff send --to review` 실행 시 `Mode`가 아닌 `SessionID: "review"`로 파싱되는지
  * `guard review <cmd>` 실행 시 하위 명령이 0이 아닌 종료 코드(예: `exit 42`)로 실패할 때 종료 코드 전파 여부
  * 인자가 있는 미정의 명령(`guard unknowncmd arg1`)의 실패 처리

---

### 2. Contract 02 (`internal/adapters/tui`) 누락 시나리오
* **요약 모달 표시 경계값**
  * **Payload 정확히 12줄**: `... (+N lines)` 생략 표시가 **나오지 않아야** 하는 경계값(현재 15줄만 테스트됨)
  * **공백만 있는 Payload**(`"  \n\t "`): 상세 내용 섹션이 렌더링되지 않아야 함
  * **Summary 자동 줄바꿈** 및 **좁은 터미널 폭**(예: `Width: 40`)에서 긴 단일 행 Payload가 줄바꿈되어 12줄을 넘길 때의 생략 처리
* **참조 문서 뷰어(`handoffViewRefViewer`) 예외**
  * `References`가 **1개**일 때 헤더에 `[1/N]` 및 Footer에 `[Tab / ←/→]`가 표시되지 않는지(`len > 1` 조건)
  * `References`에 **존재하지 않는 파일**이나 **워크스페이스 외부 경로**(`../outside.md`)가 포함됐을 때 패닉 없이 에러 처리되는지
* **오버레이 우선순위 및 모드 순환(`m`/`M`) 예외**
  * 모달/도움말(`?`)이 열려 있을 때 `m`/`M` 입력이 배경 세션 모드를 변경하지 않는지
  * 정책 파일이 없는 워크스페이스에서 `m`/`M` 실행 시 기본 정책 모드로 순환하는지
  * 세션의 현재 `Mode`가 정책 목록에 없는 값일 때 `m`/`M` 입력 시 첫 번째 모드로 안전하게 폴백하는지

---

원하시면 위 예외/엣지 시나리오들을 각 패키지의 블랙박스 테스트 파일에 추가 작성하고, 실제로 실패하는 케이스(버그)가 드러나는지 검증하겠습니다. 진행할까요?

---

## 🧑‍💻 User · `2026-11-28 23:33:06` {#step-59}

승인

---

## 🤖 Antigravity · `2026-11-28 23:34:31` {#step-79}

추가한 예외·경계값·실패 시나리오를 포함하여 전체 테스트를 실행한 결과, **모든 패키지(11개)가 결함 없이 통과**했습니다.

### 추가된 예외 및 엣지 시나리오
1. **Core (`internal/core`)** — [custom_mode_blackbox_test.go](file:///mnt/data/mytask/kms/guard/internal/core/custom_mode_blackbox_test.go#L144-L221)
   - 모든 규칙이 전역(`Mode: ""` 또는 공백)뿐일 때 `ExtractModes`가 `["ALL"]`만 반환
   - `Rules`가 빈 슬라이스일 때 `HasMode`가 일반/커스텀 모드에 `false`, 바이패스(`ALL`/`QA`)에만 `true` 반환
   - 커스텀 모드(`REVIEW`)에 `ExcludePatterns`, `command` 타입 차단, 전역 규칙 결합 및 대소문자/공백 포함 모드 입력 검증
2. **Handoff (`internal/hub/handoff`)** — [custom_mode_blackbox_test.go](file:///mnt/data/mytask/kms/guard/internal/hub/handoff/custom_mode_blackbox_test.go#L176-L246)
   - `SetKnownModes(nil)` 재설정 시 커스텀 모드 티켓이 다시 `ErrInvalidItem`으로 거부되는지 검증
   - `knownModes`에 빈 문자열이 섞여도 `IsKnownMode("") == false` 유지
   - `knownModes`에 없는 유사 문자열(`review-v2`)이 `SessionID`로 분기되는지, 공백/혼합 대소문자(`"  ReViEw  "`)가 소문자 `review`로 정규화되는지, 빈 `From`/`To` 노드가 거부되는지 검증
3. **CLI 라우팅 (`cmd/guard`)** — [custom_mode_blackbox_test.go](file:///mnt/data/mytask/kms/guard/cmd/guard/custom_mode_blackbox_test.go#L153-L207)
   - 정책에 커스텀 모드가 없는 상태에서 `guard review` 실행 시 종료 코드 `1` 반환
   - 정책에 없는 상태에서 `guard handoff send --to review` 실행 시 `To.SessionID="review"`, `To.Mode=""`로 파싱
   - `guard review sh -c "exit 42"` 실행 시 하위 명령의 비정상 종료 코드(`42`) 전파
   - 인자가 포함된 미정의 명령(`guard unknowncmd arg1 arg2`)의 종료 코드 `1` 반환
4. **TUI UX (`internal/adapters/tui`)** — [handoff_ux_blackbox_test.go](file:///mnt/data/mytask/kms/guard/internal/adapters/tui/handoff_ux_blackbox_test.go#L407-L585)
   - Payload가 정확히 **12줄**일 때 `... (+N lines)` 생략 표시가 출력되지 않는 경계값
   - 공백·개행만 있는 Payload일 때 상세 내용 섹션이 렌더링되지 않음
   - 좁은 터미널 폭(`Width: 40`)에서 긴 단일 행 Payload가 줄바꿈 후 12줄 초과 시 생략 표시 동작
   - 단일 참조 문서(`len(References) == 1`)일 때 `[1/1]` 및 `[Tab / ←/→]` 안내 미표시
   - 존재하지 않는 파일 및 워크스페이스 외부 경로(`../outside_secret.txt`) 참조 시 패닉 없이 안전 처리
   - 도움말(`?`) 오버레이가 열린 상태에서 `m`/`M` 입력 시 배경 세션 모드가 변경되지 않음
   - 알 수 없는 기존 세션 모드(`LEGACY_UNKNOWN`)에서 `m` 입력 시 정책 모드로 안전하게 폴백 전환

### 실행 결과
- `go test -count=1 ./...`: 전체 11개 패키지 통과 (`exit code 0`)

---

## 🧑‍💻 User · `2026-11-28 23:39:10` {#step-80}

테스트 성공했습니다. hand off 에 대한 ack 가 필요한가요 ?

---

## 🤖 Antigravity · `2026-11-28 23:39:10` {#step-81}

아니요, 이미 이전 단계에서 `guard handoff ack hnd-20261128-ab925557`를 실행해 완료 처리(`Handoff acknowledged: hnd-20261128-ab925557`)했으므로 추가로 `ack`를 실행할 필요가 없습니다.

---

