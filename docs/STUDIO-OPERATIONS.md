# Studio 운영·복구 가이드

## 적용 범위

Studio API는 loopback 단일 인스턴스 운영을 전제로 한다. SQLite, content-addressed artifact와 생성 package를 하나의 data directory에 보관하며 원격 배포 실행은 Crosswalk가 담당한다.

이 문서는 Phase 7의 로컬 운영과 사내 SSO reverse proxy 뒤의 공유 구성을 다룬다. API 자체를 공개 주소에 bind하지 않으며 인터넷 공개 서비스 구성은 범위에 포함하지 않는다.

## 인증 모드와 프로젝트 격리

기본 `local` 모드는 기존 개발 환경과 호환되며 모든 요청을 `local-user` 소유자로 처리한다. 여러 사용자가 접근하는 환경에서는 `trusted-header` 모드를 사용한다.

```sh
STUDIO_AUTH_MODE=trusted-header \
STUDIO_AUTH_PROXY_SECRET='replace-with-at-least-32-random-base64url-characters' \
STUDIO_ALLOWED_ORIGINS='https://studio.example.com' \
go run ./cmd/studio-api --data-dir /var/lib/overpass-studio/data
```

`trusted-header`에서는 인증 프록시가 사용자 ID와 proxy secret을 모두 전달해야 하며 누락·변조 요청은 401로 거부한다. 사용자 ID는 요청 context에만 저장되고 모든 project, topology, artifact, generation, package 조회가 해당 owner로 범위화된다. 다른 owner의 알려진 ID 조회도 존재 여부를 노출하지 않고 404를 반환한다. `/healthz`만 인증에서 제외한다.

브라우저의 POST/PUT/PATCH 요청은 `Sec-Fetch-Site: cross-site`이거나 browser fetch metadata가 있으면서 `Origin`이 없으면 거부한다. Origin이 있으면 정확한 allowlist 검사를 통과해야 한다. CLI처럼 fetch metadata를 보내지 않는 클라이언트도 trusted-header 모드에서는 proxy secret 없이는 API를 사용할 수 없다.

same-origin Nginx/SSO 계약과 systemd/API 환경 골격은 `studio/deploy/`에 있다. 외부에서 받은 인증 헤더를 그대로 신뢰하지 말고 Nginx에서 삭제·재설정하며, 백엔드 포트는 loopback 외부에 노출하지 않는다.

## 데이터 구조와 권한

```text
data-dir/
  studio.db       # 프로젝트, 토폴로지, artifact/package metadata
  artifacts/      # immutable 업로드 blob
  packages/       # generated deploy.yml 및 ZIP blob
  tmp/            # 중단 가능한 작업 파일, 백업 제외
```

data directory와 하위 디렉터리는 group/other 접근 권한이 없어야 하며 DB, WAL/SHM과 blob은 `0600` 이하이어야 한다. symbolic link는 허용하지 않는다.

```sh
cd studio/backend
go run ./cmd/studio-admin inspect --data-dir ../../.cache/studio-local
```

검사가 실패하면 경로를 임의로 교체하거나 링크를 따라가지 말고 소유자와 권한을 먼저 확인한다. Studio API는 시작 시 관리 디렉터리를 `0700`, SQLite DB/WAL/SHM을 `0600`으로 고정한다.

## 온라인 백업

백업은 SQLite serialization API로 실행 시점의 committed transaction을 포함한 일관된 이미지를 만든다. artifact와 package는 immutable이므로 checksum을 계산한 뒤 archive 기록 시 다시 확인한다. `tmp/`, orphan blob과 브라우저 진단 자료는 제외한다.

```sh
cd studio/backend
go run ./cmd/studio-admin backup \
  --data-dir ../../.cache/studio-local \
  --output ../../studio-backup-YYYYMMDD-HHMMSS.tar.gz

go run ./cmd/studio-admin verify \
  --input ../../studio-backup-YYYYMMDD-HHMMSS.tar.gz
```

출력 파일이 이미 있으면 덮어쓰지 않고 실패한다. 실패한 archive는 부분 파일임을 오류로 알리고 자동 삭제하지 않는다. 운영자가 원인을 확인한 뒤 별도 승인된 절차로 정리한다.

백업 파일에는 manifest, SQLite snapshot과 artifact/package 원문이 포함되므로 운영 Secret과 동일하게 취급한다. 공유 스토리지로 복사할 때는 조직의 암호화·보존 정책을 적용한다.

## 복구 훈련과 전환

복구 대상은 존재하지 않는 새 data directory여야 한다. 기존 운영 경로에는 복구하지 않는다.

```sh
cd studio/backend
go run ./cmd/studio-admin restore \
  --input ../../studio-backup-YYYYMMDD-HHMMSS.tar.gz \
  --data-dir ../../.cache/studio-restored

go run ./cmd/studio-admin inspect \
  --data-dir ../../.cache/studio-restored

go run ./cmd/studio-api \
  --address 127.0.0.1:18080 \
  --data-dir ../../.cache/studio-restored
```

복구 명령은 archive 전체 checksum을 먼저 검증하고 같은 filesystem의 staging 디렉터리에 압축을 푼다. SQLite `integrity_check`와 권한 검사가 끝난 뒤에만 최종 경로로 rename한다. 기존 경로와 archive entry를 덮어쓰지 않으며 path traversal, symlink와 manifest에 없는 파일을 거부한다.

검증 체크리스트:

1. `/healthz`가 200을 반환한다.
2. 프로젝트 목록과 환경별 topology revision이 백업 시점과 일치한다.
3. artifact 목록과 YAML/XML 원문을 조회할 수 있다.
4. 완료된 generation의 package metadata, preview와 ZIP 다운로드가 동작한다.
5. 새 generation을 한 건 실행해 worker와 package 저장 경로를 확인한다.

전환은 검증된 새 경로로 API를 재기동하는 방식으로 수행한다. 기존 data directory는 즉시 삭제하거나 덮어쓰지 않고 조직 보존 정책에 따라 rollback 대상으로 유지한다.

## 회귀·부하 smoke

```sh
make test-studio-operational
make test-studio-browser-e2e
```

운영 회귀 gate는 backend 전체 race test, vet, 64개 동시 read-only 요청 smoke, request body 제한, panic 응답 비노출, access log의 query/header/body 비기록, 백업→checksum 검증→새 경로 복구를 포함한다.

실제 운영 데이터로 부하를 확인할 때는 복구 훈련용 사본을 사용한다. 운영 SQLite 파일에 직접 벤치마크 write를 수행하지 않는다.

## 로그와 장애 정보

access log는 request ID, HTTP method, URL path, status, response byte와 duration만 기록한다. query string, Authorization header와 request body는 기록하지 않는다. panic payload와 stack trace, 내부 파일 경로는 응답·generation failure·운영 로그에 남기지 않는다.

조사 시 request ID, generation ID, failure code를 기준으로 연관 이벤트를 찾는다. 원문 Secret이나 전체 요청 body를 임시 로그로 추가하지 않는다.

## 롤백 기준

- migration 또는 기동 실패: 기존 바이너리와 기존 data directory 조합으로 복귀한다.
- 복구 archive checksum 실패: 해당 archive를 사용하지 않고 이전 검증본을 선택한다.
- 새 경로 검증 실패: 기존 경로를 유지하며 staging 경로를 자동 삭제하지 않는다.
- package 생성 실패: 원본 generation 이력을 보존하고 snapshot 재시도로 재현한다.
- 인증 전환 후 프로젝트가 보이지 않음: DB를 수정하지 말고 SSO의 안정적인 사용자 ID 매핑과 `STUDIO_AUTH_MODE`를 확인한다. 기존 local 데이터 owner는 `local-user`다.
- proxy secret 불일치: 직전 설정 파일로 Nginx와 API를 함께 되돌린다. 두 프로세스의 secret을 서로 다른 시점에 부분 적용하지 않는다.
