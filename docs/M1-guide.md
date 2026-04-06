# M1-guide.md — VM 모드 MVP 설계 가이드

M1 구현의 배경, 설계 결정 이유, 핵심 개념을 설명합니다.
"무엇을 만드는지"는 `M1-PLAN.md`, "어떻게 만드는지"는 `M1-AGENT.md`를 참고합니다.
이 문서는 "왜 이렇게 설계했는지"를 다룹니다.

---

## 이 도구의 포지셔닝

go-deployer는 **Infrastructure Bootstrap CLI**입니다.
CI/CD 파이프라인(Jenkins, GitLab CI)이 아닙니다.

| 구분 | CI/CD 파이프라인 | go-deployer |
|---|---|---|
| 실행 시점 | 코드 변경마다 자동 | 프로젝트 초기 1회 (또는 재셋업 시) |
| 핵심 목표 | 빠른 배포, 롤백, 무중단 | 올바른 초기 상태 보장 |
| 재실행 시 | 새 버전 배포 (의도적) | 이미 된 단계는 skip |
| 대상 사용자 | 자동화 시스템 | 개발자/인프라 담당자 (사람이 직접 실행) |

M1은 이 도구의 첫 번째 마일스톤으로, "정상 경로가 동작하는 것"을 목표로 합니다.
완전한 멱등성은 M2에서 완성합니다.

---

## 왜 Go인가

- **단일 바이너리**: bastion에 파일 하나만 올려두면 됩니다. Python처럼 런타임 환경이 필요 없습니다.
- **표준 라이브러리 충분**: SSH, SFTP, SHA256, 템플릿 렌더링 모두 외부 의존성 없이 가능합니다.
- **goroutine**: M4 병렬 실행 확장이 자연스럽습니다.
- **AWS SDK**: M5 ECR/ECS 확장에도 aws-sdk-go-v2가 잘 지원됩니다.
- **일관성**: 팀 내 Ansible 경험자가 없어 새로운 도구를 배우는 비용이 큽니다. Go 단일 언어로 유지하는 것이 유지보수 면에서 유리합니다.

---

## 왜 Ansible을 쓰지 않는가

Ansible이 강력한 도구인 것은 맞지만, 현재 상황에서는 오히려 부담입니다.

- **팀 내 경험자 없음**: Python 버전 충돌, 인벤토리 관리, YAML 들여쓰기 디버깅을 배우는 비용
- **OS 혼용 대응**: Ansible 모듈 호환성을 직접 확인해야 합니다
- **단일 언어 원칙**: 프로젝트 전체를 Go로 유지하면 팀이 한 가지 도구만 관리합니다

다만 M5 이후 AWS 리소스 생성 범위가 커지면 Terraform과의 역할 분담을 다시 검토합니다.

---

## 단일 바이너리 vs 분리 바이너리

VM 모드와 Docker 모드를 하나의 바이너리(`deploy`)로 만들기로 결정했습니다.

두 모드가 SSH, Config 파서, Logger, SCP 전송을 100% 공유합니다.
분리하면 `internal/common` 패키지 설계를 별도로 해야 하고, bastion에 올릴 파일도 2개가 됩니다.

```bash
deploy vm     --config deploy.yml   # VM 모드 (M1~M2)
deploy docker --config deploy.yml   # Docker 모드 (M3)
```

향후 모드별 기능이 크게 늘어나면 그때 분리를 검토합니다.

---

## 실행 환경 구조

```
Bastion host
├── deploy (바이너리)
├── deploy.yml
├── dist/
│   └── auth-service.jar        ← 여기서 VM으로 SCP 전송
└── config/
    ├── application-prod.yml
    └── logback-prod.xml

VM (192.168.1.10)
└── /opt/auth-service/          ← base_dir
    ├── bin/auth-service.jar
    ├── config/
    │   ├── application.yml
    │   └── logback.xml
    ├── scripts/
    │   ├── start.sh
    │   └── stop.sh
    ├── logs/
    └── run/
        └── auth-service.pid
```

---

## jar 전송: SCP + SHA256

수백 MB 파일을 매번 전송하면 실용적이지 않습니다.
원격 파일의 SHA256과 로컬 파일의 SHA256을 비교해서 동일하면 전송을 건너뜁니다.

```
로컬 SHA256 계산
        ↓
원격 SHA256 계산 (SSH: sha256sum {path})
        ↓
동일? → SKIP
다름? → SFTP 전송
```

원격 파일이 없는 경우는 에러가 아니라 "전송 필요" 상태로 처리합니다.
설정 파일(application.yml, logback.xml)도 동일한 방식으로 처리합니다.
M2에서는 이 원칙을 나머지 모든 단계로 확장합니다.

---

## 셸 스크립트 템플릿 방식

start.sh를 직접 작성하지 않고 `text/template`으로 렌더링합니다.

```
start.sh.tmpl (git에 저장)
    + ScriptData (deploy.yml 값)
    → start.sh (VM에 배포)
```

서비스마다 `app.name`, `port`, `jvm` 설정이 다르기 때문입니다.
템플릿은 git으로 버전 관리하고, 실제 값은 yml에서 주입합니다.
사용자 지정 템플릿 경로(`script.template`)를 지정하면 내장 템플릿 대신 사용합니다.

내장 템플릿은 `embed.FS`로 바이너리에 포함해서 배포 시 별도 파일 전달이 필요 없습니다.

---

## 프로세스 관리: nohup + PID 파일

systemd 서비스 등록 대신 nohup + PID 파일 방식을 선택했습니다.

| 방식 | 장점 | 단점 |
|---|---|---|
| systemd | 서버 재시작 시 자동 복구 | root 권한 필요, 서비스 파일 관리 필요 |
| nohup + PID | 권한 불필요, 단순 | 서버 재시작 시 수동 재기동 필요 |

이 도구는 초기 부트스트랩 용도이고 서비스 재시작 자동화는 운영 파이프라인의 역할입니다.
단순함을 우선합니다.

---

## deploy.yml 설계 원칙

**선언형 (Desired State)**: "이 명령을 실행해라"가 아니라 "이 서버는 이런 상태여야 한다"를 정의합니다.

**환경변수 치환**: 민감 정보는 yml에 직접 쓰지 않습니다.

```yaml
extra_opts: "-Ddb.password=${DB_PASSWORD}"
```

실행 시 환경변수 값으로 치환됩니다.
치환되지 않은 `${VAR}`가 남아있으면 실행 전에 오류로 처리합니다.

---

## 공유 레이어 설계 의도

M1에서 만드는 패키지들은 M3 Docker 모드의 기반이 됩니다.

```
M1에서 설계              M3에서 재사용
────────────────────────────────────────
internal/ssh   ───────→  docker login, docker run
internal/scp   ───────→  설정 파일 배포
internal/config ──────→  동일한 yml 파서
pkg/logger     ───────→  동일한 로그 형식
```

특히 `ssh.Runner` 인터페이스는 M2 단위 테스트를 위해 지금 정의해두는 것이 중요합니다.
나중에 인터페이스를 추가하면 기존 코드를 수정해야 하는 blast radius가 커집니다.

---

## M1에서 의도적으로 제외한 것들

| 항목 | 제외 이유 | 담당 마일스톤 |
|---|---|---|
| 디렉토리 존재 확인 후 skip | 정상 경로 우선 | M2 |
| JVM 프로세스 실행 중 확인 | 동일 | M2 |
| OS 판별 (Ubuntu/CentOS) | Ubuntu 전제 허용 | M2 |
| JDK 자동 설치 | Java 설치 전제 | M2 |
| 유틸 서비스 설정 (`utils[]`) | 코어 안정화 후 | M2 |
| 병렬 실행 | 단일 서버 검증 우선 | M4 |
| `--dry-run` / `--target` | UX는 코어 이후 | M4 |
| Docker 모드 | 공유 레이어 검증 후 | M3 |
| known_hosts SSH 검증 | 내부망 환경 전제 | M4 |

이 항목들이 필요해 보여도 M1에서는 `TODO(M{N})` 주석만 남깁니다.

---

## 테스트 전략

M1은 단위 테스트보다 통합 검증을 우선합니다.
외부 의존성이 없는 순수 함수(config 파싱, 템플릿 렌더링)만 단위 테스트를 작성합니다.
SSH / SFTP 관련 코드는 로컬 Docker SSH 서버로 통합 테스트합니다.

```bash
docker run -d -p 2222:22 \
  -e PUBLIC_KEY="$(cat ~/.ssh/id_rsa.pub)" \
  lscr.io/linuxserver/openssh-server:latest
```

`Runner` 인터페이스를 지금 정의해두면 M2에서 Mock 기반 단위 테스트를 쉽게 추가할 수 있습니다.