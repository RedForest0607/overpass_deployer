# PLAN.md

전체 마일스톤 진행 추적 문서입니다.
마일스톤별 상세 체크리스트는 `docs/M{N}-PLAN.md`를 참고합니다.

---

## 상태 범례

```
[ ] 미시작
[~] 진행 중
[x] 완료
[!] 블로킹 이슈
```

---

## 마일스톤 전체 현황

| 마일스톤 | 제목 | 상태 | 상세 |
|---|---|---|---|
| M1 | VM 모드 MVP | [x] | `docs/M1-PLAN.md` |
| M2 | VM 모드 완성 (멱등성) | [ ] | `docs/M2-PLAN.md` |
| M3 | Docker 모드 | [ ] | `docs/M3-PLAN.md` |
| M4 | UX & Reliability | [ ] | `docs/M4-PLAN.md` |
| M5 | AWS Extension | [ ] | `docs/M5-PLAN.md` |

---

## M1 — VM 모드 MVP

**목표:** `deploy vm --config deploy.yml` 또는 `deploy vm --dry-run --config deploy.yml`으로 VM 1대 배포 또는 예정 작업 확인이 가능하다.

**범위:**
- SSH Key 인증 연결
- `--dry-run` 플래그로 변경 없이 예상 동작 출력
- 디렉토리 구조 자동 생성
- jar SCP 전송 (SHA256 비교로 skip)
- 설정 파일 배포 (application.yml, logback.xml)
- server.sh 템플릿 렌더링 및 배포
- 단계별 구조화 로그 출력

**완료 기준:** `docs/M1-PLAN.md` 체크리스트 전체 통과
**상세 규칙:** `docs/M1-AGENT.md`
**설계 가이드:** `docs/M1-guide.md`
**현재 상태:** [x] 핵심 배포 경로, dry-run, Podman 기반 통합 검증 시나리오 1~5가 완료되었고, 체크리스트도 현재 코드 기준으로 정리됨. 일부 확장 기능은 M2/M4 기반 작업으로 유지

---

## M2 — VM 모드 완성 (멱등성)

**목표:** 같은 서버에 몇 번을 실행해도 안전하고, 실제 운영 환경에서 쓸 수 있는 수준이 된다.

**범위:**
- 각 단계 사전 상태 확인 후 skip (디렉토리, 프로세스, 파일)
- OS 판별 분기 처리 (Ubuntu / CentOS)
- JDK 자동 설치 (없을 경우)
- 유틸 서비스 설정 파일 배포 (`utils[]` — Redis, ES, Kibana, 모니터링 툴)
- yml 유효성 검증 강화
- 실행 결과 요약 출력 (서버별 skip / done / failed)

**완료 기준:** `docs/M2-PLAN.md` 체크리스트 전체 통과
**선행 조건:** M1 완료

---

## M3 — Docker 모드

**목표:** `deploy docker --config deploy.yml` 로 Harbor에서 이미지를 pull해 컨테이너로 실행할 수 있다.

**범위:**
- `deploy docker` 서브커맨드 구현
- Harbor 토큰 인증 + docker login
- Docker 설치 확인 / 자동 설치
- docker pull (digest 비교로 skip)
- docker run (포트 매핑, 환경변수, restart 정책)
- 컨테이너 상태 확인 후 skip

**완료 기준:** `docs/M3-PLAN.md` 체크리스트 전체 통과
**선행 조건:** M1 완료 (SSH / SCP / Config 공유 레이어 재사용)

---

## M4 — UX & Reliability

**목표:** 여러 VM에 동시에 배포하고, 실행 전에 무엇을 할지 미리 확인할 수 있다.

**범위:**
- 병렬 실행 (goroutine) + 서버별 strategy override
- `--target` 플래그 — 특정 서버만 선택 실행
- `--service` 플래그 — 특정 서비스만 선택 실행
- SSH 연결 타임아웃 및 재시도
- 실행 결과 요약 테이블 출력
- known_hosts 기반 SSH 검증

**완료 기준:** `docs/M4-PLAN.md` 체크리스트 전체 통과
**선행 조건:** M2, M3 완료

---

## M5 — AWS Extension

**목표:** ECR에서 이미지를 가져오고, ECS에서 컨테이너를 실행할 수 있다.

**범위:**
- ECR 지원 (`registry.type: ecr`, aws-sdk-go-v2)
- IAM Role / Access Key 기반 인증
- ECR 로그인 토큰 자동 갱신
- ECS Task Definition 자동 생성 / 업데이트
- ECS Service 생성 및 Desired Count 설정
- AWS 리소스 프로비저닝 (범위는 Terraform과 역할 분담 정의 후 확정)

**완료 기준:** `docs/M5-PLAN.md` 체크리스트 전체 통과
**선행 조건:** M3 완료
**주의:** VPC / IAM / 네트워크 레벨 리소스 생성은 Terraform과 역할 분담 먼저 결정 필요

---

## 마일스톤 간 의존 관계

```
M1 (VM MVP)
 ├─→ M2 (VM 멱등성)     ← VM 모드 심화
 ├─→ M3 (Docker 모드)   ← M1 공유 레이어 재사용
 │
 M2 ──┐
 M3 ──┼─→ M4 (UX)       ← 두 모드 모두 완성 후
      │
 M3 ──┴─→ M5 (AWS)      ← Docker 모드 기반 확장
```

---

## 이슈 및 의사결정 로그

> 마일스톤 진행 중 발생한 주요 의사결정과 블로킹 이슈를 기록합니다.

| 날짜 | 마일스톤 | 내용 | 결정 |
|---|---|---|---|
| 2026-04-06 | M1 | 코드 기준 진행도 재점검 | M1을 `[~]`로 조정. 핵심 VM 배포 경로는 구현되었고, 남은 작업은 체크리스트 정리, 통합 검증, 일부 CLI/테스트 갭으로 한정 |
| 2026-04-06 | M4 기반 작업 | SSH host key/known_hosts, bastion alias 동기화가 선반영됨 | M1 완료 전까지는 “선행 기반 작업”으로 간주하고 공식 마일스톤 상태는 올리지 않음 |
| 2026-04-06 | M1 | dry-run 예정 작업 출력 기능을 M1 범위로 승격 | CLI와 VM/scp/bastion 경계에서 원격 변경 없이 계획 로그만 출력하도록 정리 |
| 2026-04-06 | M1 | Podman 기반 TEST 이미지로 통합 검증 완료 | happy path, 원격 파일 미존재 자동 전송, dry-run, SSH 실패, bastion alias/known_hosts 등록까지 확인 |
| 2026-04-13 | M1 | 체크리스트를 현재 코드 기준으로 재정렬 | obsolete 항목을 실제 구현 형태로 정리하고 M1 상태를 `[x]`로 승격 |
