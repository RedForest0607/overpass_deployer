# M4-PLAN.md — UX, Reliability & Dashboard 체크리스트

마일스톤 M4의 상세 구현 체크리스트입니다.
M4는 운영자가 배포 전후 상태를 안전하게 확인하고, 필요한 서버/앱만 제어할 수 있는 UX와 안정성 기능을 다룹니다.

---

## 상태 범례

```
[ ] 미시작
[~] 진행 중
[x] 완료
[!] 블로킹 이슈
```

---

## 완료 기준

아래 조건을 모두 만족해야 M4 완료입니다.

- [x] `deploy dashboard --config deploy.yml` 명령이 실행된다
- [x] 대시보드가 `deploy.yml`의 서버/앱 목록을 기준으로 상태를 조회한다
- [x] 앱 상태, PID, 포트, 상세 메시지가 표 형태로 표시된다
- [x] 선택 앱의 로그를 `tail -n 200 -F` 방식으로 스트리밍한다
- [x] 앱 start/stop/restart 실행 전에 확인 모달을 표시한다
- [x] 선택 앱 dry-run/실배포 실행 전에 확인 모달을 표시한다
- [x] 대시보드 실행 시 배포 원본 파일이 없어도 monitoring 전용 검증 경로로 설정을 읽을 수 있다
- [x] 로그 stream 종료 시 원격 tail 프로세스가 남지 않도록 세션 종료를 처리한다
- [~] `script.mode: local-file` 앱의 상태 판별 규약을 실제 운영 환경 기준으로 확정한다
- [x] 앱별 process match/log path override 스키마를 확정하고 구현한다
- [ ] 서버/앱별 실행 결과 요약 테이블을 제공한다
- [ ] SSH 연결 실패/타임아웃/부분 실패를 대시보드에서 재시도 가능한 형태로 보여준다
- [ ] 명시적 target/service 선택 UX와 기존 tag 기반 선택 배포의 관계를 문서화한다
- [ ] M3 Docker 모드 구현 후 VM/Docker 공통 UX 범위를 재점검한다

---

## Phase 1 — Dashboard 진입점

- [x] `cmd/deploy`에 `dashboard` 서브커맨드 추가
- [x] `--config` 플래그 지원
- [x] monitoring 전용 설정 로더(`LoadForMonitoring`) 연결
- [x] CLI 라우팅 테스트 추가

---

## Phase 2 — 상태 조회와 로그

- [x] `internal/monitor` 서비스 추가
- [x] 서버별 SSH 연결 후 앱 상태 read-only 조회
- [x] 실제 `java` 실행 파일의 명령행에서 앱별 literal process match를 찾는 PID 조회
- [x] 기본 로그 경로 `{base_dir}/logs/{app.name}.log` 사용
- [x] 로그 스트리밍용 SSH session 시작/종료 처리
- [~] local-file 스크립트나 커스텀 프로세스명 앱에서 상태 판별 규약 검증
- [x] `monitor.process_match`/`monitor.log_path` override 지원

---

## Phase 3 — 제어와 선택 배포

- [x] start/stop/restart 허용 action만 실행
- [x] 선택 앱 단위 dry-run 실행
- [x] 선택 앱 단위 실배포 실행
- [x] 선택 배포 시 bastion 설정을 변경하지 않도록 분리
- [x] 제어/배포 전 확인 모달 표시
- [x] 확인창이 열린 동안 전역 단축키 차단
- [ ] 실패 결과를 앱/서버 단위로 축약해 재시도 판단을 쉽게 표시

---

## Phase 4 — 운영 검증

- [x] loopback mock SSH 기반 상태 조회/제어/로그 stream 테스트
- [x] 전체 Go 테스트 통과
- [x] DEV bastion에서 `oa` 상태 조회와 로그 tail stream 검증
- [x] 대시보드 종료 후 원격 tail 미잔류 확인
- [~] DEV read-only 계정/환경에서 custom 앱 규약 추가 검증
- [ ] PRD read-only 환경에서 상태 조회만 선검증

---

## 이슈 로그

| 날짜 | 내용 | 상태 |
|---|---|---|
| 2026-07-31 | `deploy dashboard` TUI, 상태 조회, 로그 stream, 앱 제어/배포 확인창 구현 | 1차 완료 |
| 2026-07-31 | DEV bastion에서 `oa` 상태 조회와 로그 stream 확인 | read-only 검증 완료 |
| 2026-08-24 | 대시보드를 공식 M4 마일스톤으로 편입 | 진행 중 |
| 2026-09-17 | 앱별 process match/log path override 및 read-only 로그 조회 구현 | 로컬 검증 완료 |
