# M7 Docker Compose 운영 smoke runbook

## 목적과 실행 경계

이 문서는 M7의 로컬 SSH/SFTP fixture 이후, 격리된 Linux VM과 테스트 전용 Registry에서 실제 Docker Compose 동작을 확인하는 절차다. 운영 VM이나 공유 project에는 바로 실행하지 않는다. 테스트 project 이름, 원격 디렉터리, image repository와 token은 smoke 전용 자산을 사용한다.

자동 fixture는 저장소 루트에서 다음 명령으로 반복 실행한다.

```bash
make test-compose-fixture
```

이 fixture는 외부 네트워크를 사용하지 않으며 신규 적용, 동일 revision skip, 정상 업그레이드, unhealthy rollback을 실제 SSH/SFTP 경로로 검증한다. Docker daemon과 Registry는 fake CLI이므로 아래 실제 smoke를 대체하지 않는다.

실제 smoke는 저장소가 제공하는 `scratch` 기반 health fixture와 자동화 target으로 수행할 수 있다. 실행 시 전용 Registry에 새 image를 push하고 원격 Compose workload를 생성하므로 대상과 승인 범위를 먼저 확인한다.

```bash
export SMOKE_REGISTRY=registry.example.com
export SMOKE_REGISTRY_USERNAME='robot$crosswalk-smoke'
export SMOKE_REGISTRY_TOKEN_ENV=CROSSWALK_SMOKE_TOKEN
export CROSSWALK_SMOKE_TOKEN='<token>'
export SMOKE_SSH_HOST=192.0.2.20
export SMOKE_SSH_USER=deploy
export SMOKE_SSH_KEY=/absolute/path/to/smoke-key
export SMOKE_PROJECT=crosswalk-smoke-commerce
export SMOKE_REMOTE_DIR=/app/crosswalk-smoke/commerce-20260923
export SMOKE_PLATFORM=linux/amd64
make smoke-compose-real
```

`SMOKE_IMAGE_REPOSITORY`, `SMOKE_SSH_PORT`, `SMOKE_PARALLEL`은 선택 설정이다. image repository 기본값은 `<registry>/crosswalk/compose-smoke`다. 스크립트는 기존 remote directory를 재사용하거나 삭제하지 않으며, 성공·실패 진단을 위해 revision과 비밀값을 제외한 로컬 로그를 보존한다. 임시 Docker credential directory만 종료 시 제거한다.

로컬 Podman machine이 있으면 실제 Linux engine·공식 Compose provider·임시 Registry를 묶은 검증을 실행할 수 있다.

```bash
make smoke-compose-podman
```

`PODMAN_SMOKE_MACHINE`으로 머신을 선택할 수 있다(기본값 `podman-machine-default`). 머신 이름과 같은 이름의 `core` 사용자 rootless connection이 필요하며, 해당 connection의 SSH endpoint와 머신 설정이 일치하는지 시작 전에 검사한다. Registry·image·workload 명령과 하위 smoke의 login/build/push 모두 이 connection을 명시적으로 사용한다. 기본 connection이나 `CONTAINER_HOST` 설정을 변경하지 않는다. Linux arm64/amd64에 맞는 provider와 fixture를 선택한다.

이 target은 기존 provider/config/동명 Registry container가 있거나 engine 상태를 조회할 수 없으면 중단한다. Compose provider 설치·checksum 검증은 VM 내부용 `prepare-podman-provider.sh`, 배포 수명주기 검증은 `run-real-smoke.sh`가 담당한다.

성공 시 자신이 만든 workload container/network/revision과 임시 provider/config/Registry container를 제거하고, 시작한 Podman machine을 정지한다. workload 정리에 실패하면 오류를 출력하고 종료 코드 1을 반환하며 revision과 나머지 환경을 보존한다. 배포 수명주기 검증 실패 시에는 진단에 필요한 provider·Registry·workload와 실행 중인 VM을 자동 보존한다. 성공 후에도 환경을 남기려면 `PODMAN_SMOKE_PRESERVE=true`를 사용한다. 보존된 환경은 확인 후 수동 정리가 필요하며, 재실행은 기존 자산을 덮어쓰지 않는다.

connection 선택·실패·정리 경계는 다음 명령으로 검증한다. 이 테스트는 모든 Podman/VM 작업을 shell 함수로 대체하며 실제 다운로드·컨테이너 변경·파일 삭제를 실행하지 않는다.

```bash
node --test crosswalk/tests/compose/podman-smoke.test.mjs
```

## 사전 조건

- 대상은 폐기 가능한 격리 Linux VM이어야 한다.
- SSH 사용자가 Docker daemon과 Compose v2 plugin을 `sudo` 없이 사용할 수 있어야 한다.
- Registry에는 smoke 전용 image가 SHA-256 digest로 등록되어 있어야 한다.
- `deploy.yml`의 `remote_dir`은 smoke 전용 절대 경로여야 한다.
- `token_env`가 가리키는 환경변수는 현재 shell에만 주입하고 YAML에 기록하지 않는다.
- 같은 Compose project label을 사용하는 기존 container가 없어야 한다.
- 자동화 스크립트에 지정한 remote directory는 존재하지 않는 새 경로여야 한다.

## 실행 순서

1. 정적 계획을 검토한다.

   ```bash
   deploy compose --dry-run --config deploy.compose.smoke.yml
   ```

2. Docker, Compose v2와 Registry 인증만 확인한다. `docker login`은 SSH 사용자의 credential 설정을 갱신할 수 있다.

   ```bash
   deploy compose --preflight --config deploy.compose.smoke.yml
   ```

3. 첫 revision을 적용하고 모든 service가 running/healthy인지 확인한다.

   ```bash
   deploy compose --apply --config deploy.compose.smoke.yml
   ```

4. 같은 명령을 다시 실행해 summary가 `skipped`이고 container가 재생성되지 않는지 확인한다.

5. image digest 또는 안전한 환경값을 변경한 두 번째 package를 적용해 `active`가 새 checksum revision을 가리키는지 확인한다.

6. health check가 실패하는 전용 image 또는 설정을 적용한다. 명령은 실패해야 하며 `active`와 실행 중 service가 직전 정상 revision으로 복구되어야 한다.

7. 복구 후 같은 정상 package를 다시 실행해 `skipped`가 되는지 확인한다.

## 서버 확인 항목

```bash
readlink <remote_dir>/active
find <remote_dir>/.revisions -maxdepth 2 -type f -name compose.yml -print
docker compose --project-name <project> --file <remote_dir>/active/compose.yml ps --all
docker ps --filter label=com.docker.compose.project=<project>
```

확인 결과에는 Registry token이나 container 환경변수 값이 포함되지 않아야 한다. `.crosswalk-lock`이 남았다면 다른 배포 프로세스가 없음을 먼저 확인한 후에만 운영자가 정리한다. Crosswalk는 revision을 자동 삭제하지 않는다.

## 실패 판정

- digest와 다른 image가 실행되거나 tag 기반 pull이 발생함
- 같은 revision 재실행에서 `up` 또는 container 재생성이 발생함
- unhealthy 적용 후 `active`가 실패 revision을 계속 가리킴
- rollback 뒤 service가 running/healthy가 아님
- 로그·오류·명령행에 Registry token 또는 환경변수 값이 노출됨

실패 시 추가 apply를 중단하고 `active` 링크, revision checksum, `docker compose ps`, daemon log를 보존한 뒤 원인을 분석한다. rollback도 실패했다면 자동 재시도하지 않는다.
