# Deployer

배포 설계와 실행을 두 산출물로 제공하는 상위 프로젝트입니다.

| 산출물 | 디렉터리 | 책임 |
| --- | --- | --- |
| Studio | `studio/` | 프로젝트·토폴로지 편집, 검증, 배포 차트·패키지 생성 |
| Studio Backend | `studio/backend/` | Studio API, SQLite 메타데이터, 아티팩트·패키지 저장 |
| Crosswalk | `crosswalk/` | 배포 차트 검증, SSH 파일 전송, VM 배포·모니터링 |

Studio → 배포 차트와 자산 → Crosswalk

현재 Studio는 프론트 설계 초안만 생성하며, Crosswalk와의 완전한 패키지 연동은 아직 구현되지 않았습니다.
Crosswalk의 기존 실행 파일명 `deploy`, 설정 파일명 `deploy.yml`, Go 모듈명 `go-deployer`는 호환성을 위해 유지합니다.

## 실행

저장소 루트에서:

```sh
# Crosswalk — 로컬 버전 확인
go -C crosswalk run ./cmd/deploy version

# Studio — 로컬 프론트
cd studio
npm run dev -- --host 127.0.0.1

# Studio Backend — 별도 터미널에서 로컬 API
go -C studio/backend run ./cmd/studio-api --data-dir ./.studio-data
```

Studio 의존성이 없는 환경에서는 승인을 거쳐 `npm ci`로 설치합니다.
Studio는 로컬 실행 전용이며 외부 게시가 필요하지 않습니다.

프런트와 백엔드를 함께 실행하려면 `make dev-studio-local`, 연결 상태를 확인하려면 다른 터미널에서
`make smoke-studio-local`을 실행합니다. 상세 설정은 [Studio 로컬 개발 환경](docs/STUDIO-LOCAL-DEVELOPMENT.md)을 참고합니다.

## 검증과 빌드

저장소 루트에서:

```sh
make test-crosswalk
make test-studio
make test-studio-backend
make build-crosswalk
make build-studio
make build-studio-backend
```

Crosswalk 빌드 결과는 `crosswalk/bin/deploy`, Studio 빌드 결과는 `studio/dist/`,
Studio Backend 빌드 결과는 `studio/backend/bin/studio-api`입니다.
네트워크·서버 접근을 수반하는 Podman/SSH 통합 검증은 위 테스트에 포함하지 않습니다.

## 경계와 경로

- 각 산출물은 별도 의존성 및 빌드 과정을 사용합니다.
- Studio Backend는 현재 loopback 전용 Phase 0~2 구현이며 프로젝트와 환경별 토폴로지를 SQLite에 저장합니다.
- Studio가 Crosswalk의 SSH 실행 코드를 직접 호출하지 않습니다.
- 차트 규격은 소비자인 Crosswalk가 기준이며, 버전 규격과 생성물 호환성 테스트는 후속 작업입니다.
- `docs/`, `PLAN.md`, `PROGRESS.md`는 프로젝트 공통 문서입니다.
- `ops/`, `infra/`, `TEST/` 및 기존 루트 빌드 자산은 이동하거나 삭제하지 않았습니다.
- 기존 문서의 루트 `go run ./cmd/deploy` 예제는 이제 `crosswalk/`에서 실행합니다.
- 운영 설정에 상대 자산 경로가 있으면 기존 실행 디렉터리를 유지하고 빌드된 바이너리를 실행하세요. 디렉터리 이동에 맞춰 운영 경로를 일괄 변경하지 않습니다.

상세 사용법: [Crosswalk](crosswalk/README.md), [Studio](studio/README.md).

백엔드 연동 계약: [Studio API 명세 초안](docs/studio-api.md).
