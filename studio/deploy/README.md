# Studio same-origin 배포 계약

이 디렉터리는 사내 SSO 앞에서 Studio를 단일 origin으로 제공하기 위한 참조 자산이다. 특정 IdP 제품을 내장하지 않으며, 운영 인증 프록시가 아래 계약을 충족해야 한다.

## 요청 경계

```text
browser -> TLS Nginx -> SSO auth_request
                     -> /       : frontend 127.0.0.1:3000
                     -> /api/*  : API 127.0.0.1:8080
                     -> /healthz: API readiness (인증 제외)
```

- API와 frontend는 loopback에서만 수신한다.
- SSO 검증 성공 응답은 `X-Authenticated-User`에 128 byte 이하의 안정적인 불변 사용자 ID를 넣는다.
- Nginx는 외부 요청의 신뢰 헤더를 전달하지 않고 `X-Studio-Authenticated-User`와 `X-Studio-Proxy-Secret`을 새로 설정한다.
- proxy secret은 32자 이상의 base64url 난수로 만들고 Nginx 설정과 API 환경 파일을 root/서비스 계정만 읽을 수 있게 한다.
- `STUDIO_ALLOWED_ORIGINS`는 사용자가 여는 HTTPS origin과 정확히 같아야 한다.

## 빌드 산출물

저장소 루트에서 `make build-studio-release`를 실행한다. 결과는 다음과 같다.

- `studio/backend/bin/studio-api`
- `studio/backend/bin/studio-admin`
- `studio/dist/`, `studio/package.json`, `studio/package-lock.json`

배포 release는 `/opt/overpass-studio/releases/<release-id>/` 아래에 불변으로 두고 `current` symlink만 검증된 release로 전환한다. API/frontend service 예시는 각 `.service.template`, 인증 프록시 예시는 `nginx.conf.template`, 비밀이 없는 환경변수 골격은 `studio-api.env.example`을 사용한다. frontend release 디렉터리에서 runtime dependency를 준비한 뒤 `npm run start -- --hostname 127.0.0.1 --port 3000`으로 Vinext production server를 실행한다. 의존성 설치가 필요한 경우 승인된 내부 registry와 lockfile 기반 `npm ci --omit=dev`를 사용한다.

## 적용 전 검증

1. `studio-admin backup`과 `verify`로 현재 data directory 백업을 만든다.
2. 새 release에서 backend/frontend 테스트와 빌드를 통과시킨다.
3. Nginx 설정을 별도 staging 파일로 렌더링하고 `nginx -t`를 통과시킨다.
4. API를 재시작한 뒤 loopback `/healthz`를 확인한다.
5. Nginx를 reload하고 SSO 사용자 A/B로 프로젝트 목록 격리와 변경 요청을 확인한다.

## 롤백

코드 오류만 있으면 `current`를 직전 release로 되돌리고 API/frontend/Nginx를 순서대로 재시작한다. migration 이후 이전 바이너리와 DB schema가 호환되지 않으면 기존 data directory를 덮어쓰지 말고 배포 전에 검증한 백업을 새 경로로 복구한 뒤 환경 파일의 `STUDIO_DATA_DIR`을 그 경로로 전환한다. 실패한 release와 data directory는 원인 분석 전 삭제하지 않는다.

`local` 인증 모드의 기존 데이터는 owner가 `local-user`다. `trusted-header` 전환 후 기존 데이터를 계속 사용하려면 SSO가 해당 운영 주체를 `local-user`로 안정적으로 매핑해야 한다. 사용자별 분리가 필요하면 새 사용자 owner로 프로젝트를 생성하고, DB의 `owner_id`를 수동 변경하지 않는다.
