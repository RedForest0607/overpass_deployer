# M1-PLAN.md — VM 모드 MVP 체크리스트

마일스톤 M1의 상세 구현 체크리스트입니다.
구현 규칙은 `docs/M1-AGENT.md`, 설계 배경은 `docs/M1-guide.md`를 참고합니다.

---

## 상태 범례

```
[ ] 미시작
[~] 진행 중
[x] 완료
[!] 블로킹 이슈
```

---

## 완료 기준

아래 조건을 모두 만족해야 M1 완료입니다.

- [x] `deploy vm --config deploy.yml` 명령이 실행된다
- [x] `deploy vm --dry-run --config deploy.yml` 명령이 원격 변경 없이 예상 작업을 출력한다
- [x] SSH Key 인증으로 VM에 접속된다
- [x] `{base_dir}/bin|config|scripts|logs|run` 5개 디렉토리가 VM에 생성된다
- [x] jar 파일이 bastion에서 VM으로 전송된다
- [x] 동일한 jar 재전송 시 SHA256 비교 후 skip 된다
- [x] `application.yml`, `logback.xml`이 VM에 배포된다
- [x] `server.sh start|stop` 실행 모델을 따르는 서버 제어 스크립트가 템플릿으로부터 렌더링되어 VM에 배포된다
- [x] 배포된 서버 제어 스크립트에 실행 권한(+x)이 부여된다
- [x] 각 단계가 `HH:MM:SS LEVEL [host] message` 형식으로 로그 출력된다

---

## Phase 0 — 프로젝트 초기화

- [x] `go mod init` 및 프로젝트 기본 모듈 구성
- [x] 디렉토리 구조 생성
  ```
  cmd/deploy/
  internal/config/
  internal/ssh/
  internal/scp/
  internal/template/templates/
  internal/vm/
  pkg/logger/
  docs/
  templates/
  ```
- [x] `go.mod` 의존성 추가
  - [x] `golang.org/x/crypto` (SSH + SFTP)
  - [x] `gopkg.in/yaml.v3` (YAML 파싱)
- [x] `deploy.example.yml` 작성 (민감 정보 없는 예시 파일)
- [x] `.gitignore` 작성 및 로컬 산출물/테스트 자산 분리

**Review checklist (Phase 0)**
- [x] blast radius: 파일 생성만, 기존 코드 영향 없음
- [x] rollback: 디렉토리/파일 삭제로 되돌릴 수 있음

---

## Phase 1 — `pkg/logger`

> 모든 패키지가 의존하므로 가장 먼저 구현합니다.

- [x] `logger.go` 구현
  - [x] 레벨 상수 정의: `INFO` / `OK` / `SKIP` / `WARN` / `ERROR`
  - [x] 레벨 너비 5자 고정 (정렬)
  - [x] 형식: `HH:MM:SS  LEVEL [host] message`
  - [x] host 없는 전역 로그 지원 (`[host]` 생략)
  - [x] 현재 코드 기준 API: `Info`, `Ok`, `Skip`, `Warn`, `Error`, `GlobalInfo`, `GlobalError`

**검증**
```
logger.GlobalInfo("starting deployment")
→ 15:04:05   INFO starting deployment

logger.Ok("192.168.1.10", "Connected")
→ 15:04:06     OK [192.168.1.10] Connected

logger.Skip("192.168.1.10", "logback.xml unchanged")
→ 15:04:06   SKIP [192.168.1.10] logback.xml unchanged
```

**Review checklist (Phase 1)**
- [x] requirement coverage: 배포 경로에서 필요한 레벨 + host 유무 조합 구현
- [~] testability: 로그 출력 전용 단위 테스트는 없지만 통합/경계 테스트에서 형식 사용을 검증

---

## Phase 2 — `internal/config`

- [x] `config.go` — 구조체 정의
  - [x] `Config`, `SSHConfig`, `ServerConfig`, `AppConfig`
  - [x] `JarConfig`, `JvmConfig`, `ConfigFile`, `ScriptConfig`
  - [x] 상수: `DefaultSSHTimeout = 30`, `DefaultJvmMin = "256m"`, `DefaultJvmMax = "1g"`

- [x] `loader.go` — 파싱 및 환경변수 치환
  - [x] `Load(path string) (*Config, error)`
  - [x] raw 문자열에서 `${VAR}` 탐색 및 치환
  - [x] 치환되지 않은 `${VAR}` 목록 수집 (validator로 전달)
  - [x] YAML 언마샬 후 `applyDefaults()` 호출
  - [x] `ssh.key_path`의 `~/` 경로 확장

- [x] `validator.go` — 유효성 검증
  - [x] 모든 오류 수집 후 한 번에 반환
  - [x] 필수 필드: `ssh.user`, `ssh.key_path`, `servers[i].host`
  - [x] 필수 필드: `app.name`, `app.base_dir`, `app.jar.local_path`, `app.jar.remote_path`
  - [x] 파일 존재 확인: `app.jar.local_path`, `config_files[i].local`
  - [x] 미치환 `${VAR}` 에러 처리

**검증**
```bash
deploy vm --config broken.yml
# → config validation failed:
# →   - ssh.user is required
# →   - servers[0].app.jar.local_path: file not found: ./dist/app.jar
```

**Review checklist (Phase 2)**
- [x] requirement coverage: 스키마의 모든 필수 필드 검증 포함
- [x] hidden side effects: 파일 존재 확인은 read-only

---

## Phase 3 — `internal/ssh`

- [x] `client.go` 구현
  - [x] `Runner` 인터페이스 정의
    ```go
    type Runner interface {
        Run(cmd string) (string, error)
        RunSudo(cmd string) (string, error)
        Host() string
        Close()
    }
    ```
  - [x] `Client` 구조체 (`Runner` 충족)
  - [x] `Connect(user, host, keyPath, hostKeyChecking, knownHostsPath string, port, timeoutSec int) (*Client, error)`
  - [x] `Run()`: stdout + stderr 합산 반환
  - [x] `RunSudo()`: `sudo ` 접두사 래퍼
  - [x] host key 검증 모드 지원: `strict`, `accept-new`, `insecure`

**검증**
```go
client, _ := ssh.Connect("ubuntu", "192.168.1.10", "~/.ssh/id_rsa", "accept-new", "~/.ssh/known_hosts", 22, 30)
out, err := client.Run("echo hello")   // out == "hello\n"
out, err = client.Run("ls /none")      // err != nil
```

**Review checklist (Phase 3)**
- [x] testability: Runner 인터페이스로 Mock 대체 가능
- [x] rollback: Dial 실패 시 nil 반환으로 이후 단계 실행 차단

---

## Phase 4 — `internal/scp`

- [x] `transfer.go` 구현
  - [x] `Transfer(client *ssh.Client, localPath, remotePath string, opts TransferOptions) error`
  - [x] 로컬 SHA256 계산
  - [x] 원격 SHA256: SSH로 `sha256sum {path}` 실행 후 파싱
  - [x] 원격 파일 없음 → 전송 진행 (에러 아님)
  - [x] SHA256 동일 → `SKIP` 로그 후 전송 생략
  - [x] SHA256 다름 → SFTP 전송
  - [x] 전송 전 원격 디렉토리 보장
  - [x] 전송 진행 로그: 시작 / progress / 완료

**검증**
```
1회차: INFO Transferring ... → OK Transferred
2회차: SKIP auth-service.jar unchanged (SHA256 match)
```

**Review checklist (Phase 4)**
- [x] blast radius: 원격 파일 덮어쓰기 — SHA256 다를 때만
- [~] rollback path: 부분 전송 실패 시 원격 임시 파일/원자 교체는 아직 없음

---

## Phase 5 — `internal/template`

- [x] `templates/start.sh.tmpl`
  - [x] 변수: `AppName`, `JarPath`, `BaseDir`, `Port`, `JvmMin`, `JvmMax`, `ExtraOpts`
  - [x] `server.sh start` 실행 모델 지원
  - [x] 이미 실행 중이면 종료 (PID 파일 + kill -0 확인)
  - [x] `nohup java ... &` + PID 파일 저장

- [x] `templates/stop.sh.tmpl`
  - [x] `server.sh stop` 실행 모델 지원
  - [x] PID 파일 없음 처리
  - [x] `kill ${PID}` + PID 파일 삭제
  - [x] stale PID 처리

- [x] `renderer.go`
  - [x] `ScriptData` 구조체 및 template data merge
  - [x] `//go:embed templates/*.tmpl` 내장 템플릿
  - [x] `Render(tmplPath string, defaultName string, data any) (tmpFile string, err error)`
  - [x] `tmplPath` 비어있으면 내장 템플릿 사용
  - [x] 렌더링 결과를 `os.CreateTemp`로 임시 파일 저장 후 경로 반환

**Review checklist (Phase 5)**
- [x] hidden side effects: 임시 파일 미정리 시 디스크 누수 — defer 확인
- [x] testability: 렌더링 결과와 템플릿 우선순위를 단위 테스트로 검증

---

## Phase 6 — `internal/vm`

- [x] `dirs.go`
  - [x] `CreateDirectories(runner ssh.Runner, app *config.AppConfig, opts RunOptions, host string) error`
  - [x] `{base_dir}/bin|config|scripts|logs|run` 생성
  - [x] 서버 단위 `CreateServerDirectories(...)` 지원

- [x] `files.go`
  - [x] `DeployConfigFiles(...)`, `DeployExtraFiles(...)`, `DeployScripts(...)`
  - [x] 각 파일 `scp.Transfer()` 호출 + skip/transferred 로깅

- [x] `runner.go`
  - [x] `Run(cfg *config.Config) error` 진입점
  - [x] 실행 순서: bootstrap → server dirs/files → app dirs → jar → config → extra files → script/chmod → bastion sync
  - [x] 단계별 에러 시 즉시 반환

**Review checklist (Phase 6)**
- [x] requirement coverage: 실행 순서 7단계 전부 구현
- [~] rollback path: 중간 실패 시 롤백은 없고 즉시 중단

---

## Phase 7 — `cmd/deploy/main.go`

- [x] 서브커맨드 라우팅: `vm` / `docker`
- [x] `--config` 플래그 (기본값: `deploy.yml`)
- [x] `--dry-run` 플래그 (원격 변경 없이 예상 작업 로그 출력)
- [x] 서브커맨드 미입력 / 오입력 시 usage 출력 후 종료
- [x] `vm` → `config.Load()` → `vm.RunWithOptions()`
- [x] `docker` → `"not implemented yet (M3)"` 출력 후 종료

**Review checklist (Phase 7)**
- [x] requirement coverage: vm / docker 두 경로 처리
- [x] requirement coverage: `--dry-run` 옵션이 실행 옵션으로 전달됨
- [x] hidden side effects: `main()`만 `os.Exit`를 호출하고 실제 로직은 `run()`으로 분리

---

## Phase 8 — 통합 검증

- [x] 로컬 SSH 테스트 서버 구동
  ```bash
  podman build -t localhost/amazonlinux-sshd-test -f TEST/amazonlinux/Dockerfile TEST
  podman run -d --name amazonlinux-sshd-test -p 2222:2222 \
    -v ~/.ssh/id_rsa.pub:/tmp/authorized_keys/id_rsa.pub:ro \
    localhost/amazonlinux-sshd-test:latest
  ```

- [x] **시나리오 1 — 최초 실행**
  - [x] 5개 디렉토리 생성 확인
  - [x] jar 전송 확인
  - [x] 설정 파일 2개 배포 확인
  - [x] start.sh / stop.sh 생성 + 실행 권한 확인
  - [x] start.sh 변수 치환 값 확인

- [x] **시나리오 2 — 재실행**
  - [x] jar SKIP 로그 출력
  - [x] 설정 파일 SKIP 로그 출력

- [x] **시나리오 3 — dry-run**
  - [x] SSH/SFTP 연결 없이 단계별 예정 작업 로그 출력
  - [x] bastion alias/known_hosts 예정 작업만 출력
  - [x] 스크립트 렌더링용 임시 파일이 생성되지 않음

- [x] **시나리오 4 — 유효성 검증 실패**
  - [x] 모든 오류 한 번에 출력 후 종료

- [x] **시나리오 5 — SSH 연결 실패**
  - [x] 명확한 오류 메시지 출력

---

## 단위 테스트

- [x] `config/loader_test.go`: 환경변수 치환, 기본값, `~/` 확장
- [x] `config/validator_test.go`: 필수 필드 누락, 파일 미존재
- [x] `template/renderer_test.go`: 변수 치환, 사용자 템플릿 우선 적용

---

## M2로 넘어가기 전 최종 체크

- [x] Phase 0 ~ 8의 M1 범위 항목 완료
- [x] 통합 검증 시나리오 1 ~ 4 통과
- [x] dry-run 통합 검증 시나리오 통과
- [x] 단위 테스트 전부 통과
- [x] `go vet ./...` 경고 없음
- [x] 코드가 M1 범위를 충족하고 일부 M2/M4 기반 작업이 선반영됨
- [x] 루트 `PLAN.md`의 M1 상태를 `[x]`로 업데이트

---

## 이슈 로그

| 날짜 | 내용 | 상태 |
|---|---|---|
| 2026-04-06 | 코드 기준으로 M1 체크리스트 상태를 재평가함 | 진행 중 |
| 2026-04-06 | 핵심 배포 경로는 구현되었지만 통합 검증, 일부 CLI 요구사항, 일부 테스트가 남아 있음 | 진행 중 |
| 2026-04-06 | `deploy docker` placeholder, 예시 설정 파일, 누락 테스트 2종을 추가함 | 진행 중 |
| 2026-04-06 | `deploy vm --dry-run --config ...` 경로와 관련 테스트를 M1 범위로 반영함 | 진행 중 |
| 2026-04-06 | Podman 기반 TEST 이미지로 통합 검증 재수행 | 시나리오 1~5 및 bastion alias/known_hosts 등록까지 확인 완료 |
| 2026-04-13 | 현재 코드 기준으로 obsolete 체크리스트를 정리함 | M1 범위 완료, 일부 후속 기능은 M2/M4 기반 작업으로 분리 |
