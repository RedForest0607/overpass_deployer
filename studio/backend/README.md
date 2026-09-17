# Studio Backend

Studio의 로컬 API 서버입니다. API 계약, SQLite migration, 공통 HTTP 보호,
프로젝트 및 환경별 토폴로지 저장과 서버 설계 검증을 제공합니다.

## 실행

데이터 경로는 명시적으로 지정해야 하며 기본 listen 주소는 `127.0.0.1:8080`입니다.

```sh
go run ./cmd/studio-api --data-dir ./.local-data
```

브라우저 origin 기본 허용값은 `http://127.0.0.1:3000,http://localhost:3000`입니다.
다른 로컬 origin은 `--allowed-origins` 또는 `STUDIO_ALLOWED_ORIGINS`로 지정합니다.
공개 주소 bind는 현재 차단합니다.

기본 인증 모드는 로컬 개발용 `local`이며 모든 데이터 소유자는 `local-user`입니다. 사내 공유 환경에서는 TLS/SSO reverse proxy 뒤에서 다음 설정을 사용합니다.

```sh
STUDIO_AUTH_MODE=trusted-header \
STUDIO_AUTH_PROXY_SECRET='replace-with-at-least-32-random-base64url-characters' \
STUDIO_ALLOWED_ORIGINS='https://studio.example.com' \
go run ./cmd/studio-api --data-dir ./.local-data
```

proxy는 `X-Studio-Authenticated-User`와 `X-Studio-Proxy-Secret`을 외부 요청에서 제거한 뒤 검증 결과로 다시 설정해야 합니다. 사용자 헤더 이름은 `STUDIO_AUTH_HEADER`, secret 헤더 이름은 `STUDIO_AUTH_PROXY_SECRET_HEADER`로 변경할 수 있습니다. `/healthz` 외의 API는 인증되며 모든 저장소 접근은 요청 사용자 owner로 격리됩니다. 참조 구성과 롤백 절차는 `../deploy/README.md`를 참고합니다.

## 검증

```sh
cd ..
npm run check:api-generated
cd backend
go test ./...
go test -race ./...
go vet ./...
```

request DTO는 `../schemas/openapi-v1.yaml`에서 `../scripts/generate-api-contracts.mjs`로 생성된다.
계약을 변경한 뒤 `npm run generate:api`를 실행하며, 생성물을 직접 편집하지 않는다.

## 운영 백업과 복구

`studio-admin`은 실행 중인 SQLite의 일관된 snapshot과 immutable artifact/package를 checksum manifest가 포함된 `tar.gz`로 저장합니다. 백업 파일과 복구 대상은 기존 경로를 덮어쓰지 않습니다.

```sh
go run ./cmd/studio-admin inspect --data-dir ./.local-data
go run ./cmd/studio-admin backup --data-dir ./.local-data --output ./studio-backup.tar.gz
go run ./cmd/studio-admin verify --input ./studio-backup.tar.gz
go run ./cmd/studio-admin restore --input ./studio-backup.tar.gz --data-dir ./.restored-data
```

복구가 완료되면 새 data directory로 API를 기동해 프로젝트, artifact와 package 다운로드를 확인한 뒤 경로를 전환합니다. 상세 절차는 `../../docs/STUDIO-OPERATIONS.md`를 참고합니다.

## 현재 API

- `GET /healthz`
- `GET /api/v1/capabilities`
- `GET /api/v1/projects`
- `POST /api/v1/projects`
- `GET /api/v1/projects/{projectId}`
- `PATCH /api/v1/projects/{projectId}`
- `GET /api/v1/projects/{projectId}/topologies/{environment}`
- `PUT /api/v1/projects/{projectId}/topologies/{environment}`
- `POST /api/v1/projects/{projectId}/validations` — purpose=design
- `GET/POST /api/v1/projects/{projectId}/artifacts`
- `GET /api/v1/projects/{projectId}/artifacts/{artifactId}/content` — 프로젝트 범위 YAML/XML 편집 원문
- `POST /api/v1/projects/{projectId}/generations`
- `GET /api/v1/projects/{projectId}/generations/{generationId}`
- `GET /api/v1/projects/{projectId}/packages/{packageId}`
- `GET /api/v1/projects/{projectId}/packages/{packageId}/files/{fileId}`
- `GET /api/v1/projects/{projectId}/packages/{packageId}/download`

artifact와 Crosswalk generation/package 경로를 지원합니다. 생성 요청은 immutable snapshot과 함께
`queued`로 저장한 뒤 202를 반환하고, 프로세스 내 worker가 비동기로 package를 생성합니다.
서버 재시작 시 중단된 작업은 최대 3회 범위에서 복구하며 같은 Idempotency-Key와 동일 요청은 기존 결과를 반환합니다.
DB에서 참조하지 않고 24시간 이상 지난 content-addressed blob과 업로드 임시 파일은 시작 시 정리합니다.
package 목적 검증은 저장 없이 artifact 참조와 생성기 입력을 검사합니다.
`crosswalk-vm`은 Java 실행 파일은 `jar`, 압축 해제 파일은 `archive`, 설정·스크립트·values는
각 용도에 맞는 kind인지 생성 시 다시 확인합니다.
설계 검증은 구조 오류를 400, 의미상 오류를 200 + valid=false + issues로 반환합니다.
JAR 참조 누락은 경고이며 파일 존재/호환성과 실제 서버 상태를 검사하지 않습니다.
프로젝트와 토폴로지 수정은 `expectedRevision`이 현재 값과 다르면 `409 REVISION_CONFLICT`를 반환합니다.

## Crosswalk 생성기 core

`internal/generator/crosswalkvm`은 VM별 bootstrap, 서버/utility 파일, Java JAR·설정·추가 파일,
archive extract, chmod, JVM/script values, bastion 설정을 `deploy.yml`로 변환합니다. 실제 artifact는
안전한 ZIP 상대 경로에 배치하며 경로 탈출·충돌과 패키징 중 크기/SHA-256 변경을 거부합니다.
업로드 artifact와 생성 package는 data directory의 content-addressed blob으로 보관합니다.
VM에 배치된 hosted Redis/Elasticsearch 노드는 Crosswalk utility로 변환합니다. 외부 관리형 노드는
배포 대상이 아니며, utility는 제품 설치기가 아니라 선언된 디렉터리·파일·압축·systemd 상태를 적용합니다.
구성 예시는 `../../docs/STUDIO-CROSSWALK-UTILITIES.md`를 참고합니다.
