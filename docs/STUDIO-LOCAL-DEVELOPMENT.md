# Studio 로컬 통합 개발 환경

## 목적

Studio 프런트엔드와 Go 백엔드를 한 컴퓨터에서 함께 실행하고, 브라우저의 `/api` 요청을
Vite 개발 프록시를 통해 백엔드로 전달한다. 프런트와 API가 동일 origin처럼 동작하므로
개발 중 별도 CORS 설정 없이 프로젝트·토폴로지 API를 확인할 수 있다.

## 준비 사항

- Go 1.26.1 이상
- Node.js 22.13 이상
- npm
- curl

최초 한 번만 Studio 의존성을 설치한다. 이 명령은 네트워크 접근이 필요하다.

```sh
cd studio
npm ci
```

## 통합 실행

저장소 루트에서 다음 명령을 실행한다.

```sh
make dev-studio-local
```

기본 주소:

- 프런트엔드: `http://127.0.0.1:3000`
- 백엔드: `http://127.0.0.1:8080`
- 영속 데이터: `.cache/studio-local`

종료는 `Ctrl-C`를 사용한다. 실행 스크립트가 자신이 시작한 백엔드 프로세스도 함께 종료한다.
SQLite 데이터는 삭제하지 않으므로 다음 실행에서도 프로젝트와 토폴로지가 유지된다.

## 연결 확인

통합 서버가 실행 중인 다른 터미널에서 다음 명령을 실행한다.

```sh
make smoke-studio-local
```

이 검사는 다음 항목을 확인한다.

1. 백엔드 `/healthz`와 SQLite readiness
2. 프런트엔드 루트 응답
3. 프런트엔드 `/api/v1/capabilities` 프록시

데이터를 생성하거나 기존 로컬 프로젝트를 변경하지 않는다.

## 전체 로컬 테스트

실행 중인 개발 서버와 관계없이 다음 명령으로 양쪽 테스트를 수행한다.

```sh
make test-studio-local
```

이 명령은 Go 백엔드 테스트와 Studio 타입·모델·adapter·계약 테스트를 실행한다.
프로덕션 빌드까지 확인하려면 다음 명령을 추가로 실행한다.

```sh
make build-studio-backend
make build-studio
```

## 설정 변경

| 환경변수 | 기본값 | 용도 |
| --- | --- | --- |
| `STUDIO_LOCAL_BACKEND_ADDRESS` | `127.0.0.1:8080` | 통합 스크립트가 시작할 API 주소 |
| `STUDIO_LOCAL_FRONTEND_HOST` | `127.0.0.1` | 프런트 개발 서버 host |
| `STUDIO_LOCAL_FRONTEND_PORT` | `3000` | 프런트 개발 서버 port |
| `STUDIO_LOCAL_DATA_DIR` | `.cache/studio-local` | SQLite·artifact·package 데이터 경로 |
| `STUDIO_API_PROXY_TARGET` | `http://127.0.0.1:8080` | 프런트만 따로 실행할 때 사용할 API proxy 대상 |

예를 들어 기본 포트가 이미 사용 중이면 다음과 같이 실행한다.

```sh
STUDIO_LOCAL_BACKEND_ADDRESS=127.0.0.1:18080 \
STUDIO_LOCAL_FRONTEND_PORT=13000 \
make dev-studio-local
```

통합 스크립트는 변경된 백엔드 주소를 Vite proxy에 자동 전달한다.

### 기존 프런트 개발 서버가 실행 중인 경우

Vinext는 같은 Studio 디렉터리에서 개발 서버를 하나만 실행한다. 이미 실행 중이라는 메시지가 나오면
기존 프런트 프로세스를 강제로 종료하지 말고 백엔드만 별도 터미널에서 실행한다.

```sh
go -C studio/backend run ./cmd/studio-api \
  --address 127.0.0.1:8080 \
  --data-dir ../../.cache/studio-local
```

기존 프런트가 기본 proxy 대상인 `127.0.0.1:8080`을 사용하면 `make smoke-studio-local`로 바로 확인할 수 있다.
Vite 설정을 변경하기 전에 시작한 서버라면 정상 종료한 뒤 `make dev-studio-local`로 다시 시작한다.

## 개별 실행

백엔드만 실행:

```sh
go -C studio/backend run ./cmd/studio-api \
  --address 127.0.0.1:8080 \
  --data-dir ../../.cache/studio-local
```

프런트만 실행:

```sh
cd studio
STUDIO_API_PROXY_TARGET=http://127.0.0.1:8080 npm run dev
```

백엔드는 보안을 위해 loopback 주소만 허용한다. 외부 기기에서 접근하거나 공개 배포하는 용도로
이 로컬 구성을 변경하지 않는다.
