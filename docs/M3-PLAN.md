# M3 Docker 모드 구현 계획

작성일: 2026-09-17

상태: Phase 1~4 완료, Phase 5 로컬 구현 완료·실환경 smoke 대기

## 목표

`deploy docker --config deploy.yml`이 SSH 대상 서버의 Docker 컨테이너를 digest 기반 desired state로 배포한다.
Registry 인증, image 확인, pull, container 재조정은 서로 분리하고 dry-run은 원격 연결이나 변경 없이 같은 계획을 보여준다.

Docker Engine 설치는 기본 동작에 포함하지 않는다. 운영체제·repository·daemon 권한 정책이 환경마다 다르므로,
기존 `bootstrap`에 사용자가 명시한 설치 정책이 있는 경우에만 실행하고 Docker 모드는 항상 engine preflight부터 시작한다.

## 설정 계약

```yaml
docker:
  registry:
    host: harbor.example.com
    username: robot$deployer
    token_env: HARBOR_TOKEN

servers:
  - name: app-1
    host: 10.0.0.10
    tags: [prod, wave1]
    containers:
      - name: commerce-api
        tags: [api, wave1]
        image: team/commerce-api
        digest: sha256:<64 lowercase hex characters>
        restart: unless-stopped
        ports:
          - host: 8080
            container: 8080
            protocol: tcp
        environment:
          SPRING_PROFILES_ACTIVE: prod
```

- Registry token 값은 YAML에 저장하지 않고 `token_env`가 가리키는 실행 환경 변수에서만 읽는다.
- Image tag는 허용하지 않고 repository와 SHA-256 digest를 별도 필드로 고정한다.
- Dry-run과 오류 로그에는 환경변수 이름만 표시하고 값은 출력하지 않는다.
- 같은 서버의 container name과 host port/protocol 조합은 중복될 수 없다.
- restart 기본값은 `unless-stopped`다.

## 관심사 분리

```text
internal/config/            # YAML 모델, 정규화, 정적 검증
internal/docker/plan.go       # 비밀값 없는 결정적 배포 계획
internal/docker/runner.go     # 실행 모드 경계와 서버별 오케스트레이션
internal/docker/preflight.go  # Docker daemon/권한 확인
internal/docker/registry.go   # password-stdin 인증
internal/docker/inspect.go    # container image ID와 local RepoDigest 조회
internal/docker/pull.go       # 조건부 digest pull과 사후 검증
internal/docker/spec.go       # 비밀값 비노출 canonical spec 비교
internal/docker/health.go     # running/HEALTHCHECK readiness 확인
internal/docker/reconcile.go  # container 생성, 보존형 교체와 rollback
cmd/deploy                  # CLI flag와 의존성 조립
```

## 구현 단계

### Phase 1 — 계약과 안전한 dry-run

- [x] registry/container 설정 모델과 Docker-only server 허용
- [x] container name, repository, digest, restart, port, environment key 검증
- [x] digest 고정 image reference와 비밀값 없는 계획 생성
- [x] `deploy docker --dry-run --config ...` 라우팅
- [x] 실제 apply 요청은 명시적으로 거부
- [x] 단위·CLI 회귀 테스트와 예시 설정

완료 기준: 네트워크 없이 plan 생성, 환경변수 값 비노출, 잘못된 digest·중복 port·구형 설정 거부.

### Phase 2 — 원격 preflight와 Registry 인증

- [x] `--preflight`에서 SSH 연결과 `docker version`/daemon 접근 권한 확인
- [x] Docker 미설치 상태는 설치 명령을 추측하지 않고 actionable error 반환
- [x] token 환경변수 존재 확인 후 `docker login --password-stdin` 사용
- [x] token, login 출력과 원격 명령 오류에서 credential redaction
- [x] dry-run에서는 SSH 연결과 token 조회를 수행하지 않음

`--preflight`는 image pull, container 생성·중지·삭제를 수행하지 않는다. 다만 `docker login` 자체의 정상 동작에 따라
대상 SSH 사용자의 Docker credential 파일 또는 credential store가 갱신될 수 있다. 순수 읽기 검토에는 `--dry-run`을 사용한다.

완료 기준: mock SSH 경계에서 token이 명령·로그·오류에 나타나지 않고, token 누락·daemon 접근 실패·Registry 인증 실패가 단계별로 중단된다.

### Phase 3 — digest 상태 판정과 pull

- [x] `--pull`에서 실행 중 container의 image ID와 RepoDigest 조회
- [x] 원하는 digest가 로컬에 있으면 pull skip
- [x] digest-pinned pull 이후 local RepoDigests 재검증
- [x] tag 입력과 pull 결과 digest 불일치 시 배포 중단

`--pull`은 Phase 2의 daemon/Registry preflight를 먼저 수행하고 image cache만 변경한다. container 생성·중지·삭제·교체는
수행하지 않는다. Phase 3 시점에는 옵션 없는 `deploy docker`를 Phase 4 reconcile 완성 전까지 거부했다.

완료 기준: local desired digest는 pull 없이 건너뛰고, missing digest만 exact `repository@sha256`로 pull하며, 사후 RepoDigest가 다르면 중단한다.

### Phase 4 — container reconcile

- [x] port/environment/restart/image로 메모리 내 canonical spec hash 계산
- [x] 동일 spec이며 healthy인 container는 replacement skip
- [x] 다른 상태는 기존 container를 backup 이름으로 보존한 뒤 교체
- [x] running/HEALTHCHECK 확인과 실패 시 이전 container 복구
- [x] environment 값은 명령 인자·label 대신 권한 600 임시 env-file에 stdin 전달

옵션 없는 `deploy docker --config ...`가 preflight → digest 준비 → reconcile 전체 흐름을 실행한다.
`health_timeout_sec` 기본값은 60초이며 image에 HEALTHCHECK가 있으면 `healthy`, 없으면 `running`을 완료 조건으로 사용한다.
환경변수 값은 canonical 비교 메모리에만 존재하며 Docker label에는 Crosswalk 관리 여부와 환경변수 key 목록만 기록한다.

동일 host port를 새 container와 동시에 바인딩할 수 없으므로 현재 교체는 무중단 rolling 배포가 아니다. 기존 container를 중지하고
`<name>-crosswalk-backup`으로 보존한 뒤 새 container가 정상화되면 backup을 삭제한다. 실패하면 새 container를 제거하고 이전 이름과
실행 상태를 복구한다. 이미 backup 이름이 존재하면 운영자 확인 없이 덮어쓰거나 삭제하지 않고 교체 전에 중단한다.

완료 기준: 최초 생성, 무변경 재실행, spec 변경 교체, unhealthy 실패 rollback이 mock Docker 경계에서 검증되고 secret이 명령·label·오류에 나타나지 않는다.

### Phase 5 — 운영 완성

- [x] server/container tag filter와 제한된 병렬 실행
- [x] 서버별 결정적 done/skipped/failed 요약과 복수 실패 집계
- [x] mock Docker runner에서 최초 배포·무변경 재실행·digest 변경·실패 rollback 검증
- [x] 실제 SSH/Harbor 검증 절차와 rollback 운영 runbook
- [ ] 지정된 Podman/Docker SSH fixture와 Harbor에서 smoke 결과 기록

필터는 OR 조건으로 동작하며 일치 대상이 없으면 오류를 반환한다. `--parallel`은 서버 단위 worker 수만 제한하고 한 서버 안의
container 순서는 유지한다. 병렬 완료 순서와 관계없이 summary는 설정 파일의 서버 순서로 출력한다.

실환경 smoke는 원격 credential store, image cache와 container 상태를 변경하므로 구체적인 fixture/Registry가 지정된 후 별도로 실행한다.

## 오류 케이스

1. tag가 같은데 image 내용이 바뀜: tag 입력을 금지하고 digest를 필수화한다.
2. dry-run 로그에 Secret이 노출됨: plan은 environment key와 token 환경변수 이름만 보존한다.
3. Docker가 없거나 daemon 권한이 없음: 자동 설치를 추측하지 않고 preflight에서 중단한다.
4. 같은 host port를 여러 container가 사용함: 설정 로드 단계에서 server 범위 충돌로 거부한다.
5. pull 이후 digest가 요청값과 다름: local RepoDigests를 재검증하고 container 변경 전에 중단한다.
6. 새 container가 기동하지 못함: backup으로 보존한 기존 container의 이름과 이전 실행 상태를 복구한다.
