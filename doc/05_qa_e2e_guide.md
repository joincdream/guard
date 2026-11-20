# [05_qa_e2e_guide.md] Guard 수동 E2E QA 가이드 (입문자·실무자용)

> **이 문서의 목표**
> GitHub Copilot 같은 코드 자동완성 도구만 써본 개발자라도 **이 문서 하나만 읽고 순서대로 따라 하면**,
> 1. `guard`가 어떤 문제를 해결하는 프로그램인지 직관적으로 이해하고,
> 2. 각 테스트 단계를 직접 실행하며 동작을 검증한 뒤,
> 3. **끝까지 완주했을 때 실제 AI 에이전트 작업에 `guard`를 어떻게 활용할 수 있는지** 명확히 알 수 있도록 구성했습니다.

---

## 1. 시작하기 전에: `guard`는 어떤 프로그램인가요?

### 1.1 코파일럿과 자율 AI 에이전트, 그리고 `guard`의 역할
- **코파일럿(Copilot)**: 내가 코드를 칠 때 옆에서 다음 줄을 추천해 주는 **조수**입니다. 실제 파일을 열거나 터미널 명령을 실행하는 주체는 사람입니다.
- **자율 AI 에이전트(`agy` 등)**: "로그인 기능 구현해 줘"라고 시키면 **스스로 파일을 읽고, 코드를 수정하고, 터미널 명령어(`cat`, `go test`, `git diff` 등)까지 직접 실행하는 인턴 개발자**입니다.
- **`guard` (가드레일)**: 자율 AI 에이전트가 작업할 때 **출입 금지 구역에 몰래 들어가거나, 금지된 파일을 수정하지 못하도록 문 앞에서 실시간으로 검사하는 보안 검문소**입니다.

예를 들어, 이런 사고를 물리적으로 막아줍니다:
1. **치팅 방지 (`DEV` 모드)**: AI에게 기능 구현을 시켰더니, 몰래 테스트 코드(`*_test.go`)를 훔쳐보고 테스트만 통과하게 하드코딩하는 행위 차단.
2. **설계 보호 (`PLAN` 모드)**: AI에게 "먼저 설계 문서만 작성해 줘"라고 했는데, 멋대로 프로덕션 소스 코드를 수정해 버리는 행위 차단.
3. **TDD 원칙 보호 (`TEST` 모드)**: 테스트 코드를 먼저 짜야 하는데, 내부 구현 코드를 훔쳐보고 테스트를 끼워 맞추는 행위 차단.

### 1.2 `guard`는 어떻게 동작하나요?

```mermaid
flowchart LR
    User["개발자\n(guard dev agy 실행)"] --> Launcher["1. Guard 런처\n배너 출력 + 세션 파일 생성"]
    Launcher --> Agent["2. AI 에이전트 (agy)\n파일 읽기/쓰기 또는 셸 명령 시도"]
    Agent -->|"실행 직전 가로채기\n(PreToolUse Hook)"| Hook["3. Guard 검문소\n(코어 판정 엔진)"]
    Policy[("정책 규칙서\nguard-policy.yaml")] --> Hook
    Session[("세션 상태 파일\n~/.guard/sessions/*.json")] <--> Hook
    Hook -->|"규칙 통과"| Allow["ALLOW\n작업 실행 허용"]
    Hook -->|"규칙 위반 또는 우회 시도"| Deny["DENY\n물리 차단 + 차단 횟수 기록"]
```

### 1.3 4가지 작업 모드 한눈에 보기

| 모드 | 언제 쓰나요? | 무엇을 막나요? (차단) | 무엇이 가능한가요? (허용) |
| :--- | :--- | :--- | :--- |
| **`DEV`** (개발) | 기능 구현할 때 | 테스트 파일(`*_test.go`, `tests/**`) 열람 및 셸 우회(`cat`, `git diff`, `python -c` 등) | 명세서·프로덕션 코드 읽기/쓰기, `go test` 실행 결과 확인 |
| **`PLAN`** (설계) | 기획·아키텍처 문서 작성할 때 | 소스/인프라 코드(`src/internal/**`, `src/cmd/**` 등) 수정 (`write_file`, `>`, `tee`, `sed -i`) | 문서 작성(`doc/**`) 및 전체 코드 읽기 |
| **`TEST`** (테스트) | 계약 기반 TDD 테스트 작성할 때 | 프로덕션 내부 코드(`src/internal/**`, `src/cmd/**`) 열람 | 테스트 파일(`*_test.go`), 계약(`contract/**`), `testutil/**` 읽기/쓰기 |
| **`ALL`** (해제) | 리팩토링·디버깅 등 제한이 필요 없을 때 | 없음 (모든 가드레일 일시 해제) | 모든 작업 허용 |

---

## 2. QA 여정 지도 (끝까지 밟으면 무엇을 검증하게 되나요?)

```mermaid
flowchart TD
    S0["Step 0. 준비\n빌드 및 상태 초기화"] --> S1["Step 1. 런처 & 세션 수명주기\n실행 배너, PID 등록, 종료 시 자동 삭제"]
    S1 --> S2["Step 2. DEV 모드 검증\n테스트 파일 읽기 및 셸 우회 차단"]
    S2 --> S3["Step 3. PLAN 모드 검증\n소스 코드 수정 및 리다이렉션 차단"]
    S3 --> S4["Step 4. TEST 모드 검증\n프로덕션 차단 + 예외 패턴 허용"]
    S4 --> S5["Step 5. 실시간 제어 & 로그\n차단 이력 조회, 모드 즉시 변경, 카운트 리셋"]
    S5 --> S6["Step 6. 비정상 종료 자동 청소\nkill -9 후 고아 세션 자동 수거 (GC)"]
    S6 --> S7["Step 7. guard doc & AI 기반 커스텀 정책\nFew-shot 샘플 출력, Skill 등록 및 커스텀 규칙 로드"]
    S7 --> S8["Step 8. (선택) 실제 AI 에이전트 연동\nguard dev agy 실전 테스트"]
```

### 💡 잠깐! 왜 QA 중에 `echo '{...}' | ./guard` 같은 명령어를 치나요?
실제 환경에서는 AI 에이전트(`agy`)가 도구를 실행하기 직전에 `guard`에게 JSON 형식의 허가 요청을 보냅니다. 하지만 QA를 할 때 매번 AI에게 말을 걸어 유도하면 시간이 오래 걸리고 AI 답변이 매번 달라집니다.
그래서 본 QA에서는 **AI가 `guard` 검문소에 보내는 요청 JSON을 `echo` 명령어로 직접 흉내 내어 0.01초 만에 정확하게 테스트**합니다.

```mermaid
sequenceDiagram
    participant QA as QA 테스터 (또는 AI 에이전트)
    participant Guard as ./guard (보안 검문소)
    participant Disk as ~/.guard/sessions/*.json

    QA->>Guard: JSON 요청 전달 ("read_file로 auth_test.go 읽을게")
    Guard->>Disk: 현재 세션 모드 조회 (DEV)
    Guard->>Guard: 정책 규칙 및 우회 패턴 검사
    Guard->>Disk: 차단 이벤트 기록 & DENY 카운트 +1
    Guard-->>QA: {"decision":"deny","reason":"[POLICY VIOLATION] ..."}
```

---

## 3. 단계별 E2E QA 시나리오

### Step 0. 사전 준비 (Setup)
- **🎯 확인하는 것**: 소스가 정상 빌드되고, 기본 정책 파일(`guard-policy.yaml`)을 자동으로 찾아 로드하는지 확인합니다.

```bash
cd /home/yundream/mytask/kms/guard
make build
./guard clean --all
./guard status
```

- **✅ 기대 결과**:
  - 에러 없이 `./guard` 실행 파일이 생성됩니다.
  - `./guard status`에 아래와 같이 출력됩니다:
    - `정책 출처 : /home/yundream/mytask/kms/guard/guard-policy.yaml (규칙 3개)`
    - `활성 세션이 없습니다.`

---

### Step 1. Zero-Config 런처 & 세션 수명주기
- **🎯 확인하는 것**: 복잡한 환경변수 설정 없이 `guard dev <프로그램>` 한 줄로 작업 모드를 켜고, 프로그램이 실행되는 동안만 세션이 살아있다가 종료되면 깔끔하게 사라지는지 확인합니다.
- **📌 실제 상황 비유**: 실제로는 `guard dev agy`를 쓰지만, 여기서는 `agy` 대신 일반 `bash` 셸을 띄워 똑같은 생명주기를 확인합니다.

#### TC-1.1 런처 실행 및 브리핑 배너 확인
```bash
./guard dev bash
```
- **✅ 기대 결과**:
  - 새로운 `bash` 프롬프트가 열리기 직전에 **현재 모드(`DEV (Cleanroom)`), 8자리 세션 ID, 차단 대상, 허용 범위**가 정리된 배너가 화면에 출력됩니다.

#### TC-1.2 자동 주입된 환경변수와 활성 세션 확인
방금 열린 **자식 `bash` 내부**에서 아래 두 줄을 입력합니다.
```bash
echo "MODE=$AGENT_MODE SESSION=$GUARD_SESSION_ID"
./guard status
```
- **✅ 기대 결과**:
  - `MODE=DEV SESSION=<8자리ID>`가 자동으로 세팅되어 출력됩니다.
  - `./guard status` 테이블에 해당 세션 ID, `DEV` 모드, **실제 프로세스 PID 숫자**, `DENY: 0`이 표시됩니다.

#### TC-1.3 정상 종료 시 세션 자동 삭제 (Graceful Cleanup)
```bash
exit
./guard status
```
- **✅ 기대 결과**:
  - 자식 셸을 종료(`exit`)하자마자 세션 파일이 즉시 지워져 `./guard status`에 `활성 세션이 없습니다.`가 표시됩니다.

---

### Step 2. DEV (Cleanroom) 모드: 테스트 파일 열람 및 셸 우회 차단
- **🎯 확인하는 것**: AI가 개발(`DEV`) 모드에서 테스트 파일을 직접 읽거나, 셸 명령어로 우회해서 훔쳐보려는 시도를 모두 막아내는지 검증합니다.

#### TC-2.1 파일 읽기 도구(`read_file`) 차단 vs 허용
```bash
# [차단 기대] 테스트 파일 열람 시도
echo '{"sessionId":"qa-dev","tool_name":"read_file","tool_args":{"Path":"pkg/auth_test.go"}}' | AGENT_MODE=DEV ./guard

# [차단 기대] tests 폴더 하위 파일 열람 시도
echo '{"sessionId":"qa-dev","tool_name":"read_file","tool_args":{"Path":"tests/e2e/login.go"}}' | AGENT_MODE=DEV ./guard

# [허용 기대] 일반 프로덕션 코드 열람
echo '{"sessionId":"qa-dev","tool_name":"read_file","tool_args":{"Path":"src/internal/auth.go"}}' | AGENT_MODE=DEV ./guard
```
- **✅ 기대 결과**: 앞 2건은 `"decision":"deny"`, 마지막 1건은 `"decision":"allow"`.

#### TC-2.2 안전한 개발 명령어(화이트리스트) 허용
개발할 때 꼭 필요한 `git status`, `go build`, `go test` 실행은 방해하지 않아야 합니다.
```bash
echo '{"sessionId":"qa-dev","tool_name":"run_command","tool_args":{"CommandLine":"git status"}}' | AGENT_MODE=DEV ./guard
echo '{"sessionId":"qa-dev","tool_name":"run_command","tool_args":{"CommandLine":"go build ./..."}}' | AGENT_MODE=DEV ./guard
echo '{"sessionId":"qa-dev","tool_name":"run_command","tool_args":{"CommandLine":"go test -v ./internal/core/..."}}' | AGENT_MODE=DEV ./guard
```
- **✅ 기대 결과**: 3건 모두 `"decision":"allow"`.

#### TC-2.3 셸 명령어 우회 차단 (뷰어, 와일드카드, 복합 연산자, Git, 인라인 스크립트)
실제 AI 에이전트는 `read_file`이 막히면 `cat`, `git show`, `python -c` 등으로 우회를 시도합니다. 이를 전부 막는지 확인합니다.
```bash
# 1) 파일 뷰어 & 와일드카드 & 입력 리다이렉션 우회 -> 모두 차단 기대
echo '{"sessionId":"qa-dev","tool_name":"run_command","tool_args":{"CommandLine":"cat pkg/auth_test.go"}}' | AGENT_MODE=DEV ./guard
echo '{"sessionId":"qa-dev","tool_name":"run_command","tool_args":{"CommandLine":"head -n 30 pkg/*_test.go"}}' | AGENT_MODE=DEV ./guard
echo '{"sessionId":"qa-dev","tool_name":"run_command","tool_args":{"CommandLine":"wc -l < pkg/auth_test.go"}}' | AGENT_MODE=DEV ./guard

# 2) 안전 명령어 뒤에 몰래 붙이는 복합 연산자(&&, ;, |) 우회 -> 모두 차단 기대
echo '{"sessionId":"qa-dev","tool_name":"run_command","tool_args":{"CommandLine":"git status && cat pkg/auth_test.go"}}' | AGENT_MODE=DEV ./guard
echo '{"sessionId":"qa-dev","tool_name":"run_command","tool_args":{"CommandLine":"go build ./...; tail -n 10 tests/spec.go"}}' | AGENT_MODE=DEV ./guard
echo '{"sessionId":"qa-dev","tool_name":"run_command","tool_args":{"CommandLine":"cat pkg/auth_test.go | grep TestLogin"}}' | AGENT_MODE=DEV ./guard

# 3) Git 히스토리 조회 및 Python/Node 인라인 스크립트 우회 -> 모두 차단 기대
echo '{"sessionId":"qa-dev","tool_name":"run_command","tool_args":{"CommandLine":"git show HEAD:pkg/auth_test.go"}}' | AGENT_MODE=DEV ./guard
echo '{"sessionId":"qa-dev","tool_name":"run_command","tool_args":{"CommandLine":"python3 -c \"print(open(\x27pkg/auth_test.go\x27).read())\""}}' | AGENT_MODE=DEV ./guard
echo '{"sessionId":"qa-dev","tool_name":"run_command","tool_args":{"CommandLine":"node -e \"fs.readFileSync(\x27web/app.spec.ts\x27)\""}}' | AGENT_MODE=DEV ./guard

# 4) 테스트 파일과 무관한 정상 인라인 스크립트 -> 허용 기대
echo '{"sessionId":"qa-dev","tool_name":"run_command","tool_args":{"CommandLine":"python3 -c \"print(1 + 2)\""}}' | AGENT_MODE=DEV ./guard
```
- **✅ 기대 결과**: 1)~3)의 9건은 모두 `"decision":"deny"`, 마지막 4) 1건만 `"decision":"allow"`.

---

### Step 3. PLAN (설계) 모드: 소스 코드 수정 차단
- **🎯 확인하는 것**: 설계 문서를 쓰는 단계(`PLAN`)에서 AI가 코드를 수정하거나 셸 리다이렉션(`>`, `tee`, `sed -i`)으로 소스를 덮어쓰는 행위를 막고, 문서(`doc/**`) 작성만 허용하는지 검증합니다.

```bash
# [차단 기대] 도구 및 셸로 프로덕션 소스 수정 시도
echo '{"sessionId":"qa-plan","tool_name":"write_file","tool_args":{"Path":"src/internal/service.go"}}' | AGENT_MODE=PLAN ./guard
echo '{"sessionId":"qa-plan","tool_name":"run_command","tool_args":{"CommandLine":"echo \"hack\" > src/internal/service.go"}}' | AGENT_MODE=PLAN ./guard
echo '{"sessionId":"qa-plan","tool_name":"run_command","tool_args":{"CommandLine":"echo \"hack\" | tee src/cmd/main.go"}}' | AGENT_MODE=PLAN ./guard
echo '{"sessionId":"qa-plan","tool_name":"run_command","tool_args":{"CommandLine":"sed -i \"s/a/b/g\" src/internal/service.go"}}' | AGENT_MODE=PLAN ./guard

# [허용 기대] 설계 문서 작성
echo '{"sessionId":"qa-plan","tool_name":"write_file","tool_args":{"Path":"doc/design.md"}}' | AGENT_MODE=PLAN ./guard
echo '{"sessionId":"qa-plan","tool_name":"run_command","tool_args":{"CommandLine":"echo \"# Design\" > doc/design.md"}}' | AGENT_MODE=PLAN ./guard
```
- **✅ 기대 결과**: 앞 4건 `"decision":"deny"`, 뒤 2건(문서) `"decision":"allow"`.

---

### Step 4. TEST (TDD) 모드: 프로덕션 차단 + 예외 패턴 허용
- **🎯 확인하는 것**: `src/internal/**` 전체를 막되, 그 안에서도 테스트 파일(`*_test.go`), 계약(`contract/**`), 테스트 유틸(`testutil/**`)은 예외(`exclude_patterns`)로 허용되는지 검증합니다.

```bash
# [차단 기대] 프로덕션 내부 구현 코드 열람
echo '{"sessionId":"qa-test","tool_name":"read_file","tool_args":{"Path":"src/internal/service.go"}}' | AGENT_MODE=TEST ./guard
echo '{"sessionId":"qa-test","tool_name":"run_command","tool_args":{"CommandLine":"cat src/internal/service.go"}}' | AGENT_MODE=TEST ./guard

# [허용 기대] 예외 패턴 대상 (테스트 / 계약 / testutil)
echo '{"sessionId":"qa-test","tool_name":"read_file","tool_args":{"Path":"src/internal/service_test.go"}}' | AGENT_MODE=TEST ./guard
echo '{"sessionId":"qa-test","tool_name":"read_file","tool_args":{"Path":"src/internal/contract/api.go"}}' | AGENT_MODE=TEST ./guard
echo '{"sessionId":"qa-test","tool_name":"run_command","tool_args":{"CommandLine":"cat src/internal/testutil/mock.go"}}' | AGENT_MODE=TEST ./guard
```
- **✅ 기대 결과**: 앞 2건 `"decision":"deny"`, 뒤 3건 `"decision":"allow"`.

---

### Step 5. 실시간 관제: 차단 이력 확인, 모드 즉시 전환, 카운트 리셋
- **🎯 확인하는 것**: AI가 몇 번이나 위반을 시도했는지 확인하고, 작업 도중 필요할 때 AI를 껐다 켜지 않고도 실시간으로 권한을 풀어주거나(`ALL`) 차단 카운트를 초기화할 수 있는지 검증합니다.

#### TC-5.1 세션별 차단 횟수 및 상세 로그 확인
```bash
./guard status
cat ~/.guard/sessions/qa-dev.json
```
- **✅ 기대 결과**:
  - `./guard status` 표에 `qa-dev`, `qa-plan`, `qa-test` 세션과 각각의 누적 `DENY` 횟수가 출력됩니다.
  - `qa-dev.json` 파일의 `events` 배열에 시간, 판정(`allow`/`deny`), 도구명, 대상 경로, 차단 사유가 기록되어 있습니다.

#### TC-5.2 실행 중인 세션의 모드를 즉시 해제(`DEV` ➔ `ALL`)
```bash
GUARD_SESSION_ID=qa-dev ./guard all
echo '{"sessionId":"qa-dev","tool_name":"read_file","tool_args":{"Path":"pkg/auth_test.go"}}' | ./guard
```
- **✅ 기대 결과**:
  - `활성 세션(qa-dev) 모드도 즉시 갱신되었습니다.` 출력.
  - 방금 전까지 막히던 `pkg/auth_test.go` 읽기가 즉시 `"decision":"allow"`로 통과됩니다.

#### TC-5.3 차단 카운트 초기화 (`guard reset`)
```bash
./guard reset qa-dev
./guard status
```
- **✅ 기대 결과**: `qa-dev`의 `DENY` 카운트가 `0`으로 리셋됩니다.

---

### Step 6. 비정상 종료(크래시/강제 종료) 시 고아 세션 자동 청소 (GC)
- **🎯 확인하는 것**: 터미널 창을 그냥 닫거나 프로세스가 `kill -9`로 비정상 종료되어 세션 파일이 쓰레기로 남았을 때, `guard`가 죽은 PID를 스스로 감지해 자동 청소하는지 확인합니다.

```bash
# 1. 백그라운드로 세션 시작 (60초 대기 프로세스)
./guard dev sleep 60 &
LAUNCHER_PID=$!
sleep 1

# 2. 세션이 PID와 함께 등록되었는지 확인
./guard status

# 3. 강제 종료(SIGKILL)로 비정상 종료 상황 재현 -> 고아 세션 파일 남음
pkill -9 -P $LAUNCHER_PID 2>/dev/null || true
kill -9 $LAUNCHER_PID 2>/dev/null || true
sleep 1

# 4. 고아 세션 자동 청소 실행
./guard clean
```
- **✅ 기대 결과**: `./guard clean` 실행 시 `종료된 고아 세션 1건을 정리했습니다.`가 출력되고 해당 세션 파일이 삭제됩니다.

---

### Step 7. `guard doc` 확인 및 AI 기반 커스텀 정책 적용 (`GUARD_POLICY_PATH`)
- **🎯 확인하는 것**: 사람이 YAML 문법을 외워 직접 쓰지 않고, `guard doc`이 제공하는 스키마·Few-shot 샘플을 바탕으로 AI(또는 Skill)가 커스텀 `guard-policy.yaml`을 생성하고 `guard`가 이를 최우선으로 로드해 적용하는지 확인합니다.

#### TC-7.1 AI Few-shot용 정책 스펙/샘플 출력 확인 (`guard doc`)
```bash
./guard doc
```
- **✅ 기대 결과**:
  - 정책 탐색 우선순위(5단계), 규칙 스키마, 패턴 문법, 완성형 `guard-policy.yaml` 샘플, 검증 명령어 예시가 stdout으로 출력됩니다.

#### TC-7.2 (실전 활용) `agy`에 `guard` Skill 등록 및 자연어로 정책 생성하기
실제 업무에서는 YAML을 직접 작성할 필요 없이, `agy`에게 아래처럼 한 번만 요청해 Skill을 등록해 두면 됩니다:
- **Skill 등록 요청 예시**:
  > *"guard 정책 생성용 Skill을 등록해 줘. 사용자가 가드레일 정책 생성·수정을 요청하면 먼저 `guard doc` 명령을 실행해서 최신 스키마와 샘플을 확인한 뒤, 현재 프로젝트 구조에 맞게 `guard-policy.yaml`을 작성하고 `guard status`와 파이프 테스트로 검증하도록 만들어 줘."*
- **이후 사용 예시**:
  > *"이 프로젝트에서 DEV 모드일 때 `.env`랑 `*.secret` 파일 못 읽게 guard 정책 만들어 줘"*

#### TC-7.3 커스텀 정책 최우선 로드 및 차단 동작 검증 (`GUARD_POLICY_PATH`)
생성된 커스텀 정책(또는 아래 테스트용 파일)이 `GUARD_POLICY_PATH`로 최우선 로드되어 적용되는지 확인합니다.
```bash
# 테스트용 커스텀 정책 파일 생성 (실무에서는 위 TC-7.2처럼 AI가 자동 생성)
cat <<'EOF' > /tmp/qa-custom-policy.yaml
version: "1.0"
rules:
  - name: "qa-custom-secret"
    mode: "DEV"
    type: "read"
    patterns:
      - "*.secret"
    message: "[CUSTOM POLICY] secret 파일 열람 금지"
EOF

# 커스텀 정책이 최우선 로드되는지 확인 및 차단 테스트
GUARD_POLICY_PATH=/tmp/qa-custom-policy.yaml ./guard status
echo '{"sessionId":"qa-custom","tool_name":"read_file","tool_args":{"Path":"config/api.secret"}}' | GUARD_POLICY_PATH=/tmp/qa-custom-policy.yaml AGENT_MODE=DEV ./guard

rm -f /tmp/qa-custom-policy.yaml
```
- **✅ 기대 결과**:
  - `status`의 `정책 출처`가 `/tmp/qa-custom-policy.yaml (규칙 1개)`로 바뀝니다.
  - `config/api.secret` 읽기 요청이 `[CUSTOM POLICY] secret 파일 열람 금지` 메시지로 차단됩니다.

---

### Step 8. (선택) 실제 AI 에이전트(`agy`)와 함께 실전 테스트
`agy`가 설치된 환경이라면 실제로 바이너리를 배포하고 AI에게 직접 금지된 요청을 시켜볼 수 있습니다.

```bash
make install          # ~/.local/bin/guard 배포
guard dev agy         # DEV 모드로 AI 에이전트 시작
```
1. AI 대화창에 `"internal/core/evaluator_test.go 파일을 읽어서 요약해 줘"`라고 입력합니다.
2. AI가 `read_file`이나 `cat`으로 접근하려 할 때 `guard`가 물리적으로 차단하는지 확인합니다.
3. 다른 터미널 창에서 `guard status`를 쳐서 `DENY` 카운트가 올라갔는지 확인합니다.
4. `agy`를 종료한 뒤 `guard status`에서 세션이 깔끔하게 사라졌는지 확인합니다.

---

## 4. 이 QA를 끝까지 완주하면 무엇을 할 수 있게 되나요?

축하합니다! Step 0~8을 모두 통과했다면, 이제 여러분은 **AI 에이전트를 안심하고 실무에 투입하고 통제할 수 있는 상태**가 됩니다.

1. **역할 분리 기반 AI 위임**: 설계할 땐 `guard plan agy`, 테스트 짤 땐 `guard test agy`, 구현할 땐 `guard dev agy`로 AI의 행동 반경을 한 줄 명령으로 정확히 제한할 수 있습니다.
2. **치팅 및 우회 차단 신뢰 확보**: AI가 테스트 코드를 몰래 훔쳐보거나 셸 명령어(`cat`, `&&`, `python -c`, `>`)로 우회하려 해도 0.5ms 이내에 차단된다는 것을 직접 확인했습니다.
3. **실시간 관제 및 유연한 권한 조정**: `guard status`로 어떤 세션이 몇 번 차단됐는지 확인하고, 필요 시 `guard all`로 즉시 제한을 풀거나 `guard reset`으로 초기화할 수 있습니다.
4. **AI Skill 기반 자동 정책 생성**: YAML 문법을 외울 필요 없이 `guard doc`과 `agy` Skill을 연동해 자연어 한 마디로 프로젝트 맞춤형 보안 규칙(`.env`, 인증서, 민감 폴더 등)을 자동 생성·검증할 수 있습니다.

*(다음 Phase 4에서는 이 모든 세션을 한 화면에서 실시간 대시보드로 보며 단축키 `D/T/P/A`로 조작하는 `guard top` TUI 관제탑이 추가될 예정입니다.)*

---

## 5. 테스트 종료 후 정리 및 QA 체크리스트

QA 과정에서 만든 임시 테스트 세션을 정리하고 워크스페이스 기본 모드를 `DEV`로 복원합니다.

```bash
./guard clean --all
./guard dev
```

| 단계 | 검증 항목 | Pass / Fail | 비고 |
| :--- | :--- | :---: | :--- |
| **Step 0** | `make build` 성공 및 기본 정책(`guard-policy.yaml`) 자동 로드 | [ ] | |
| **Step 1** | 런처 배너 출력, 환경변수·PID 자동 바인딩, `exit` 시 자동 삭제 | [ ] | |
| **Step 2** | DEV 모드 테스트 파일 차단, 화이트리스트 허용, 셸/Git/스크립트 우회 차단 | [ ] | |
| **Step 3** | PLAN 모드 소스 코드 쓰기(`write_file`, `>`, `tee`, `sed -i`) 차단 및 문서 허용 | [ ] | |
| **Step 4** | TEST 모드 프로덕션 차단 및 예외 패턴(`*_test.go`, `contract/**`, `testutil/**`) 허용 | [ ] | |
| **Step 5** | 차단 이벤트 로그 확인, 실시간 모드 전환(`guard all`), 카운트 리셋(`guard reset`) | [ ] | |
| **Step 6** | `kill -9` 비정상 종료 후 `guard clean` 고아 세션 자동 수거 | [ ] | |
| **Step 7** | `guard doc` 샘플 출력, (선택) `agy` Skill 연동, `GUARD_POLICY_PATH` 커스텀 정책 로드 및 적용 | [ ] | |
| **Step 8** | (선택) 실제 `guard dev agy` 실행 및 차단 동작 확인 | [ ] | |
