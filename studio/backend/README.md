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

## 검증

```sh
go test ./...
go vet ./...
```

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

artifact 및 generation 경로는 후속 Phase에서 구현합니다.
package 목적 또는 generator를 지정한 검증 요청은 422 UNSUPPORTED_GENERATOR입니다.
설계 검증은 구조 오류를 400, 의미상 오류를 200 + valid=false + issues로 반환합니다.
JAR 참조 누락은 경고이며 파일 존재/호환성과 실제 서버 상태를 검사하지 않습니다.
프로젝트와 토폴로지 수정은 `expectedRevision`이 현재 값과 다르면 `409 REVISION_CONFLICT`를 반환합니다.
