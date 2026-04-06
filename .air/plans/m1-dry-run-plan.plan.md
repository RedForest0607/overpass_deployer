## 1. Goal
M1에서 `deploy vm --dry-run --config ...` 실행 시 SSH/SFTP 및 원격 파일 변경 없이 서버별 예상 작업을 로그로 출력하도록 구현하고, 관련 마일스톤 문서를 M1 기준으로 정렬한다.

## 2. Approach
현재 배포 흐름은 VM 러너가 직접 SSH 연결, 디렉토리 생성, 파일 전송, bastion 동기화를 호출하는 구조이며 `internal/vm/runner.go:12`, `internal/vm/dirs.go:12`, `internal/vm/files.go:15`, `internal/scp/transfer.go:67`, `internal/vm/bastion.go:19` 어디에도 dry-run 분기가 없습니다. 따라서 M1 범위에서 가장 작은 blast radius를 유지하려면 전역적인 플래그 분산이 아니라 `RunOptions` 같은 명시적 실행 옵션을 도입하고, 실제 side effect가 발생하는 경계(SSH 연결, SFTP 전송, 원격 명령 실행, bastion 파일 갱신)에서만 dry-run 분기를 통제하는 방식이 적합합니다.

이 접근은 문서상 M4로 밀려 있는 기능을 M1로 앞당기면서도 M2/M4 기능(병렬, target filtering, summary table)과 분리해 유지보수성을 지킬 수 있습니다. 또한 dry-run 중에는 템플릿 렌더링처럼 로컬 임시 파일을 만드는 작업도 피해서 “변경 없이 예상 동작 출력”이라는 `PLAN.md:90`의 의미를 M1 기준으로 더 엄격하게 맞춥니다.

## 3. File Changes
- **Modify** `docs/M1-PLAN.md`
  - 현재 완료 기준과 Phase 7 CLI 범위에 dry-run 항목이 없습니다 (`docs/M1-PLAN.md:23`, `docs/M1-PLAN.md:239`).
  - M1 완료 기준, 단계별 구현 작업, 검증 절차, review checklist에 dry-run 기능을 명시적으로 추가합니다.
- **Modify** `docs/M1-AGENT.md`
  - M1 범위 밖 목록에서 `--dry-run`이 M4로 표시되어 있습니다 (`docs/M1-AGENT.md:256`).
  - dry-run만 M1 허용 항목으로 승격하고, `--target`은 계속 M4에 남겨 범위를 분리합니다.
- **Modify** `PLAN.md`
  - 전체 마일스톤 표기에서 dry-run이 M4 UX 항목으로 정의돼 있습니다 (`PLAN.md:84`).
  - M1 목표/범위와 M4 범위 설명을 재조정해 문서 간 불일치를 제거합니다.
- **Create** `cmd/deploy/main.go`
  - `docs/M1-PLAN.md:239`에는 CLI 엔트리포인트가 정의되어 있으나 현재 저장소에는 `package main`이 없습니다.
  - `vm` 서브커맨드, `--config`, `--dry-run` 플래그 파싱과 `config.Load()` → `vm.Run(...)` 호출을 담당하게 합니다.
- **Create** `cmd/deploy/main_test.go`
  - dry-run 플래그 파싱과 서브커맨드 라우팅을 검증하는 최소 단위 테스트를 둡니다.
- **Create** `internal/vm/options.go`
  - `RunOptions` 또는 동등한 구조를 두어 dry-run 여부를 VM 배포 흐름 전체에 명시적으로 전달합니다.
- **Modify** `internal/vm/runner.go`
  - 현재 `Run(cfg *config.Config) error`가 직접 실배포를 수행합니다 (`internal/vm/runner.go:12-28`).
  - `Run(cfg *config.Config, opts RunOptions) error` 또는 하위 호환 래퍼로 바꾸고, 서버별 로그에 dry-run 상태를 반영하며 bastion 후속 동작도 옵션을 따르도록 수정합니다.
- **Modify** `internal/vm/dirs.go`
  - 현재 `runner.RunSudo`/`runner.Run`으로 즉시 원격 디렉토리를 생성합니다 (`internal/vm/dirs.go:12-38`).
  - dry-run일 때는 생성 예정 경로와 명령을 로그만 남기고 실행하지 않도록 바꿉니다.
- **Modify** `internal/vm/files.go`
  - 현재 jar/config/script/chmod 단계가 모두 즉시 side effect를 발생시킵니다 (`internal/vm/files.go:15-63`).
  - dry-run일 때는 전송 대상, 렌더링될 스크립트 경로, chmod 예정 경로만 기록하고 `template.Render` 및 `scp.Transfer`를 호출하지 않게 합니다.
- **Modify** `internal/scp/transfer.go`
  - 현재 `Transfer`는 로컬 SHA 계산 후 원격 SHA 조회, SFTP 생성, 파일 생성까지 항상 수행합니다 (`internal/scp/transfer.go:66-129`).
  - dry-run 옵션을 수용해 “예상 체크섬 비교/예상 업로드” 로그만 출력하고 실제 `sftp.NewClient`, `MkdirAll`, `Create`, `io.Copy`가 호출되지 않도록 분리합니다.
- **Modify** `internal/vm/bastion.go`
  - 현재 bastion 동기화는 SSH 접속 후 `~/.ssh/config`, `known_hosts`를 직접 변경합니다 (`internal/vm/bastion.go:19-49`, `internal/vm/bastion.go:65-95`).
  - dry-run에서는 bastion 접속 자체를 생략하거나, 최소한 변경 예정 alias/known_hosts 작업만 요약 로그로 남기고 원격 명령은 실행하지 않도록 분기합니다.
- **Modify** `internal/scp/transfer.go` 관련 테스트 또는 **Create** `internal/scp/transfer_test.go`
  - dry-run 시 SFTP가 호출되지 않고 예상 로그/결과만 반환하는지 검증합니다.
- **Modify** `internal/vm/bastion_test.go`
  - dry-run에서 bastion 명령 문자열 생성 규칙 또는 요약 로그 포맷이 깨지지 않는지 검증합니다.
- **Create** `internal/vm/runner_test.go`
  - dry-run 옵션이 들어오면 SSH/SFTP/원격 명령 호출 없이 단계 로그만 생성되는지, bastion enabled 상태에서도 mutation이 차단되는지 테스트합니다.
- **Modify** `progress.md` (if exists, else skip)
  - 루트 지침상 주요 마일스톤 변경 또는 파일 구조 변경 시 업데이트가 필요합니다. 실제 실행 단계에서는 존재 여부를 확인한 뒤 파일이 있으면 기능 완료/다음 To-Do를 짧게 반영합니다.

## 4. Implementation Steps
### Task 1: 문서 범위 재정의
1. `docs/M1-PLAN.md:23-31`의 완료 기준에 “`deploy vm --dry-run --config deploy.yml`이 원격 변경 없이 예상 작업을 출력한다”를 추가합니다.
2. `docs/M1-PLAN.md:239-249`의 CLI 단계에 `--dry-run` 플래그 파싱, dry-run 시 `vm.Run` 옵션 전달, dry-run 검증 항목을 추가합니다.
3. `docs/M1-AGENT.md:256-270`에서 `--dry-run`을 M1 허용 범위로 이동시키고, `--target`은 계속 M4 항목으로 남겨 scope creep를 방지합니다.
4. `PLAN.md:84-97`에서 M4 설명의 dry-run 항목을 제거하거나 “advanced targeting/summary only”로 정리하고, M1 범위 문서와 충돌이 없도록 문장을 맞춥니다.

### Task 2: CLI 옵션 및 실행 옵션 구조 도입
1. `cmd/deploy/main.go`를 생성해 `vm`/`docker` 서브커맨드를 정의하고, `vm`에서 `--config`와 `--dry-run`을 처리하도록 구현합니다.
2. `internal/vm/options.go`에 `RunOptions`를 정의하고, 최소한 `DryRun bool` 필드를 둡니다.
3. `internal/vm/runner.go:12-28`을 수정해 `RunOptions`를 받도록 하고, 기존 호출부가 있다면 하위 호환 래퍼 또는 새로운 함수명을 통해 깨지지 않게 조정합니다.
4. `cmd/deploy/main_test.go`를 추가해 `vm --dry-run --config sample.yml`이 올바른 옵션으로 라우팅되는지 검증합니다.

### Task 3: VM 배포 단계별 dry-run 차단 지점 추가
1. `internal/vm/dirs.go:12-38`에서 dry-run일 때 생성 예정 디렉터리 목록과 sudo fallback 의도를 로그만 남기고 실제 `Run`/`RunSudo`를 호출하지 않도록 합니다.
2. `internal/vm/files.go:15-63`에서 jar/config/script/chmod 단계를 dry-run aware 하게 바꾸고, 스크립트는 “어떤 템플릿이 어떤 원격 경로로 배포될지”만 표시하도록 분기합니다.
3. `internal/vm/runner.go:31-60`에서 서버 시작/종료 로그에 dry-run 여부를 반영하고, 실패 메시지에 “planned step”과 “executed step”이 혼동되지 않도록 wording을 분리합니다.
4. `internal/vm/runner_test.go`를 추가해 dry-run 모드에서 단계 순서 로그는 유지되지만 side effect 메서드는 호출되지 않는지 fake runner로 검증합니다.

### Task 4: 파일 전송 및 bastion 변경 차단
1. `internal/scp/transfer.go:66-129`를 옵션 기반으로 리팩터링해 dry-run 시 체크섬 확인/전송 계획만 출력하고 실제 `sftp.NewClient`, `Create`, `io.Copy`를 건너뜁니다.
2. `internal/vm/bastion.go:19-49`에서 dry-run일 때 bastion SSH 접속과 `syncBastionSSHConfig`, `syncBastionKnownHosts` 실행을 막고, 어떤 alias/known_hosts 항목이 갱신될 예정인지 로그만 남기도록 합니다.
3. `internal/vm/bastion.go:65-157`의 기존 명령 빌더는 재사용하되, dry-run 로그가 실제 명령 문자열 전체를 노출할지 요약형으로 남길지 하나의 정책으로 통일합니다.
4. `internal/scp/transfer_test.go`와 `internal/vm/bastion_test.go`를 보강해 dry-run이 원격 변경 함수를 부르지 않음을 보장합니다.

### Task 5: 회귀 방지와 문서 마무리
1. `docs/M1-PLAN.md` 검증 섹션에 dry-run 실행 예시와 기대 로그를 추가합니다.
2. `progress.md`가 존재하면 dry-run 기능 추가와 다음 TODO를 짧게 갱신합니다.
3. 구현 후 아래 3가지 오류 케이스를 self-review checklist로 점검합니다.
   - dry-run인데 `ssh.Connect` 또는 `sftp.NewClient`가 여전히 호출되는 경우
   - dry-run인데 `template.Render`가 임시 파일을 생성해 로컬 상태를 바꾸는 경우
   - bastion enabled 상태에서 dry-run이 `~/.ssh/config`/`known_hosts`를 실제 변경하는 경우

## 5. Acceptance Criteria
- `deploy vm --dry-run --config deploy.yml`는 서버별 예상 작업 로그를 출력하고, 실제 SSH 연결을 시도하지 않는다.
- dry-run 실행 중 `internal/vm/runner.go` 경로에서 `ssh.Connect`가 호출되지 않거나, 최소한 원격 명령/파일 전송이 전혀 수행되지 않는다.
- dry-run 실행 중 `internal/scp/transfer.go`는 `sftp.NewClient`, `MkdirAll`, `Create`, `io.Copy`를 호출하지 않는다.
- dry-run 실행 중 `internal/vm/files.go`는 `template.Render`와 `os.Remove`를 호출하지 않고도 start/stop 스크립트 배포 계획을 로그로 표현한다.
- bastion 설정이 있는 구성에서도 dry-run은 `internal/vm/bastion.go`를 통해 원격 `~/.ssh/config` 및 `known_hosts` 변경을 수행하지 않는다.
- 일반 실행(`--dry-run` 미지정)은 기존 M1 동작을 유지하며 jar/config/script 배포 흐름이 바뀌지 않는다.
- `docs/M1-PLAN.md`, `docs/M1-AGENT.md`, `PLAN.md`에서 dry-run의 소속 마일스톤 표기가 일관되게 M1 기준으로 정렬된다.
- dry-run 관련 단위 테스트가 추가되고, CLI 옵션 파싱 테스트와 VM/scp/bastion 레벨 테스트가 모두 통과한다.

## 6. Verification Steps
- `go test ./internal/vm ./internal/scp ./internal/template ./internal/config`
- `go test ./cmd/deploy`
- 수동 검증 1: 실제 배포 가능한 샘플 설정으로 `deploy vm --dry-run --config deploy.example.yml` 실행 후, 로그에 디렉터리 생성/파일 전송/스크립트 배포/bastion sync 예정 단계가 순서대로 표시되는지 확인합니다.
- 수동 검증 2: 존재하지 않는 로컬 jar 경로를 가진 설정으로 dry-run 실행 시, 설정 검증 오류는 그대로 반환되고 네트워크 단계로 진행하지 않는지 확인합니다.
- 수동 검증 3: bastion enabled 설정으로 dry-run 실행 시, bastion alias/known_hosts 예정 작업은 로그에 보이되 원격 변경은 발생하지 않는지 확인합니다.
- 회귀 검증: `--dry-run` 없이 기존 happy path 실행 시 현재 M1 동작 로그 형식과 단계 순서가 유지되는지 비교합니다.

## 7. Risks & Mitigations
- **Risk:** dry-run 분기가 여러 파일에 흩어져 일부 side effect가 남을 수 있습니다.
  - **Mitigation:** side effect 경계를 `ssh.Connect`, `Runner.Run`, `sftp.NewClient`, `template.Render` 기준으로 식별하고, 각 경계마다 테스트를 둡니다.
- **Risk:** 기존 함수 시그니처 변경으로 현재 테스트나 호출부가 크게 흔들릴 수 있습니다.
  - **Mitigation:** `RunOptions`를 추가하되 하위 호환 래퍼를 유지하거나 변경 범위를 `cmd/deploy/main.go`와 `internal/vm/*`로 한정합니다.
- **Risk:** dry-run 로그가 실제 실행 로그와 너무 다르면 사용자가 결과를 오해할 수 있습니다.
  - **Mitigation:** 기존 logger 포맷을 그대로 유지하고, 메시지에 `DRY-RUN` 또는 `would ...` 표현을 일관되게 적용합니다.
- **Risk:** 문서만 M1로 옮기고 전체 `PLAN.md`와 `docs/M1-AGENT.md`가 동기화되지 않으면 이후 Executor가 잘못된 범위를 따를 수 있습니다.
  - **Mitigation:** 같은 작업 묶음에서 세 문서를 함께 수정 대상으로 명시하고, review 시 문서 간 dry-run 소속 마일스톤 일치 여부를 체크리스트로 확인합니다.