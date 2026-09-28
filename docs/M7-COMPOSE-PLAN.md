# M7 Docker Compose 모드 구현 계획

작성일: 2026-09-18

상태: Phase 1~6 구현 및 Podman Linux 실제 engine smoke 완료 · 외부 Docker VM/Registry 인수 대기

## 목표

Studio의 VM별 Container 설계를 표준 Compose 문서로 변환하고, Crosswalk가 원격 적용 전에 수행할 작업을 안전하게 검토할 수 있게 한다.

## 책임 구조

```text
Studio Container topology
  └─ crosswalk-compose generator
       ├─ deploy.yml
       └─ compose/<server>/compose.yml
            └─ deploy compose --dry-run | --preflight | --sync | --apply
```

- Phase 1은 VM 하나당 Compose project 하나를 만든다.
- Registry token 값은 파일에 기록하지 않고 `token_env` 이름만 유지한다.
- 모든 service image는 Registry host + repository + SHA-256 digest로 고정한다.
- Docker Engine과 Compose v2 plugin은 기존 설치를 전제로 하며 자동 설치하지 않는다.

## 관심사 분리

- `crosswalk/internal/config`: `compose_projects` 설정 계약과 파일·경로 검증
- `crosswalk/internal/compose`: 계획, 원격 preflight, checksum 동기화, digest 검증, revision, readiness 및 rollback
- `crosswalk/cmd/deploy`: `deploy compose --dry-run|--preflight|--sync|--apply` 라우팅
- `studio/backend/internal/generator/crosswalkcompose`: Docker 검증 결과를 Compose package로 변환
- `studio/backend/internal/service`: Compose inline 파일 저장·미리보기·ZIP packaging
- `studio/features/generation`: `crosswalk-compose` capability 및 생성기 선택

## Phase 1 — 계약·생성·안전한 dry-run

- [x] Crosswalk `compose_projects` config와 정적 검증
- [x] `deploy compose --dry-run --config deploy.yml` 계획 출력
- [x] dry-run 없는 Compose apply 명시적 차단
- [x] Studio `crosswalk-compose@1.0.0` capability·validation·generation
- [x] VM별 `compose.yml` 생성, 미리보기와 ZIP 다운로드
- [x] repository+digest image, port, environment, restart 변환
- [x] Crosswalk·Studio 단위, 계약, API lifecycle 테스트

## Phase 2 — 원격 preflight와 파일 동기화

- [x] SSH 연결과 Docker Engine/Compose v2 plugin 확인
- [x] Registry `password-stdin` 인증 재사용
- [x] Compose 파일 checksum 비교와 권한 제한 전송
- [x] 원격 `docker compose config --quiet` 검증
- [x] workload/file 변경 없는 preflight과 서버별 결과 요약

`--preflight`는 Compose 파일과 container를 변경하지 않지만 `docker login`이 SSH 사용자의 Docker credential 설정을 갱신할 수 있다. `--sync`는 명시적으로 선택한 경우에만 원격 디렉터리를 `0700`, Compose 파일을 `0600`으로 유지하며 checksum이 다를 때만 전송한다. 전송 후 config 검증에 실패해도 workload는 적용하지 않는다.

## Phase 3 — pull·apply·rollback

- [x] digest image pull과 사후 RepoDigests 확인
- [x] checksum revision 디렉터리와 active symlink 전환
- [x] `docker compose up -d --remove-orphans --pull never --no-build`
- [x] service health 확인과 이전 revision rollback
- [x] 서버/project 필터, 제한 병렬 실행과 결과 요약

`--apply`는 로컬 파일이 생성기가 지원하는 `name`, `services[].image/ports/environment/restart` 구조인지 확인한다. image는 설정된 registry의 SHA-256 digest로 고정하고 Compose 변수 보간 및 추가 문서를 거부한다. SSH 접근·Docker/Compose·Registry 인증 후, `.revisions/<sha256>/compose.yml`을 제한된 권한으로 저장하고 원격 `config --quiet`, 서비스·image 일치 및 image digest를 확인한다. `active`는 리비전 디렉터리를 가리키는 상대 symlink이며 `up` 전에 원자적으로 전환한다. 모든 service가 running이고 image HEALTHCHECK가 있으면 healthy여야 성공이다(기본 60초). 실패 시 이전 리비전으로 symlink를 되돌리고 `up` 및 상태 검증을 다시 실행한다. 첫 배포 실패 시 새 stack을 내리고 `active`를 제거한다. 롤백 자체가 실패하면 오류를 반환하고 상태 점검이 필요하다. 파일은 자동 삭제하지 않는다.

동일 서버/project 동시 변경은 `.crosswalk-lock`으로 차단하며 비정상 종료로 lock이 남으면 다른 배포가 없는지 확인한 후 운영자가 정리한다. 최초 적용 시 같은 Compose project label의 미관리 컨테이너가 있으면 적용을 거부한다. `--server-tag`, `--project`, `--parallel`은 dry-run/preflight/sync/apply에 공통 적용한다. 같은 서버의 여러 project는 순차 처리되고 다른 서버는 병렬 처리된다.

## Phase 4 — 격리 통합 검증과 운영 인수

- [x] 실제 SSH session과 SFTP 전송을 사용하는 loopback fixture
- [x] Docker Engine·Compose v2·Registry 응답을 재현하는 상태 기반 fake CLI
- [x] 신규 적용, 동일 revision 재실행 skip, 정상 업그레이드 검증
- [x] unhealthy 업그레이드 실패와 이전 revision 재적용·health 복구 검증
- [x] 반복 실행 가능한 전용 `make test-compose-fixture` target
- [ ] 격리 Linux VM과 전용 Registry에서 실제 Docker Compose smoke

fixture는 제품의 SSH client와 SFTP 전송 경로, 원격 파일 권한·checksum revision·symlink 전환 및 rollback 명령을 실제로 실행한다. Docker daemon과 Registry만 상태 기반 CLI로 대체하므로 외부 image 또는 credential 없이 결정적으로 반복할 수 있다. 실제 daemon의 pull, Compose network/container 동작과 배포 대상 Linux의 명령 호환성은 [M7 운영 smoke runbook](M7-COMPOSE-SMOKE.md)으로 별도 확인한다.

## Phase 5 — 실제 smoke 자동화

- [x] 외부 base image가 필요 없는 `scratch` 기반 정적 health fixture
- [x] amd64/arm64 fixture build와 전용 Registry push·manifest digest 추출
- [x] 필수 격리 project/remote directory 검증과 기존 경로 재사용 거부
- [x] dry-run → preflight → 신규 적용 → 재실행 → 업그레이드 → unhealthy rollback 자동 검증
- [x] 임시 Docker credential 격리 및 종료 시 credential directory 제거
- [x] 원격 revision 자동 삭제 금지와 진단 로그 보존
- [ ] 승인된 Linux VM과 Registry를 지정해 실제 실행 결과 기록

`make smoke-compose-real`은 명시적인 `SMOKE_*` 환경변수가 모두 제공된 경우에만 실행된다. 새 timestamp tag의 fixture image를 전용 Registry에 push하고 원격 workload를 변경하므로 일반 테스트 target에는 포함하지 않는다. 대상 remote directory가 이미 존재하면 삭제하거나 재사용하지 않고 즉시 중단한다.

## Phase 6 — 로컬 Linux 실제 engine 검증

- [x] Podman Linux VM에 공식 Docker Compose v2.29.7 provider checksum 검증 설치
- [x] 실제 Registry v2 container에 `scratch` health image build·push
- [x] 포트 포함 Registry image 계약과 RepoDigest 검증
- [x] 실제 container 신규 생성과 동일 revision 재실행 skip
- [x] v2 revision 교체와 unhealthy revision의 이전 revision 복구
- [x] Podman provider 안내 출력 비활성화로 구조화 출력 보호
- [x] 성공 결과 확인 후 전용 workload/container/network/revision 정리
- [ ] 외부 Docker Engine VM과 조직 Registry에서 최종 인수

2026-09-23 `podman-machine-default`의 Linux arm64/Podman 5.5.2, 공식 Compose v2.29.7, 임시 Registry v2 조합에서 전체 자동화가 통과했다. project `crosswalk-smoke-podman-20260923035831`은 신규 적용, skip, v2 적용, unhealthy 실패와 v2 rollback을 완료했다. 확인 후 해당 container/network/revision, 임시 Compose provider와 insecure Registry 설정을 제거하고 VM을 정지했다. Registry image와 빌드 layer cache만 재실행 용도로 남겼다.

## 현재 제한

- VM별 project 하나만 생성한다.
- 기본 Compose network만 사용하며 volumes, custom networks, secrets, configs, `depends_on`은 아직 모델링하지 않는다.
- `--sync`는 기존 단일 파일을 동기화·검증하는 모드이고 workload를 변경하지 않는다. `--apply`는 별도 checksum revision을 생성한다. 실제 Docker VM/Registry에서의 smoke는 아직 실행하지 않았다.

## 오류 케이스 검토

1. 의도치 않은 apply: 실행 mode가 없으면 중단하고 `--sync`도 `pull`/`up`을 호출하지 않는다. `--apply`만 workload를 변경한다.
2. Registry token이 plan이나 YAML에 노출됨: 환경변수 이름만 저장하고 plan에도 이름만 표시한다.
3. mutable image tag가 Compose에 포함됨: 기존 Docker 생성기의 repository/digest 검증을 재사용한다.
4. Compose project/path가 명령 또는 경로를 오염시킴: project name allowlist, 기존 local file, 절대 remote directory를 검증한다.
5. 같은 VM의 host port가 충돌함: Studio 설계 검증에서 package 생성 전에 차단한다.
6. 동일한 Compose 파일을 매번 전송함: 로컬/원격 SHA-256가 같으면 skip하고 요약에 `skipped`로 표시한다.
7. 잘못된 Compose 파일이 적용됨: 로컬 지원 필드·SHA-256, 원격 파일 checksum·`config --quiet`·서비스와 image 목록을 검증한 뒤에만 active를 전환한다.
8. 새 revision이 unhealthy: 이전 revision으로 복원하고 health를 재검증한다. 첫 적용 실패는 stack을 내린다.
9. 이미 사용 중인 Compose project: Crosswalk active 링크가 없고 같은 project label을 가진 컨테이너가 있으면 중단한다.
