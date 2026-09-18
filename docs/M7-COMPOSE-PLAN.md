# M7 Docker Compose 모드 구현 계획

작성일: 2026-09-18

상태: Phase 1 로컬 구현 완료

## 목표

Studio의 VM별 Container 설계를 표준 Compose 문서로 변환하고, Crosswalk가 원격 적용 전에 수행할 작업을 안전하게 검토할 수 있게 한다.

## 책임 구조

```text
Studio Container topology
  └─ crosswalk-compose generator
       ├─ deploy.yml
       └─ compose/<server>/compose.yml
            └─ deploy compose --dry-run
```

- Phase 1은 VM 하나당 Compose project 하나를 만든다.
- Registry token 값은 파일에 기록하지 않고 `token_env` 이름만 유지한다.
- 모든 service image는 Registry host + repository + SHA-256 digest로 고정한다.
- Docker Engine과 Compose v2 plugin은 기존 설치를 전제로 하며 자동 설치하지 않는다.

## 관심사 분리

- `crosswalk/internal/config`: `compose_projects` 설정 계약과 파일·경로 검증
- `crosswalk/internal/compose`: 비밀값 없는 결정적 계획과 Phase 1 실행 차단
- `crosswalk/cmd/deploy`: `deploy compose --dry-run` 라우팅
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

- [ ] SSH 연결과 Docker Engine/Compose v2 plugin 확인
- [ ] Registry `password-stdin` 인증 재사용
- [ ] Compose 파일 checksum 비교와 권한 제한 전송
- [ ] 원격 `docker compose config --quiet` 검증
- [ ] 원격 변경 없이 preflight 결과 요약

## Phase 3 — pull·apply·rollback

- [ ] digest image pull과 사후 digest 확인
- [ ] revision 디렉터리와 active symlink 전환
- [ ] `docker compose up -d --remove-orphans`
- [ ] service health 확인과 이전 revision rollback
- [ ] 서버/project 필터, 병렬 실행과 결과 요약

## 현재 제한

- VM별 project 하나만 생성한다.
- 기본 Compose network만 사용하며 volumes, custom networks, secrets, configs, `depends_on`은 아직 모델링하지 않는다.
- Compose 파일은 생성되지만 Phase 1 Crosswalk는 원격 전송·`up`을 실행하지 않는다.

## 오류 케이스 검토

1. 미완성 apply가 실제 변경을 수행함: `--dry-run`이 없으면 Phase 1 오류로 즉시 중단한다.
2. Registry token이 plan이나 YAML에 노출됨: 환경변수 이름만 저장하고 plan에도 이름만 표시한다.
3. mutable image tag가 Compose에 포함됨: 기존 Docker 생성기의 repository/digest 검증을 재사용한다.
4. Compose project/path가 명령 또는 경로를 오염시킴: project name allowlist, 기존 local file, 절대 remote directory를 검증한다.
5. 같은 VM의 host port가 충돌함: Studio 설계 검증에서 package 생성 전에 차단한다.
