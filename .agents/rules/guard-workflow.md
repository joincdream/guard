---
trigger: always_on
---

# Guard Handoff Workflow Rule

이 워크스페이스는 Guard 범용 핸드오프 프로토콜(`guard handoff`)을 사용합니다.

## 규칙
1. **수신 핸드오프 확인**:
   - 환경변수 `GUARD_INBOX_PATH`가 가리키는 인박스 파일 또는 `guard handoff inbox`로 전달된 `GUARD_HANDOFF_ENVELOPE` 메시지를 확인합니다.
   - 참조 문서(`References`)를 근거로 작업하며, 수정 허용 대상(`Targets`)이 지정된 경우 해당 범위 안에서만 수정합니다.
2. **작업 완료 처리 (`ack`)**:
   - 수신한 핸드오프 티켓의 작업을 완료하면 반드시 `guard handoff ack <id>`를 실행해 완료 상태로 갱신합니다.
3. **다른 세션으로 작업 위임 (`send`)**:
   - 다른 모드(`plan`, `dev`, `test`)로 작업을 넘겨야 할 때 다음 명령으로 티켓을 발행합니다:
     `guard handoff send --to <plan|dev|test|all|sessionId> --type <task|verify|fix|review> --ref <문서경로> --targets <대상경로> -m "<핵심 요약>"`
   - 이전 티켓에 대한 회신·재검증 요청 시 `--parent <이전_id>`를 함께 전달합니다.
4. **승인 권한 경계**:
   - `guard handoff approve`는 사용자 전용 명령이므로 에이전트가 직접 실행하지 않습니다.
