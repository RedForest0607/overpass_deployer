# M3 Docker 운영 Runbook

작성일: 2026-09-17

## 적용 전 조건

- 대상 서버에 Docker Engine과 CLI가 설치되어 있고 SSH 사용자가 daemon socket에 접근할 수 있어야 한다.
- Crosswalk는 Docker를 자동 설치하거나 `sudo` 권한을 추측하지 않는다.
- Registry robot/user token은 설정 파일이 아니라 `docker.registry.token_env`가 가리키는 환경변수로 전달한다.
- 운영 적용 전 `--dry-run`, `--preflight`, `--pull` 순서로 경계를 확인한다.

## 단계별 실행

```bash
export HARBOR_TOKEN='runtime-secret'

deploy docker --dry-run --config deploy.yml
deploy docker --preflight --config deploy.yml
deploy docker --pull --config deploy.yml
deploy docker --config deploy.yml
```

일부 대상만 실행할 때는 tag와 서버 병렬 수를 명시한다.

```bash
deploy docker --config deploy.yml \
  --server-tag wave1,prod \
  --container-tag api \
  --parallel 2
```

필터는 쉼표 목록 중 하나라도 일치하면 선택한다. 일치하는 container가 하나도 없으면 성공으로 위장하지 않고 실행을 거부한다.

## 교체와 rollback

동일 host port를 재사용하므로 현재 교체는 무중단 rolling 방식이 아니다.

1. 기존 container를 중지한다.
2. `<name>-crosswalk-backup`으로 rename하여 보존한다.
3. 새 container를 생성하고 시작한다.
4. image HEALTHCHECK가 있으면 `healthy`, 없으면 `running`을 기다린다.
5. 성공하면 backup을 삭제한다.
6. 실패하면 새 container를 제거하고 backup의 원래 이름과 이전 실행 상태를 복구한다.

backup 이름이 이미 있으면 자동 삭제하지 않는다. 다음을 확인한 후 운영자가 보존·삭제 여부를 결정한다.

```bash
docker container inspect <name>-crosswalk-backup
docker container logs <name>-crosswalk-backup
```

## Smoke 체크리스트

실제 SSH/Registry 환경에서 다음 항목을 순서대로 기록한다.

- 최초 배포: `done`, container running/healthy
- 동일 설정 재실행: `skipped`, container ID 유지
- digest 변경: 새 digest로 교체되고 backup 정리
- 의도적 unhealthy image: 명령 실패, 이전 container 이름·실행 상태 복구
- 잘못된 Registry token: pull/create 전에 실패하고 token 비노출
- 두 서버 `--parallel 2`: 결과 summary 순서는 설정 파일 순서 유지

실환경 smoke는 container와 image cache, Docker credential store를 변경하므로 대상 서버와 Registry가 명시된 경우에만 수행한다.
