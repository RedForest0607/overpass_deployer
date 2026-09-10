# Studio

Deployer 프로젝트의 설계·차트 생성 산출물입니다. React / TypeScript 기반 토폴로지 편집 화면이며 Crosswalk와 독립적으로 실행됩니다.

## 실행

Node 22.13 이상 (테스트 실행은 Node 22.18 이상 권장):

```sh
cd studio
npm ci
npm run dev
```

개발 서버에 표시되는 URL을 엽니다. 기본값은 http://localhost:3000 입니다.

백엔드 골격은 `backend/`에 별도 Go 모듈로 구성되어 있습니다.

```sh
go -C backend run ./cmd/studio-api --data-dir ./.studio-data
```

백엔드는 health/capabilities, 프로젝트 생성·조회·수정, 환경별 토폴로지 revision 저장 및 서버 설계 검증을 제공합니다.
프론트의 프로젝트 화면, 설계 저장/불러오기, 서버 검증 버튼이 API에 연결되어 있습니다.

프런트와 백엔드를 함께 실행할 때는 저장소 루트에서 `make dev-studio-local`을 사용합니다.
실행 후 `make smoke-studio-local`로 프런트, 백엔드, API proxy 연결을 확인할 수 있습니다.

## 검증

```sh
npx tsc --noEmit
node --test features/topology/model.test.mjs
node --test features/topology/api-adapter.test.mjs
node --test schemas/contracts.test.mjs
npm run build
```

## 구현 범위

- 실제 프로젝트 생성·목록·수정과 환경별 독립 설계 (DEV / QA / PROD)
- React Flow 노드 추가, 드래그, 줌, 미니맵, typed connection
- VM/EC2 내부에 여러 Java·Redis·Elasticsearch 배치, 호스트 이동 시 함께 이동
- 라이브러리에서 호스트 영역으로 드롭하거나 서비스 설정의 실행 대상 선택으로 배치 변경
- Redis/Elasticsearch는 외부 endpoint로도 사용 가능; 호스트 삭제 시 내부 서비스 보존
- 노드 더블클릭 / 연결 클릭으로 설정 편집
- 노드·연결 삭제 및 마지막 삭제 취소
- 이름·포트·배치 대상·VM 포트 충돌·연결 포트 검증
- 토폴로지 JSON 다운로드, VM YAML 초안 및 ZIP 미리보기
- 모바일 노드 팔레트와 접근 가능한 설정 Sheet

설계 저장 버튼으로 SQLite에 저장합니다. 새로고침 시 URL의 프로젝트·환경을 복원하며 미저장 편집은 복원하지 않습니다.
프로젝트/환경 전환은 미저장 변경이 있으면 차단합니다. 저장 충돌(409) 시 편집 내용은 유지하고 자동 덮어쓰기를 금지합니다.
JSON으로 편집 내용을 보관한 뒤 확인 대화상자를 통해 서버 버전을 다시 불러올 수 있습니다.
환경 간 자동 복사나 overlay는 적용하지 않습니다.

서버 검증은 모델·배치·포트 충돌·Java 버전/Heap을 검사하며 실제 SSH/네트워크 점검이 아닙니다.
검증 이후 설계가 변경되면 결과를 무효화합니다. 오류를 클릭하면 노드 설정과 관련 필드 오류를 표시합니다.
현재 편집기는 서비스당 main TCP 포트 하나와 VM process/Kubernetes 배치를 지원합니다.
API로 저장한 복수 포트·Compose 등 미지원 표현은 불러올 때 거부하며 원본을 변경하지 않습니다.
JAR 파일명 입력은 로컬 초안 전용이며 영속 저장·업로드가 아닙니다. artifactRef 등록은 후속 작업입니다.

## 모듈

- `features/studio`: 작업공간 구성과 세션 상태
- `features/topology`: 노드 모델, 연결 규칙, 검증, React Flow 노드, Inspector
- `features/topology/api-adapter.ts`: 지원하는 React Flow 표현과 API Blueprint/layout 변환; 미지원 모델의 조용한 손실 방지
- `features/studio/usePersistence.ts`: 프로젝트/환경 선택, revision 저장, dirty 상태 및 검증 응답 수명
- `lib/api`: HTTP 오류 처리와 타입이 있는 프로젝트·설계·검증 호출
- `features/generation`: 화면용 초안 직렬화, 파일 미리보기와 ZIP 다운로드
- `components/ui`: scaffold가 제공한 UI primitives
- `backend`: 독립 Go API 서버와 SQLite migration
- `schemas`: OpenAPI 3.1과 Blueprint v1 JSON Schema

## 백엔드 경계

현재 생성물은 설계 초안이며 실행 가능한 배포 번들이 아닙니다.
JAR 업로드·보관, 설정/env 파일 패키징, Secret 저장, Go CLI 실행은 연결되지 않았습니다.
VM YAML 초안은 Java→VM 배치만 매핑하며 DB/Redis 환경변수와 방화벽을 생성하지 않습니다.
Redis/Elasticsearch의 배치는 JSON에 보존되지만 설치·실행 스크립트는 생성하지 않습니다.
AWS EC2는 VM 호스트 유형 표시이며 AWS 프로비저닝을 수행하지 않습니다.
Helm/Terraform 노드는 설계용이며 해당 산출물 생성기는 후속 구현 대상입니다.
운영 SSH 키나 비밀 값을 이 화면에 입력하지 마세요.
