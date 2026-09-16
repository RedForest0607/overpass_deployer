# Studio Backend 구현 계획

작성일: 2026-09-10

상태: 진행 중

관련 계약: `docs/studio-api.md`

## 구현 현황

- [~] Phase 0: OpenAPI 3.1과 Blueprint v1 JSON Schema 추가. Crosswalk 차트 version/validation 경계와 타입 생성은 미완료.
- [x] Phase 1: Go 서버 골격, SQLite migration, 공통 오류/middleware, health/capabilities, loopback 제한 구현.
- [x] Phase 2: 프로젝트 CRUD, cursor 목록, 환경별 topology revision CAS 저장, React Flow API adapter 구현.
- [x] Phase 3: 서버 design/package 검증, HTTP API·프론트 오류 연결, Crosswalk 배포 상세 구조 및 artifact/generator issue 구현.
- [x] Phase 4: immutable artifact streaming 업로드·목록, content-addressed 저장, JAR/ZIP·UTF-8·archive 검증 구현. 프론트 선택 UI는 Phase 6에 포함.
- [x] Phase 5: Crosswalk deploy.yml renderer, 실제 artifact ZIP 배치, 용도별 artifact kind 재검증, persistent 비동기 worker/recovery, idempotent generation, package 메타데이터·preview·download API와 실제 Crosswalk loader/CLI dry-run E2E 구현.
- [x] Phase 6: 프로젝트 CRUD, 환경별 저장/불러오기, dirty·409 처리, artifact 등록 및 토폴로지 전체 매핑, 서버 package 미리보기·ZIP 다운로드 구현. HTTP lifecycle과 실제 Crosswalk CLI를 잇는 로컬 E2E harness 추가.
- [~] Phase 7: 24시간 grace 기반 orphan blob 정리 구현. generation 이력/재진입, 브라우저 조작 E2E와 운영 보강이 다음 단계.

## 1. 목표

Studio 프론트엔드의 메모리 기반 프로젝트·토폴로지·패키지 생성을 서버 기반으로 전환한다.
백엔드는 프로젝트와 설계 저장, 서버 검증, 아티팩트 보관, Crosswalk VM 패키지 생성을 담당한다.
SSH 접속과 실제 배포는 계속 Crosswalk의 책임으로 두며 Studio 백엔드가 원격 배포를 실행하지 않는다.

MVP 완료 상태는 다음과 같다.

- 프로젝트와 DEV/QA/PROD 토폴로지가 재시작 후에도 보존된다.
- revision 기반 낙관적 잠금으로 여러 탭의 덮어쓰기를 방지한다.
- JAR 및 placeholder 기반 설정 템플릿을 안전하게 등록하고 프로젝트 범위로 참조한다.
- 저장 전 설계 검증과 Crosswalk VM 패키지 생성 전 검증을 구분한다.
- 비동기 생성 작업의 상태를 조회하고 생성된 파일을 미리보기/ZIP으로 다운로드한다.
- 생성 패키지는 Crosswalk의 실제 설정 로더로 호환성 테스트를 통과한다.

## 2. 범위

### MVP 포함

- `/api/v1/capabilities`
- 프로젝트 생성·목록·조회·수정
- 환경별 Blueprint와 layout 조회·전체 저장
- 설계 및 패키지 목적 검증
- immutable artifact 업로드·목록
- `crosswalk-vm` draft/package 비동기 생성
- 패키지 파일 목록·텍스트 미리보기·ZIP 다운로드
- request ID, 구조화 로그, 요청 크기 제한, graceful shutdown
- loopback 기본 바인딩과 Host/Origin 검사

### MVP 제외

- SSH 배포 실행 및 서버 상태 조회
- 프로젝트·아티팩트·패키지 영구 삭제
- 실제 Secret 값 저장 및 배포 환경변수 관리
- GitLab/GitHub MR 생성
- Helm, Terraform, Docker, Kubernetes 생성기
- 다중 인스턴스 worker와 외부 메시지 큐
- 공개 SaaS 운영과 조직 단위 권한 관리

## 3. 기술 선택

| 영역 | MVP 선택 | 이유와 확장 방향 |
| --- | --- | --- |
| 언어 | Go, 별도 `studio/backend` 모듈 | Crosswalk와 운영 언어를 맞추되 실행 코드와 수명 주기는 분리한다. |
| HTTP | `net/http` + 얇은 middleware | 프레임워크 의존성을 줄이고 handler/domain 경계를 명확히 한다. |
| 계약 | OpenAPI 3.1 + Blueprint JSON Schema | API와 모델 버전을 명시하고 프론트 DTO 생성의 기준으로 사용한다. |
| 메타데이터 | SQLite, WAL, migration | 로컬 단일 인스턴스 MVP에 적합하다. repository 인터페이스 뒤에 두어 PostgreSQL로 교체 가능하게 한다. |
| 바이너리 | 로컬 content-addressed 파일 저장소 | 서버가 계산한 SHA-256으로 저장하며 원래 파일명과 실제 경로를 분리한다. 이후 S3 adapter를 추가할 수 있다. |
| 생성 작업 | DB에 영속화한 작업 + 프로세스 내 worker | API 재시작 시 작업 유실을 막는다. 다중 인스턴스 전환 시 queue 구현만 교체한다. |
| 패키지 | 서버 측 디렉터리 + streaming ZIP | 브라우저 JSZip을 대체하고 권한·경로 검사를 서버에서 일관되게 수행한다. |

초기 기본 데이터 경로는 명시적 `--data-dir` 또는 `STUDIO_DATA_DIR`로만 받는다.
운영체제 홈 디렉터리를 암묵적으로 조립하지 않으며 DB, artifact, package, temporary 디렉터리를 그 아래에 분리한다.

## 4. 관심사 분리와 제안 구조

프론트와 백엔드 모듈은 같은 Studio 제품 아래에 두되 의존성과 빌드는 독립시킨다.

```text
studio/
  app/                         # 현재 프론트 화면
  features/
  lib/api/
    client.ts                  # 공통 요청·오류·취소 처리
    generated.ts               # OpenAPI 생성 DTO
    projects.ts
    artifacts.ts
    topologies.ts
    generations.ts
  backend/
    cmd/studio-api/main.go      # 설정 조립, 서버 시작, shutdown
    internal/
      api/                      # HTTP handler, request/response 변환
      middleware/               # request ID, logging, limits, origin
      domain/                   # Blueprint, Project, Artifact, Generation
      service/                  # use case와 transaction 경계
      validation/               # 모델/배포/아티팩트/생성기 검증
      repository/               # 저장 인터페이스
      sqlite/                   # SQLite 구현과 migration
      blobstore/                # artifact/package 파일 저장 인터페이스
      localstore/               # 안전한 로컬 파일 구현
      generation/               # queue/worker와 generator registry
      generator/crosswalkvm/    # Crosswalk VM 패키지 생성기
      packageio/                # preview, manifest, streaming ZIP
    migrations/
    go.mod
    go.sum
  schemas/
    openapi-v1.yaml
    blueprint-v1.schema.json
```

의존 방향은 `api -> service -> domain/repository interface`로 고정한다.
SQLite, 로컬 파일 저장소, Crosswalk 생성기는 interface 구현체이며 handler가 직접 호출하지 않는다.
프론트 컴포넌트는 `lib/api`만 사용하고 fetch, DTO 변환, React Flow 변환을 UI 이벤트 안에 섞지 않는다.

## 5. 핵심 데이터 모델

| 테이블 | 주요 필드와 제약 |
| --- | --- |
| `projects` | id, owner_id, name, description, revision, created_at, updated_at |
| `topologies` | project_id + environment unique, revision, blueprint_json, layout_json, updated_at |
| `artifacts` | id, project_id, kind, original_name, blob_key, size, sha256, created_at; 수정 불가 |
| `generations` | id, project_id, environment, topology_revision, generator, output, status, error_json, package_id |
| `idempotency_keys` | owner_id + project_id + key unique, request_hash, generation_id, expires_at |
| `packages` | id, project_id, snapshot metadata, generator_version, manifest_json, created_at |
| `package_files` | package_id + path unique, blob_key, media_type, size, sha256, previewable |

프로젝트 revision과 환경별 topology revision은 별도로 관리한다.
프로젝트 이름/설명 수정은 project revision을, topology 저장은 해당 환경 revision만 증가시킨다.
모든 하위 ID 조회는 먼저 project 범위와 owner 범위를 적용하고, 접근할 수 없는 대상은 404로 통일한다.

## 6. 처리 흐름

### 토폴로지 저장

1. 요청 JSON과 Blueprint schema를 검사한다.
2. project/environment 범위에서 현재 revision을 조회한다.
3. `expectedRevision`이 다르면 아무것도 저장하지 않고 409를 반환한다.
4. 구조적으로 유효한 Blueprint와 layout을 한 transaction으로 저장한다.
5. 서버가 증가시킨 revision과 저장 결과를 반환한다.

의미상 미완성인 설계는 저장할 수 있다. 참조가 깨지거나 DTO 형식이 잘못된 문서는 저장하지 않으며,
생성 가능 여부는 validation 및 generation 단계에서 다시 판정한다.

### 아티팩트 업로드

1. multipart body 전체와 kind별 파일 크기를 streaming 방식으로 제한한다.
2. 서버 임시 파일에 기록하며 SHA-256을 동시에 계산한다.
3. 확장자, MIME hint, JAR ZIP 구조 또는 UTF-8 텍스트 형식을 검사한다.
4. 설정 템플릿에서 의심되는 실제 Secret 값을 탐지하고 원문을 로그/응답에 남기지 않는다.
5. content-addressed 최종 경로로 원자적 rename 후 DB metadata를 기록한다.
6. DB 기록 실패 시 생성한 blob을 정리하고, 시작 시 orphan temporary 파일을 정리한다.

JAR 내용은 실행하지 않는다. 압축 항목을 검사할 경우 entry 수, 비압축 총량, 압축 비율,
절대 경로, `..`, symlink를 제한해 zip bomb과 경로 탈출을 차단한다.

### 비동기 패키지 생성

1. `Idempotency-Key`, topology revision, request hash를 한 transaction에서 고정한다.
2. 저장된 topology와 artifact ID/SHA-256을 immutable snapshot으로 만든다.
3. 생성 서버가 package 목적 검증을 다시 수행한다.
4. worker는 임시 패키지 디렉터리에 파일을 만들고 manifest의 경로와 checksum을 검사한다.
5. 성공 결과를 최종 경로로 원자적 승격한 뒤 package와 generation 상태를 transaction으로 연결한다.
6. API 재시작 시 `queued`는 재개하고 오래된 `running`은 재시도 가능 횟수에 따라 재등록 또는 명시적 실패 처리한다.

같은 idempotency key와 같은 body는 기존 작업을 반환하며, body가 다르면 409를 반환한다.
worker panic과 프로세스 종료가 API 프로세스를 종료시키지 않도록 작업 경계에서 복구·상태 기록한다.

## 7. 구현 단계

### Phase 0 — 계약 및 결정 고정

- `docs/studio-api.md`를 OpenAPI 3.1로 옮기고 예제와 오류 코드를 계약 테스트 대상으로 만든다.
- Blueprint v1 JSON Schema와 환경, node spec, placement, connection 제약을 확정한다.
- Crosswalk 차트 포맷 식별자와 generator version 정책을 확정한다.
- 로컬 MVP의 인증 주체를 `local-user`로 고정하되 모든 schema/query에 owner 범위를 둔다.
- 업로드 제한 기본값과 data directory 설정을 확정한다.

완료 기준: OpenAPI/JSON Schema validation 통과, 프론트/백엔드 타입 생성 재현 가능, 미확정 필드 없음.

### Phase 1 — 서버 골격과 저장 기반

- `studio/backend` 모듈, 설정 로더, dependency wiring, HTTP server를 추가한다.
- migration runner와 SQLite repository 기반을 추가한다.
- 공통 오류 envelope, request ID, access log, recovery, timeout, body limit middleware를 구현한다.
- `/healthz`와 `/api/v1/capabilities`를 구현한다.
- loopback 기본 bind, Host/Origin allowlist, graceful shutdown을 검증한다.

완료 기준: 빈 data directory에서 기동/재기동 가능, migration 재실행이 안전하고 smoke test 통과.

### Phase 2 — 프로젝트와 토폴로지 영속화

- 프로젝트 목록/생성/조회/수정 API를 구현한다.
- 환경별 topology GET/PUT과 revision 충돌을 구현한다.
- domain validation과 저장 가능한 편집 상태의 경계를 테스트한다.
- 프론트 `api-adapter`의 React Flow ↔ Blueprint/layout round-trip을 구현한다.

완료 기준: 새로고침 후 복원, 환경 간 독립성, 두 클라이언트 동시 수정 409, layout round-trip 테스트 통과.

### Phase 3 — 서버 검증

- model, deployment, artifact, generator 네 단계 validator를 분리한다.
- JSON Pointer, 관련 node/connection ID, 안정적인 issue code를 반환한다.
- `design`과 `package` 목적에 따라 warning/error 승격 규칙을 적용한다.
- 프론트가 `clientRequestId`와 편집 버전으로 오래된 응답을 폐기하도록 연결한다.

완료 기준: API 계약의 허용/금지 조합 table test와 프론트 오류 연결 테스트 통과.

### Phase 4 — 아티팩트 저장

- artifact repository와 local blobstore를 구현한다.
- streaming upload, checksum, kind별 검사, immutable metadata를 구현한다.
- Java 노드가 filename이 아닌 artifactRef를 선택하도록 프론트를 변경한다.
- 다른 프로젝트 artifact 참조와 Secret 의심 템플릿을 거부한다.

완료 기준: 제한 초과 413, 잘못된 형식 415/422, 중단 업로드 잔여 파일 없음, 재시작 후 목록 복원.

### Phase 5 — Crosswalk VM 생성과 패키지

- generator interface와 registry, persistent worker를 구현한다.
- `crosswalk-vm` draft/package 생성기를 구현한다.
- package manifest, 파일 목록, preview, streaming ZIP API를 구현한다.
- Crosswalk 설정 parser/validator를 재사용 가능한 공개 package로 최소 추출하거나,
  생성 결과를 Crosswalk CLI 검증 경계에서 검사하는 호환성 harness를 추가한다.
- artifact snapshot과 생성기 버전을 package metadata에 고정한다.

완료 기준: sample Blueprint의 golden package 생성, Crosswalk 실제 loader validation 통과,
동일 key 재요청 단일 생성, 서버 재시작 후 작업 상태 복구, ZIP 경로 안전성 테스트 통과.

### Phase 6 — 프론트 전환

- sample project와 메모리 초기화를 서버 프로젝트 목록/상세 호출로 교체한다.
- 명시적 저장, dirty 상태, saving/error 상태, 409 재조회/재적용 UX를 구현한다.
- 로컬 미리보기 생성은 offline draft 용도로만 남기거나 제거하고 서버 package 화면으로 교체한다.
- polling은 1~5초 backoff, 화면 이탈/완료/실패 시 취소하도록 구현한다.
- 개발 프록시와 동일 origin 배포 구성을 추가한다.

완료 기준: 프로젝트 생성 → 설계 저장 → 검증 → artifact 등록 → package 생성 → ZIP 다운로드 E2E 통과.

### Phase 7 — 운영 준비

- DB backup/restore와 data directory 권한 가이드를 작성한다.
- `go test -race`, fuzz test, request size/load smoke test를 수행한다.
- 로그에 Secret, 내부 경로, stack trace가 노출되지 않는지 검토한다.
- 배포 바이너리/컨테이너 및 Studio 프론트와의 same-origin 실행 방식을 문서화한다.
- 사내 공유 전에 실제 인증, CSRF 정책, project 권한 middleware를 구현한다.

완료 기준: 보안 체크리스트, 복구 훈련, 배포/롤백 문서, 전체 회귀 테스트 통과.

## 8. 테스트 전략

- Domain unit: node spec, placement, connection, port, heap, artifact 참조를 table-driven test로 검사한다.
- Repository integration: 임시 SQLite DB에서 migration, transaction rollback, revision CAS, cursor pagination을 검사한다.
- HTTP contract: `httptest`로 상태 코드, error envelope, content type, 크기 제한을 검사한다.
- File security: traversal, absolute path, symlink, zip bomb, MIME 위장, 중단 업로드를 fixture로 검사한다.
- Generation: generator golden file과 동일 입력 결정성, idempotency, crash recovery를 검사한다.
- Cross-product compatibility: 생성 YAML을 Crosswalk 실제 loader/validator로 읽는다.
- Frontend: adapter round-trip, stale response 폐기, 409 보존 UX를 검사한다.
- E2E: 실제 HTTP API에서 프로젝트 생성·artifact 업로드·설계 저장·package 다운로드 후 Crosswalk CLI dry-run까지 자동화한다. 브라우저 조작 E2E는 운영 준비 단계에서 별도로 추가한다.

CI의 기본 gate는 backend unit/integration, frontend type/test/build, Crosswalk 전체 test,
Studio→Crosswalk 호환성 test로 구성한다. 네트워크나 실제 SSH는 이 gate에 포함하지 않는다.

## 9. 예상 오류 케이스와 방지 로직 검토

### 1. 여러 탭이 같은 토폴로지를 덮어씀

- 방지 로직: `expectedRevision`을 SQL transaction 안에서 비교·증가하는 CAS 저장.
- 프론트 대응: 409 시 로컬 편집을 유지하고 최신 서버본과 재적용 선택을 제공.
- 검증: 동일 revision으로 동시에 PUT한 요청 중 하나만 성공하는 integration test.

### 2. 업로드 중단 또는 DB 실패로 파일과 metadata가 불일치

- 방지 로직: 제한된 임시 파일, checksum 완료 후 원자적 rename, metadata 실패 시 보상 삭제,
  시작 시 오래된 temporary/orphan scan.
- 검증: client disconnect, 디스크 쓰기 실패, transaction 실패를 주입한 test.

### 3. 생성 요청 중 설계나 JAR가 바뀌어 재현할 수 없는 패키지가 생성됨

- 방지 로직: topology revision, artifact ID/SHA-256, generator version을 생성 시작 전에 snapshot으로 고정하고
  worker가 다시 checksum과 package 목적 검증을 수행.
- 검증: enqueue 후 topology 갱신, 다른 프로젝트 artifact 치환, 파일 변조 test.

### 4. 재시도 때문에 동일 패키지가 중복 생성됨

- 방지 로직: owner/project/key unique 제약, canonical request hash 비교, persistent generation 상태.
- 검증: 동시 POST와 서버 재시작 후 같은 key 재요청 test.

### 5. 패키지 경로를 통해 서버 파일이 노출되거나 ZIP 경로 탈출 발생

- 방지 로직: API는 fileId만 받고 manifest 상대 경로를 정규화한다. 절대 경로, `..`, symlink를 거부하며
  package root 내부 파일만 descriptor 기반으로 연다.
- 검증: encoded traversal, 중복 경로, symlink 및 악성 archive fixture test.

계획 단계 검토 결과 위 방지 로직은 각 구현 Phase의 완료 기준과 테스트 항목에 포함되어 있다.
구현 시에는 각 항목의 실패 주입 테스트가 통과하기 전 해당 Phase를 완료로 표시하지 않는다.

## 10. 결정이 필요한 항목

Phase 0에서 다음 항목을 확정해야 한다.

1. Crosswalk 차트 포맷의 정식 version과 공개 validation 경계.
2. JAR 256 MiB, 텍스트 1 MiB, preview 1 MiB 초기 제한의 운영 적합성.
3. 로컬 MVP 이후 인증 공급자와 사용자/조직/project 권한 모델.
4. 설정 템플릿의 허용 문법과 Secret 오탐 처리 UX.
5. SQLite 단일 인스턴스를 유지할 범위와 PostgreSQL/object storage 전환 조건.

기본 제안은 위 기술 선택대로 구현하되, 1번은 패키지 생성 Phase 전에 반드시 확정한다.

## 11. 권장 첫 번째 구현 단위

첫 PR은 Phase 0과 Phase 1까지만 포함한다.

- OpenAPI/Blueprint schema
- `studio/backend` 서버 골격
- SQLite migration 기반
- 공통 오류/middleware
- health/capabilities
- 단위·contract test와 루트 Makefile 명령

프로젝트 CRUD나 파일 업로드를 첫 PR에 함께 넣지 않는다. 이 경계가 API 계약, 저장 계층,
운영 안전 기본값을 먼저 검증하면서 이후 기능 PR의 크기를 일정하게 유지한다.
