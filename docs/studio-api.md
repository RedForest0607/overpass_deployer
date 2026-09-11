# Studio 프론트엔드–백엔드 API 계약 초안

작성일: 2026-09-10 · 상태: 부분 구현 (프로젝트·설계 저장·design 검증) · HTTP prefix: `/api/v1`

## 1. 목적과 현재 상태

Deployer의 Studio는 설계·차트 생성 도구이고 Crosswalk는 생성된 패키지의 실행기다.
이 문서는 Studio 프론트와 향후 Studio 백엔드 사이의 계약이다. Crosswalk 원격 실행 API가 아니다.

현재 확인한 구현:

- `studio/features/studio/Studio.tsx`: 실제 프로젝트와 환경별 설계 편집·저장·서버 검증 화면.
- `studio/features/studio/usePersistence.ts`: revision 충돌, 미저장 변경, 불러오기와 검증 응답 수명 관리.
- `studio/features/topology/model.ts`: 브라우저 검증과 평탄한 노드 데이터.
- `studio/features/topology/placement.ts`: 부모 배치와 내부 deployment edge 동기화.
- `studio/features/generation/preview.ts`: 브라우저에서 JSON·VM YAML 초안 생성.
- `studio/features/generation/PackagePreview.tsx`: 브라우저에서 파일 표시·복사·ZIP 생성.

Studio 프론트에서 프로젝트 생성/목록/수정, 환경별 토폴로지 GET/PUT, purpose=design 검증을 호출한다.
백엔드는 revision CAS와 SQLite 영속 저장, 모델·배포 방식 의미 검증을 지원한다.
artifact, generation, package API와 공유 환경 인증은 아직 구현 목표다. Crosswalk 배포 상세 Blueprint와 deploy.yml/ZIP 생성기 core는 구현됐으며 API 작업 흐름 연결 전이다.
purpose=package 및 generator 지정 검증은 현재 422 UNSUPPORTED_GENERATOR로 거부한다.
아래 후속 기능의 계약은 구현 목표로 유지하며, 현재 실행 가능한 배포 패키지를 생성한다는 뜻이 아니다.
기존 `studio.overpass.io/draft` JSON을 v1 요청으로 그대로 보내지 않는다.

## 2. 책임과 연동 구조

프론트: 그래프 편집, 빠른 로컬 검사, DTO 변환, 요청 상태 및 필드 오류 표시.
백엔드: 권한 검사, 저장, 모델·배포 방식 검증, 파일 보관, 차트 생성, 패키지 관리.
Crosswalk: 자기 입력 형식 검증과 실제 실행. 백엔드는 SSH 배포를 호출하지 않는다.

권장 프론트 모듈(후속 구현):

```text
studio/lib/api/
  client.ts               # base URL, JSON, 오류, 취소
  contracts.ts            # API DTO; 추후 확정된 OpenAPI에서 생성
  projects.ts
  artifacts.ts
  topologies.ts
  generations.ts
studio/features/topology/
  api-adapter.ts          # React Flow ↔ Blueprint + layout
```

UI 컴포넌트에서 fetch·직렬화·검증 규칙을 한꺼번에 구현하지 않는다.
네트워크 호출을 React Flow의 노드 이동 이벤트마다 실행하지 않고 명시적 저장으로 시작한다.

## 3. 공통 규약

- JSON은 camelCase, 시간은 UTC RFC 3339, ID는 서버 발급 불투명 문자열.
  Blueprint 내부 노드·연결 ID는 프론트 UUID 허용; 이름 변경으로 ID를 바꾸지 않는다.
- 성공 단건은 객체를 직접 반환한다. 목록은 `{items: [], nextCursor: null}`.
  `limit` 기본 20 / 최대 100, `cursor`는 불투명 값. 마지막 페이지는 null.
- 프로젝트 이름은 표시용이며 중복 허용. 배포 리소스 이름 제약은 별도 검증한다.
- 환경은 MVP에서 `dev | qa | prod`. 서로 독립적인 토폴로지 슬롯이며 암묵적 복사·overlay가 없다.
- 변경 API는 원자적 처리. 오류 응답으로 부분 저장된 데이터가 생기지 않아야 한다.
- MVP는 loopback API + 동일 origin 개발 프록시를 권장한다. 공개 바인딩과 CORS wildcard 금지.
  로컬 모드도 Host/Origin 검사와 요청 크기 제한을 적용한다.
- 사내 공유 실행 전에 사용자 인증·프로젝트별 권한 검사를 구현한다.
  쿠키 인증 채택 시 HttpOnly/SameSite 쿠키와 CSRF 방어를 적용한다.
  아래 모든 ID 접근은 소유 프로젝트 범위 검사가 필요하다.
- 비밀 값·SSH 키는 모델, URL, 오류, 로그, 미리보기에 포함하지 않는다.

### 공통 오류

```json
{
  "error": {
    "code": "REVISION_CONFLICT",
    "message": "서버에 더 최신 설계가 있습니다.",
    "requestId": "req-123",
    "details": {"expectedRevision": 3, "actualRevision": 4}
  }
}
```

| HTTP | 의미 / 대표 code |
| --- | --- |
| 400 | JSON/DTO 형식 오류, INVALID_REQUEST |
| 401 / 403 | 미인증 / 권한 부족 |
| 404 | 접근 가능한 범위에 대상 없음 |
| 409 | REVISION_CONFLICT, IDEMPOTENCY_CONFLICT |
| 413 | 업로드/요청 크기 초과 |
| 415 | 지원하지 않는 파일/Content-Type |
| 422 | 생성 불가 설계, UNSUPPORTED_VERSION, UNSUPPORTED_GENERATOR, UNSAFE_ARTIFACT |
| 429 | 요청 과다; Retry-After 준수 |
| 500 / 503 | 내부 오류 / 일시적 사용 불가; 민감한 내부 경로·stack 반환 금지 |

## 4. API 목록과 화면 연결

모든 경로는 `/api/v1` 기준이다. `p`는 projectId, `e`는 environment다.

| 화면 동작 | Method / 경로 | 성공 |
| --- | --- | --- |
| 지원 기능 확인 | GET /capabilities | 200 |
| 프로젝트 목록 | GET /projects?limit=20&cursor=… | 200 |
| 프로젝트 생성 | POST /projects | 201 |
| 프로젝트 상세 | GET /projects/{p} | 200 |
| 프로젝트 수정 | PATCH /projects/{p} | 200 |
| 파일 목록 | GET /projects/{p}/artifacts | 200 |
| 파일 등록 | POST /projects/{p}/artifacts | 201 |
| 환경별 설계 읽기 | GET /projects/{p}/topologies/{e} | 200 |
| 환경별 설계 저장 | PUT /projects/{p}/topologies/{e} | 200 |
| 검증 버튼 | POST /projects/{p}/validations | 200 |
| 산출물 생성 | POST /projects/{p}/generations | 202 |
| 생성 진행 조회 | GET /projects/{p}/generations/{generationId} | 200 |
| 패키지 파일 목록 | GET /projects/{p}/packages/{packageId} | 200 |
| 텍스트 미리보기 | GET /projects/{p}/packages/{packageId}/files/{fileId} | 200 |
| ZIP 다운로드 | GET /projects/{p}/packages/{packageId}/download | 200 binary |

프로젝트·아티팩트의 영구 삭제, GitLab MR, Secret 값 등록, SSH 실행은 MVP API 범위 밖이다.
캔버스 노드 삭제는 로컬 Blueprint 변경 후 PUT으로 저장한다.

## 5. 프로젝트와 아티팩트

### 프로젝트

POST body: `{name: string, description: string}`.
응답 및 상세: `{id, name, description, revision, createdAt, updatedAt}`.
PATCH body: `{expectedRevision, name?, description?}`.
revision은 변경마다 증가하고 불일치하면 409. ID·작성 시각은 변경할 수 없다.
환경별 설계가 없으면 GET topology는 404 `TOPOLOGY_NOT_FOUND`; 프론트는 빈 편집기를 연다.

### 파일 업로드

POST multipart/form-data:

| 필드 | 타입 | 의미 |
| --- | --- | --- |
| file | binary, 필수 | 파일 하나 |
| kind | enum, 필수 | jar / config-template / env-example |
| displayName | string, 선택 | 표시 이름; 저장 경로로 사용 금지 |

응답 Artifact:
`{id, projectId, kind, originalName, sizeBytes, sha256, createdAt}`.
파일은 immutable이다. 재업로드는 새 ID이며 서버가 SHA-256을 계산한다.
배포 모델은 파일명이 아니라 artifact ID를 참조한다. 다른 프로젝트 파일은 참조할 수 없다.
서버 파일 경로는 응답하지 않는다.

업로드 상한은 capabilities에서 제공한다(초기 제안: JAR 256 MiB, 텍스트 1 MiB).
프론트 사전 검사와 별개로 서버가 스트리밍 크기 제한·확장자·내용 형식을 검사한다.
업로드된 JAR를 실행하지 않으며, 압축 해제 검사를 도입하면 압축 폭탄·경로 탈출 제한이 필요하다.

MVP에서는 실제 비밀 값이 담긴 yml/env가 아니라 placeholder 기반 템플릿만 허용한다.
예: `password: ${DB_PASSWORD}`, `DB_PASSWORD=`.
의심되는 비밀 값은 422로 거부하고 검출 결과에 원문 값을 노출하지 않는다.
자동 검출은 완전하지 않으므로 실제 설정 파일 수용은 암호화·키 관리 정책이 마련된 뒤 확장한다.
서비스의 설정 파일 배치 DTO는 후속 계약이며, 초기에는 업로드/목록과 Java JAR 참조까지만 연결한다.

## 6. Blueprint와 레이아웃 계약

PUT은 환경별 전체 문서를 저장한다. 최초 저장은 expectedRevision=0, 이후 읽은 revision을 보낸다.
성공 응답은 `{projectId, environment, revision, updatedAt, blueprint, layout}`.
저장은 구조적으로 유효한 미완성 설계도 허용한다. 의미상 오류는 validation으로 보고하고 생성 단계에서 차단한다.
스키마를 벗어난 JSON은 저장하지 않는다. 서버는 클라이언트 revision을 새 revision으로 그대로 사용하지 않는다.

### 저장 요청 예시

```json
{
  "expectedRevision": 0,
  "blueprint": {
    "apiVersion": "studio.deployer/v1",
    "nodes": [
      {
        "id": "vm-01",
        "kind": "vm",
        "name": "dev-was-01",
        "spec": {"provider": "aws-ec2", "host": "10.0.0.10"}
      },
      {
        "id": "app-01",
        "kind": "java",
        "name": "bo-pcweb",
        "spec": {
          "artifactRef": "artifact-01",
          "javaVersion": "21",
          "maxHeap": "1g",
          "ports": [{"name": "http", "servicePort": 8080, "protocol": "TCP"}]
        }
      }
    ],
    "placements": [
      {"workloadRef": "app-01", "targetRef": "vm-01", "mode": "process"}
    ],
    "connections": []
  },
  "layout": {
    "nodes": [
      {"nodeId": "vm-01", "position": {"x": 90, "y": 65}},
      {"nodeId": "app-01", "position": {"x": 50, "y": 100}}
    ],
    "viewport": {"x": 0, "y": 0, "zoom": 1}
  }
}
```

### 타입과 필드

Node 공통 필수: `id, kind, name, spec`. kind에 따라 spec을 구분하며 불필요한 필드는 거부한다.

| kind | spec 필드 | 생성 지원 |
| --- | --- | --- |
| vm | provider: on-prem / aws-ec2, host: string | Java 직접 실행 생성기의 대상 |
| java | artifactRef: string 또는 null, javaVersion: string, maxHeap: string, ports: Port[] | Crosswalk VM 우선 |
| redis / elasticsearch | location: hosted / external, endpoint?: string, ports: Port[], secretRef?: string | 설계 저장 우선; 생성 지원은 capabilities 기준 |
| database | engine: postgresql, endpoint: string, ports: Port[], secretRef?: string | 외부 의존성 설계 |
| kubernetes | namespace: string | 설계 저장 우선 |
| registry | endpoint: string, secretRef?: string | 설계 저장 우선 |
| ingress | host: string | 설계 저장 우선 |

Port: `{name: string, servicePort: integer(1..65535), protocol: TCP|UDP}`.
포트 이름은 노드 내 고유하다. 비어 있는 필수 입력은 빈 문자열/null/빈 배열 등 정의된 편집 상태로 저장하고,
실제 필수 조건은 의미 검증에서 오류로 반환한다. artifactRef만 null을 허용하며 기타 문자열 필드는 문자열이다.
현재 v1 초안에는 publishedPort·다중 바인딩 주소를 포함하지 않는다. Compose 생성 전에 별도 확정한다.

Placement: `{workloadRef, targetRef, mode: process|compose|kubernetes}`.
Java·hosted Redis/Elasticsearch에는 정확히 하나, external에는 없어야 한다.
VM/EC2는 여러 서비스를 포함할 수 있다. Kubernetes mode는 Kubernetes 대상에만 허용한다.
process/compose mode는 VM 대상에만 허용하며 실제 생성 지원은 capabilities로 제한한다.

Connection:
`{id, kind: connects_to|routes_to|publishes_to, sourceRef, targetRef, targetPortName?}`.
connects_to/routes_to에는 targetPortName 필수, publishes_to에는 지정하지 않는다.
포트 숫자·프로토콜은 대상의 Port에서 가져와 중복 저장하지 않는다.
Java → Java/DB/Redis/Elasticsearch, Ingress → Java, Java → Registry 조합을 우선 허용한다.

### 프론트 변환 규칙

| 현재 React Flow 값 | API 표현 |
| --- | --- |
| data.label | node.name |
| data.platform | vm.spec.provider로 enum 변환 |
| data.java / heap | javaVersion / maxHeap |
| data.artifact 파일명 | 업로드 후 artifactRef; 파일명으로 추정 매칭 금지 |
| data.port | spec.ports[0]; 신규 UI에서 명명된 복수 포트로 확장 |
| parentId + deploys_to edge | placements 하나로 정규화 |
| TCP·포트 edge label | 화면 표시 전용; connections의 판별 기준으로 사용 금지 |
| position / viewport | layout |
| selected, measured, style, extent, React Flow type | API 전송 금지; 로드 시 재생성 |

배치의 진실은 placements 한 곳이다. layout에는 parentId를 중복 저장하지 않는다.
배치된 노드 좌표는 부모 기준, 나머지는 캔버스 기준이다. 로드 시 부모를 먼저 생성하고 extent=parent를 복원한다.
layout이 없거나 일부 누락되면 자동 배치하며 배포 의미에는 영향이 없다.
연결 선 표시용 포트·라벨은 대상 Port에서 다시 계산한다.

## 7. 검증 API

POST body:

```json
{
  "environment": "dev",
  "clientRequestId": "edit-12",
  "blueprint": {
    "apiVersion": "studio.deployer/v1",
    "nodes": [],
    "placements": [],
    "connections": []
  },
  "generator": "crosswalk-vm",
  "purpose": "package"
}
```

미저장 편집 내용도 검사한다. purpose는 `design | package`; design은 모델·관계 검사,
package는 생성기 지원·파일 존재·배포 방식 검사까지 수행한다.
generator는 package에서 필수이며 design에서는 생략한다.
파일 확인은 서버 소유 저장소 범위에서만 수행하며 네트워크 대상 점검은 하지 않는다.

응답 예시(의미상 실패도 정상 검증 결과이므로 200):

```json
{
  "clientRequestId": "edit-12",
  "valid": false,
  "issues": [
    {
      "code": "EMPTY_TOPOLOGY",
      "severity": "error",
      "stage": "model",
      "nodeIds": [],
      "connectionIds": [],
      "path": "/nodes",
      "message": "배포할 서비스를 추가하세요.",
      "suggestion": "Java 서비스와 VM 실행 대상을 추가하세요."
    }
  ]
}
```

Issue.stage: `model | deployment | artifact | generator`.
path는 Blueprint 기준 JSON Pointer. valid는 error가 없을 때 true이며 실행 성공을 보장하지 않는다.
warning은 생성 가능하되 UI에서 표시한다. 예: JAR 누락은 design에서 warning, package에서 error.
지원되지 않는 hosted Redis 설치를 조용히 제외하지 않고 package 검증에서 오류로 차단한다.

모델: 이름·ID·참조 무결성, 타입별 필드, placement 일치, 허용된 연결.
배포 방식: 실행 대상·모드 호환, VM 직접 실행 서비스 포트 충돌, 지원 Java 버전·heap 형식.
서버의 실제 포트 점유·SSH 접속·방화벽 개방은 별도의 승인형 점검으로 남긴다.
현재 단순 VM 모델에서는 동일 TCP 서비스 포트를 충돌로 판단한다. 컨테이너 포트 규칙을 재사용하지 않는다.

프론트는 요청 중 편집되면 이전 검증 응답을 폐기한다. clientRequestId와 로컬 편집 버전을 비교하며,
취소 가능한 요청은 AbortController로 중단한다. 성공 메시지는 반드시 검사한 편집 버전에만 적용한다.

## 8. 생성 작업과 패키지

생성 흐름: 로컬 검증 → 저장 → 반환 revision으로 생성 요청 → 상태 조회 → 파일 미리보기/다운로드.
별도 validation 성공은 보안 경계가 아니다. 생성 서버가 저장된 스냅샷을 다시 검증한다.

POST generation body:

```json
{
  "environment": "dev",
  "revision": 1,
  "generator": "crosswalk-vm",
  "output": "package"
}
```

Header: `Idempotency-Key: <프론트 UUID>` 필수.
output: `draft | package`. draft는 미완성 설정을 명시적으로 표시하며 실행 가능으로 표현하지 않는다.
package는 필수 자산·지원되지 않는 서비스가 있으면 422로 거부한다.
서버는 해당 revision을 원자적으로 스냅샷으로 고정한다. 최신 revision과 다르면 409이며 다른 버전으로 대체하지 않는다.
artifact ID와 SHA-256, 생성기 버전도 고정한다.
같은 사용자·프로젝트에서 같은 key/body 재요청은 같은 작업을 반환하고 다른 body는 409.
키 보존 기간은 최소 24시간; 서버 보존 기간 이후 프론트는 자동 재생성을 하지 않고 사용자 확인을 받는다.

POST 및 GET 응답:

```json
{
  "id": "generation-01",
  "projectId": "project-01",
  "environment": "dev",
  "revision": 1,
  "status": "queued",
  "packageId": null,
  "error": null
}
```

status 전이: queued → running → succeeded 또는 failed.
succeeded에만 packageId가 있고, failed에는 공통 오류의 error 객체가 있다.
즉시 발견한 입력 오류는 422, 작업 실행 중 실패는 GET 200 + failed로 구분한다.
프론트는 1초부터 최대 5초까지 간격을 늘려 조회하고 완료·실패·화면 이탈 시 중단한다.
일시적 조회 실패를 작업 실패로 단정하지 않고 재시도 UI를 제공한다.

패키지 상세 응답:

```json
{
  "id": "package-01",
  "projectId": "project-01",
  "environment": "dev",
  "revision": 1,
  "output": "draft",
  "generator": "crosswalk-vm",
  "generatorVersion": "0.1.0",
  "chartFormat": "crosswalk-deploy-legacy",
  "createdAt": "2026-09-10T00:00:00Z",
  "files": [
    {
      "id": "file-01",
      "path": "deploy.draft.yml",
      "mediaType": "application/yaml",
      "sizeBytes": 1200,
      "previewable": true
    }
  ]
}
```

chartFormat의 legacy는 현 Crosswalk 입력을 가리키는 제안 식별자이지 기존 차트에 버전 필드가 있다는 뜻이 아니다.
파일 목록의 path는 패키지 상대 표시 경로다. API에 임의 파일 경로를 전달하지 않고 fileId를 사용한다.
서버는 ZIP 경로의 절대 경로·상위 경로 이동·symlink 탈출을 차단한다.

파일 미리보기 응답: `{id, content, encoding: "utf-8", truncated: false}`.
초기 미리보기 상한 1 MiB. 잘린 응답은 truncated=true로 명시하고 전체 복사로 오인시키지 않는다.
JAR 등 바이너리는 previewable=false이며 미리보기 요청은 415.
ZIP 응답은 application/zip + 안전하게 인코딩한 Content-Disposition attachment.
권한 검사를 통과한 서버 패키지를 다운로드한다. 백엔드 연동 후 브라우저 JSZip으로 다시 조립하지 않는다.
초기 패키지는 로컬 영속 저장, 자동 만료 없음. 보존·삭제 정책 추가 시 계약을 별도로 확장한다.

## 9. capabilities

GET 응답 필드:

- blueprintVersions: string[] — 지원 모델 버전.
- generators: `{id, outputs: ("draft"|"package")[], supportedKinds: string[], supportedModes: string[]}[]`.
  배열은 후보 목록이며 세부 조합은 서버 의미 검증이 최종 판단한다.
- limits: `{jarBytes, textArtifactBytes, previewBytes, topologyNodes, topologyConnections}`.

서버는 실제 구현된 생성기만 반환한다. Helm/Terraform이 없으면 UI에서 비활성화한다.
현 브라우저 샘플의 기능을 서버 기능으로 간주하지 않는다.
새 모드나 Crosswalk 차트 버전 추가 시 스키마·생성기·호환성 테스트를 함께 갱신한다.

## 10. 예상 오류 케이스 3가지와 수용 기준

1. **여러 탭의 덮어쓰기 / 오래된 검증 결과**
   - 저장 revision 비교로 409, 최신 문서 재조회 및 명시적 재적용.
   - 편집 버전이 다른 validation 응답은 버림. 실패 시 로컬 편집 내용을 지우지 않음.
2. **UI 계층과 서버 배치 불일치**
   - parentId/deploys_to를 adapter에서 placements로 정규화, 서버에서 참조·단일 배치 검사.
   - 테스트: VM당 다중 Java/Redis, 호스트 변경·삭제, external/hosted 전환, round-trip 좌표 복원.
3. **생성 중 자산 누락 / 중복 생성 / 위험한 파일**
   - immutable artifact ID·해시와 revision 스냅샷, idempotency 키, 생성 전 파일 재검증.
   - 테스트: 다른 프로젝트 artifact 거부, 누락 파일 422, 같은 키 재시도, ZIP 경로 탈출 차단.

위 내용은 구현 시 필수 수용 기준이며, 현재 백엔드에 방어 로직이 구현됐다는 의미가 아니다.

## 11. 구현 순서와 미확정 항목

1. Blueprint 스키마와 이 계약 확정 → OpenAPI/JSON Schema 작성 → DTO 생성.
2. 프론트 api-adapter round-trip 테스트와 HTTP client 오류 처리.
3. 프로젝트/환경별 설계 저장 API, revision 충돌 UI.
4. 서버 모델·배포 방식 검증 API, 노드·필드 오류 연결.
5. 안전한 아티팩트 업로드·보관, artifactRef 연결.
6. Crosswalk VM 생성 작업·미리보기·ZIP 다운로드, 기존 로컬 생성기 대체.
7. Crosswalk 입력 검증과 생성 패키지 호환성 테스트.

결정 필요: Crosswalk 차트의 정식 버전 정책, 인증 방식, 업로드 용량 운영값,
실제 설정 파일 암호화와 Secret 관리, 설정 템플릿의 서비스별 배치 DTO.
이 문서만으로 API 구현·SSH 실행·외부 게시를 수행하지 않는다.
