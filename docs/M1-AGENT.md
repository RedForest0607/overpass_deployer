# AGENT.md

AI 코딩 에이전트(Claude Code, Cursor 등)를 위한 프로젝트 지시서입니다.
코드를 생성하거나 수정할 때 이 문서의 규칙을 항상 따릅니다.

---

## 프로젝트 개요

**go-deployer** — Infrastructure Bootstrap CLI  
Bastion host에서 여러 VM에 Java 애플리케이션 실행 환경을 초기 구성하는 도구입니다.

Jenkins/GitLab CI 같은 CI/CD 파이프라인이 아닙니다.  
"이 서버는 이런 상태여야 한다"를 `deploy.yml`에 선언하면,  
현재 상태와 비교해 필요한 작업만 수행합니다.

**현재 마일스톤: M1 — VM 모드 MVP**  
단일 서버, 순차 실행, 정상 경로(happy path)만 구현합니다.  
멱등성 완성, 병렬 실행, Docker 모드는 이후 마일스톤 범위입니다.

---

## 기술 스택

| 항목 | 결정 |
|---|---|
| 언어 | Go 1.22 |
| SSH / SFTP | `golang.org/x/crypto/ssh` |
| YAML 파싱 | `gopkg.in/yaml.v3` |
| 템플릿 | `text/template` (표준 라이브러리) |
| SHA256 | `crypto/sha256` (표준 라이브러리) |
| CLI | `flag` (표준 라이브러리) |

외부 라이브러리를 추가할 때는 반드시 이유를 주석으로 명시합니다.  
표준 라이브러리로 가능한 것은 표준 라이브러리를 사용합니다.

---

## 디렉토리 구조

```
go-deployer/
├── cmd/
│   └── deploy/
│       └── main.go
├── internal/
│   ├── config/
│   │   ├── config.go
│   │   ├── loader.go
│   │   └── validator.go
│   ├── ssh/
│   │   └── client.go
│   ├── scp/
│   │   └── transfer.go
│   ├── template/
│   │   ├── renderer.go
│   │   └── templates/
│   │       ├── start.sh.tmpl
│   │       └── stop.sh.tmpl
│   └── vm/
│       ├── runner.go
│       ├── dirs.go
│       └── files.go
├── pkg/
│   └── logger/
│       └── logger.go
├── templates/           # 사용자 커스텀 템플릿 (선택)
├── deploy.yml
├── go.mod
└── go.sum
```

`internal/` 패키지는 외부 import가 불가능합니다.  
`pkg/logger`는 내부 및 향후 외부에서 모두 사용 가능합니다.

---

## 코딩 규칙

### 에러 처리

```go
// 항상 fmt.Errorf + %w 로 컨텍스트를 포함합니다
if err != nil {
    return fmt.Errorf("transferring jar to %s: %w", host, err)
}

// 에러 메시지는 소문자로 시작합니다 (Go 관례)
// Good: "reading config file"
// Bad:  "Reading config file"

// 함수 반환값이 error 하나뿐일 때도 명명 반환값을 쓰지 않습니다
```

### 인터페이스

SSH 클라이언트는 반드시 인터페이스로 정의합니다.  
M2에서 단위 테스트 Mock 작성에 필요합니다.

```go
// internal/ssh/client.go
type Runner interface {
    Run(cmd string) (string, error)
    RunSudo(cmd string) (string, error)
    Host() string
    Close()
}
```

다른 패키지에서 SSH Runner를 받을 때는 구체 타입이 아닌 이 인터페이스를 씁니다.

### 구조체 설계

- Config 구조체는 `deploy.yml` 스키마와 1:1 대응되게 설계합니다
- 중첩 구조체는 별도 타입으로 정의합니다 (익명 구조체 사용 금지)
- yaml 태그와 필드명은 스키마 문서와 동일하게 맞춥니다

```go
// Good
type AppConfig struct {
    Name    string    `yaml:"name"`
    BaseDir string    `yaml:"base_dir"`
    Jar     JarConfig `yaml:"jar"`
}

// Bad — 익명 구조체
type AppConfig struct {
    Name string `yaml:"name"`
    Jar  struct {
        LocalPath string `yaml:"local_path"`
    } `yaml:"jar"`
}
```

### 상수 및 기본값

기본값은 상수로 정의합니다. 매직 넘버를 코드에 직접 쓰지 않습니다.

```go
// internal/config/config.go
const (
    DefaultSSHTimeoutSec  = 30
    DefaultJvmMin         = "256m"
    DefaultJvmMax         = "1g"
    DefaultScriptTemplate = "" // 비어있으면 내장 템플릿 사용
)
```

### 파일 경로

`~` 확장은 항상 명시적으로 처리합니다.

```go
func expandHome(path string) string {
    if strings.HasPrefix(path, "~/") {
        home, _ := os.UserHomeDir()
        return filepath.Join(home, path[2:])
    }
    return path
}
```

### TODO 주석

M1 범위 밖의 작업은 구현하지 않고 TODO로 표시합니다.

```go
// TODO(M2): known_hosts 기반 검증으로 교체
HostKeyCallback: ssh.InsecureIgnoreHostKey(),

// TODO(M2): 프로세스 실행 중 여부 확인 후 skip
// TODO(M4): OS 판별 분기 (Ubuntu / CentOS)
```

---

## 패키지별 구현 규칙

### `internal/config`

- `loader.go`: YAML 읽기 → 환경변수 치환 → 언마샬 → 기본값 적용
- 환경변수 치환은 파싱 전 raw 문자열 단계에서 수행합니다  
  (`regexp`로 `${VAR}` 탐색 → `os.LookupEnv`)
- 치환되지 않은 `${VAR}`가 남아있으면 validator에서 수집합니다
- `validator.go`: 모든 오류를 한 번에 수집한 뒤 반환합니다  
  오류 발견 즉시 종료하지 않습니다

```go
func validate(cfg *Config) error {
    var errs []string
    if cfg.SSH.User == "" {
        errs = append(errs, "ssh.user is required")
    }
    // ... 모든 검증 후
    if len(errs) > 0 {
        return fmt.Errorf("config validation failed:\n  - %s",
            strings.Join(errs, "\n  - "))
    }
    return nil
}
```

### `internal/ssh`

- `HostKeyCallback`은 `InsecureIgnoreHostKey()` 사용, TODO(M4) 주석 표시
- `Run()`은 stdout + stderr를 합쳐서 반환합니다
- defer로 세션을 닫습니다
- 연결 실패 메시지에 host 주소를 포함합니다

### `internal/scp`

- 원격 SHA256은 SSH로 `sha256sum {path}` 명령 실행 후 파싱합니다
- 원격 파일이 존재하지 않으면 전송으로 처리합니다 (에러 아님)
- 전송 진행 로그: 시작 시, 완료 시, 10MB 단위 중간 로그
- 전송 후 원격 파일 권한 설정이 필요한 경우 호출자가 별도로 처리합니다

### `internal/template`

- 내장 템플릿은 `embed.FS`로 바이너리에 포함합니다

```go
//go:embed templates/*.tmpl
var embeddedTemplates embed.FS
```

- 사용자 지정 템플릿 경로가 있으면 해당 파일을 우선 사용합니다
- 렌더링 결과는 `os.CreateTemp`로 임시 파일에 저장 후 SCP 전송합니다
- 전송 완료 후 임시 파일을 삭제합니다

### `internal/vm`

- `runner.go`의 `Run()` 함수가 단일 진입점입니다
- 각 단계는 독립 함수로 분리합니다 (dirs, files, scripts 등)
- 단계별 에러는 즉시 반환합니다 (M1은 부분 성공 없음)
- 각 단계 시작/완료 시 logger로 기록합니다

### `pkg/logger`

로그 형식: `HH:MM:SS  LEVEL [host] message`

```
15:04:05   INFO [192.168.1.10] Connecting via SSH...
15:04:06     OK [192.168.1.10] Connected
15:04:06   INFO [192.168.1.10] Transferring auth-service.jar (243 MB)...
15:04:18     OK [192.168.1.10] Transferred auth-service.jar
15:04:18   SKIP [192.168.1.10] logback.xml unchanged
15:04:18  ERROR [192.168.1.10] chmod failed: permission denied
```

- 레벨 너비를 고정(5자)해서 메시지 시작 위치를 정렬합니다
- host가 없는 전역 로그는 `[host]` 부분을 생략합니다
- 레벨: `INFO`, `OK`, `SKIP`, `WARN`, `ERROR`

---

## M1 범위 밖 — 구현 금지 항목

아래 항목이 필요해 보여도 M1에서는 구현하지 않습니다.  
TODO 주석만 남기고 넘어갑니다.

| 항목 | 담당 마일스톤 |
|---|---|
| 디렉토리 존재 여부 확인 후 skip | M2 |
| JVM 프로세스 실행 중 확인 후 skip | M2 |
| OS 판별 분기 (Ubuntu / CentOS) | M2 |
| JDK 자동 설치 | M2 |
| 유틸 서비스 설정 배포 (`utils[]`) | M2 |
| 병렬 실행 (goroutine) | M4 |
| `--dry-run` 플래그 | M1 |
| `--target` 플래그 | M4 |
| Docker 모드 | M3 |
| 결과 요약 테이블 출력 | M4 |
| known_hosts SSH 검증 | M4 |

---

## deploy.yml 스키마 (M1)

```yaml
ssh:
  user: ubuntu
  key_path: ~/.ssh/id_rsa
  timeout_sec: 30

servers:
  - host: 192.168.1.10
    # 사용자가 정하는 서버 식별자이며, bastion 사용 시 SSH alias 로도 사용됩니다.
    name: app-server-01

    app:
      name: auth-service
      base_dir: /opt/auth-service

      jar:
        local_path: ./dist/auth-service.jar
        remote_path: /opt/auth-service/bin/auth-service.jar

      jvm:
        min_heap: 512m
        max_heap: 2g

      port: 8080
      extra_opts: "-Dspring.profiles.active=prod"

      config_files:
        - local: ./config/application-prod.yml
          remote: /opt/auth-service/config/application.yml
        - local: ./config/logback-prod.xml
          remote: /opt/auth-service/config/logback.xml

      script:
        template: ""           # 비어있으면 내장 템플릿 사용
        remote_dir: /opt/auth-service/scripts
```

---

## VM 디렉토리 구조 (생성 결과)

```
{base_dir}/
├── bin/
│   └── {app.name}.jar
├── config/
│   ├── application.yml
│   └── logback.xml
├── scripts/
│   ├── start.sh
│   └── stop.sh
├── logs/
└── run/
```

---

## 실행 흐름 (M1)

```
deploy vm --config deploy.yml
  1. deploy.yml 파싱 & 환경변수 치환
  2. 유효성 검증 (실패 시 즉시 종료, 모든 오류 한번에 출력)
  3. SSH 연결
  4. 디렉토리 구조 생성 (mkdir -p 5개 디렉토리)
  5. jar 전송 (SHA256 비교 → 다르면 전송, 같으면 skip)
  6. 설정 파일 배포 (SHA256 비교 → 변경된 것만 전송)
  7. start.sh / stop.sh 렌더링 & 배포 & chmod +x
  8. SSH 연결 종료
  9. 완료 로그 출력
```

---

## 커밋 메시지 형식

```
feat(config): add env var interpolation for ${VAR} syntax
feat(ssh): implement Runner interface and SSH key auth
feat(scp): add SHA256-based skip logic for file transfer
feat(template): embed default start.sh and stop.sh templates
feat(vm): implement full bootstrap flow for single server
fix(scp): handle missing remote file as transfer (not error)
```

형식: `type(scope): description`  
type: `feat` / `fix` / `refactor` / `test` / `docs` / `chore`
