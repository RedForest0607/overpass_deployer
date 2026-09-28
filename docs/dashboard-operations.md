# Dashboard 운영 범위와 재시도

`deploy dashboard --config deploy.yml`은 VM의 Java 애플리케이션을 조회·제어한다. 실행 원본 JAR가 없어도 monitoring 전용 설정으로 진입할 수 있지만, 선택 앱을 실제 배포할 때는 로컬 파일과 전체 배포 설정을 다시 검증한다. Container만 있는 설정은 Docker/Compose CLI 사용 안내와 함께 중단한다.

## 조회와 제어

| 키 / 버튼 | 범위 | 동작 |
|---|---|---|
| `r` / Refresh | 설정의 모든 VM 앱 | 읽기 전용 상태 조회 |
| `t` / Retry status | 선택 앱 하나 | SSH·상태 조회 재시도, 다른 앱 결과 유지 |
| `l` / Log | 선택 앱 하나 | 최근 200줄과 이후 로그 스트림 |
| `s`, `x`, `R` | 선택 앱 하나 | 확인 후 start / stop / restart |
| `d`, `D` | 선택 앱 하나 | 확인 후 dry-run / 실제 배포 |
| `y` / Retry action | 선택 앱의 마지막 실패 작업 | 대상·작업·직전 오류 확인 후 재실행 |
| `q` / Ctrl-C | 대시보드 | 종료. 변경 작업 실행 중에는 완료까지 종료를 보류 |

결과 표에는 서버, 앱, 작업, done/failed, 소요 시간과 오류가 표시된다. 최근 50개 결과만 화면에 유지하되 마지막 앱별 변경 작업은 별도로 보관해, 전체 상태 갱신 때문에 실패 재시도가 사라지지 않는다. 성공한 후에는 같은 앱의 이전 실패를 다시 실행하지 않는다. 상태 조회 실패에는 `t`, 제어·배포 실패에는 `y`를 사용한다.

작업 확인창은 열린 시점의 대상을 유지한다. 이후 선택 행이 바뀌어도 다른 앱을 변경하지 않는다. SSH 실패나 timeout은 자동으로 원격 변경 작업을 반복하지 않는다. 오류 전에 일부 작업이 완료됐을 수 있으므로 작업 결과와 상태를 확인한 뒤 재시도를 승인한다. 조회 또는 변경 작업이 진행 중이면 중복 실행을 차단한다.

## 선택 앱과 태그 필터의 차이

대시보드는 선택한 서버/앱 하나를 명시적으로 실행한다. 해당 앱의 SSH·디렉터리 기본값을 복사하되 서버 bootstrap, 공통 파일, utility, bastion 동기화는 수행하지 않는다. 변경할 앱을 정확히 아는 운영 제어에 사용한다.

CLI의 `vm --server-tag ... --app-tag ...`는 태그가 일치하는 서버/앱 집합을 선택한다. 같은 종류의 여러 태그는 OR로 적용되며 서버 필터와 앱 필터는 모두 만족해야 한다. CLI의 서버 단위 작업 범위는 유지되므로 대시보드의 단일 앱 배포와 동일하게 취급하지 않는다. 일치 대상이 없으면 중단한다.

## local-file 모니터링 규약

`script.mode: local-file`에서도 원격 스크립트는 실행 가능해야 하며 `start`, `stop`, `restart`를 지원해야 한다. 상태 조회는 스크립트의 임의 `status` 출력에 의존하지 않고 실제 `java` 프로세스의 명령행을 검사한다. 기본 literal 검색 문자열은 앱 이름이고 `monitor.process_match`로 JAR 이름 등 고유 문자열을 지정할 수 있다. 일반 셸 프로세스나 다른 언어의 실행 파일은 이 규약에 포함하지 않는다.

로그 기본 경로는 `<base_dir>/logs/<app.name>.log`이며 `monitor.log_path`로 원격 절대 경로를 지정한다. 로그 조회는 디렉터리·파일을 만들지 않으며, 다른 앱 선택 또는 대시보드 종료 시 SSH tail 세션을 닫는다. DEV custom 앱과 PRD read-only 인수는 이 규약에 따라 별도로 확인한다.

## VM / Docker / Compose 공통 UX

세 모드의 CLI는 dry-run, 명시적 범위 선택, SSH timeout/host-key 검증과 결정적인 서버별 done/skipped/failed 결과를 제공한다. VM은 `--app-tag`, Docker는 `--container-tag`, Compose는 `--project`로 서버 내부 대상을 선택한다. Docker와 Compose는 digest 고정·health 확인·실패 rollback을 제공한다. VM 파일 배포 전체가 transaction 또는 자동 rollback이라는 의미는 아니다.

현재 TUI 대시보드의 상태·로그·제어 버튼은 VM Java 앱만 대상으로 한다. Docker와 Compose의 원격 변경은 각 CLI에서 실행하며, 대시보드가 같은 설정의 container나 compose project를 묵시적으로 적용하지 않는다. 컨테이너용 대시보드는 별도 후속 범위다.
