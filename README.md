# overpassDeployer

단일 바이너리 기반 VM bootstrap/deploy CLI입니다.

현재 기준으로 M1(VM 모드 MVP)은 완료되었고, `deploy update`는 사용할 수 있습니다. `deploy docker`는 placeholder 상태이며 아직 구현되지 않았습니다.

핵심 포인트:

- `deploy vm --dry-run`으로 원격 변경 없이 예정 작업을 먼저 검토할 수 있습니다.
- `.jar`, 설정 파일, 추가 파일은 SHA256 비교 기반으로 동일 파일 전송을 건너뜁니다.
- bastion SSH alias, `known_hosts` 동기화 흐름을 지원합니다.
- `deploy.yml`에 원하는 상태를 선언하는 방식으로 배포를 구성합니다.

## 개요

`overpassDeployer`는 CI/CD 파이프라인 자체를 대체하는 도구가 아니라, 사람이 직접 실행하는 초기 bootstrap/deploy CLI에 가깝습니다. bastion이나 운영 점프 호스트에 단일 바이너리와 설정 파일만 두고, VM 대상 서버에 필요한 디렉토리/애플리케이션 자산/스크립트를 선언적으로 배포하는 것을 목표로 합니다.

현재 저장소의 중심 기능은 `deploy vm`이며, SSH 접속, 파일 전송, 스크립트 렌더링, dry-run 검토, bastion 동기화까지 포함합니다.

## 현재 지원 범위

지원:

- `deploy vm --config ...`
- `deploy vm --dry-run --config ...`
- 서버/앱 `tags` 기반 선택 배포: `--server-tag`, `--app-tag`
- bootstrap 패키지/JDK 설정
- `servers[].app` 단일 앱 배포와 `servers[].apps` 다중 앱 배포
- `script.mode: template` 및 `script.mode: local-file`
- `deploy version`
- `deploy update`, `deploy update --check`, `deploy update --version ...`

비지원 또는 미구현:

- `deploy docker`: 아직 미구현(M3)
- M2/M4/M5 범위의 고도화 기능은 로드맵 단계이며 README에서는 구현된 내용만 다룹니다.

주의:

- `deploy.yml`에 적는 로컬 파일 경로는 자동 생성되지 않습니다. 예: `jar.local_path`, `config_files[].local_path`, `script.local_path`, `extra_files[].local_path`
- `ssh.host_key_checking: strict`를 쓰는 경우 `known_hosts` 파일이 미리 준비되어 있어야 합니다.

## 빠른 시작

로컬 개발 환경에서 현재 바이너리 메타데이터를 확인합니다.

```bash
go run ./cmd/deploy version
```

샘플 설정을 기준으로 실제 변경 없이 예정 작업만 확인합니다.

```bash
go run ./cmd/deploy vm --dry-run --config deploy.example.yml
```

릴리스 바이너리로 설치해서 사용하려면 아카이브를 풀고 `deploy version`으로 동작을 확인하면 됩니다. 자세한 절차는 아래 문서를 참고하세요.

- 릴리스 설치: [docs/release-install.md](docs/release-install.md)
- 릴리스 배포/업데이트: [docs/release-distribution.md](docs/release-distribution.md)

## 설정 구조

전체 샘플은 [deploy.example.yml](deploy.example.yml)을 기준으로 보고, README에서는 핵심 블록만 설명합니다.

### `ssh`

공통 SSH 접속 설정입니다.

```yaml
ssh:
  user: deploy
  key_path: ~/.ssh/id_rsa
  host_key_checking: accept-new
  port: 22
  timeout_sec: 30
```

- `user`, `key_path`는 필수입니다.
- `host_key_checking`은 `strict`, `accept-new`, `insecure` 중 하나입니다.
- `strict`일 때는 `known_hosts_path`가 실제 파일이어야 합니다.

### `bastion`

bastion SSH alias와 host key 동기화에 사용합니다.

```yaml
bastion:
  host: bastion.example.com
  alias_user: ec2-user
```

- `bastion.host`가 설정되면 서버 이름 기준 alias를 bastion에 동기화합니다.
- 서버별로 `name`이 고유해야 alias 충돌이 없습니다.

### `bootstrap`

서버 공통 bootstrap 설정입니다. 서버별 bootstrap으로 override할 수도 있습니다.

```yaml
bootstrap:
  packages:
    - nc
    - net-tools
    - unzip
    - wget
    - vim-enhanced
  jdk:
    vendor: corretto
    major: 25
    headless: false
  os_update:
    enabled: true
  timezone:
    name: Asia/Seoul
  swap:
    enabled: true
    path: /swapfile
    size: 4G
```

- `packages`는 설치되지 않은 항목만 `yum` 또는 `dnf`로 설치합니다.
- `jdk.headless: false`이면 `java-<major>-amazon-corretto` 패키지를 설치합니다.
- `timezone.name`이 현재 시간대와 다를 때만 `timedatectl set-timezone`을 실행합니다.
- `swap.enabled: true`이면 지정한 swap 파일을 없을 때만 생성하고, `/etc/fstab`에는 중복 없이 등록합니다.
- `deploy vm --dry-run`과 실제 배포는 시작 시 파일 크기와 단계 수를 기준으로 전체/서버별 예상 배포 시간을 출력합니다.

### `servers[].app` / `servers[].apps`

서버는 단일 앱 또는 다중 앱 구조를 가질 수 있습니다.

```yaml
servers:
  - host: 10.0.1.10
    name: app-prod-a
    tags: [prod, primary]
    app:
      name: sample-api
      tags: [api]
      base_dir: /opt/sample-api
      port: 8080
      jar:
        local_path: ./dist/sample-api.jar
        remote_path: /app/overpass/sample-api/bin/sample-api.jar
      config_files:
        - local_path: ./configs/application-prod.yml
          remote_path: /app/overpass/config/application.yml
      script:
        mode: template
        template: embedded:server.sh.tmpl
        values_file: ./configs/sample-api.server.values.yml
        remote_path: /app/overpass/scripts/server.sh
```

- `app`와 `apps`를 동시에 정의할 수는 없습니다.
- `tags`는 선택 배포에 사용됩니다.
- `config_files`, `extra_files`, `script`는 모두 로컬 파일 존재 여부를 사전 검증합니다.

### `script.mode`

두 가지 모드를 지원합니다.

- `template`: 내장 템플릿 또는 사용자 템플릿을 렌더링해 원격에 배포
- `local-file`: 로컬 스크립트 파일을 그대로 원격에 복사

예시:

```yaml
script:
  mode: local-file
  local_path: ./scripts/server.sh
  remote_path: /app/overpass/sample-api/bin/server.sh
```

### `tags`, `--server-tag`, `--app-tag`

서버와 앱에 태그를 붙여 subset 배포가 가능합니다.

```bash
go run ./cmd/deploy vm --config deploy.example.yml --server-tag prod
go run ./cmd/deploy vm --config deploy.example.yml --app-tag api
go run ./cmd/deploy vm --config deploy.example.yml --server-tag prod,primary --app-tag sample
```

## 주요 명령 예시

```bash
go run ./cmd/deploy version
go run ./cmd/deploy vm --config deploy.example.yml
go run ./cmd/deploy vm --dry-run --config deploy.example.yml
go run ./cmd/deploy vm --config deploy.example.yml --server-tag prod
go run ./cmd/deploy vm --config deploy.example.yml --app-tag api
go run ./cmd/deploy update --check
go run ./cmd/deploy update --version v1.0.0
```

CLI 기준 현재 서브커맨드:

- `vm`
- `version`
- `update`
- `docker` (미구현)

## 프로젝트 구조 요약

```text
cmd/deploy               CLI entrypoint
internal/config          설정 로드/검증/기본값 처리
internal/vm              VM 배포 실행 흐름
internal/ssh             SSH 연결 및 원격 실행
internal/scp             파일 전송 및 checksum 비교
internal/template        스크립트 템플릿 렌더링
internal/update          self-update 및 release 조회
pkg/logger               공통 로그 출력
docs/                    운영/릴리스/검증 문서
infra/aws-test           AWS smoke test Terraform
```

## 추가 문서

- 샘플 설정: [deploy.example.yml](deploy.example.yml)
- 전체 마일스톤 현황: [PLAN.md](PLAN.md)
- 최근 진행 기록: [PROGRESS.md](PROGRESS.md)
- M1 설계 배경: [docs/M1-guide.md](docs/M1-guide.md)
- AWS EC2 smoke test 절차: [docs/aws-test-smoke.md](docs/aws-test-smoke.md)
- 릴리스 설치 가이드: [docs/release-install.md](docs/release-install.md)
- 릴리스 배포/업데이트 가이드: [docs/release-distribution.md](docs/release-distribution.md)

## 로드맵 메모

- M2: VM 모드 멱등성/운영 안정성 강화
- M3: Docker 모드 구현
- M4: UX 및 병렬성 개선
- M5: AWS 확장

현재 README는 구현된 M1 기능과 이미 동작하는 `update` 흐름을 기준으로 작성되었습니다.
