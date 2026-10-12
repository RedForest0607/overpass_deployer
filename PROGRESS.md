# Progress

- 2026-09-28 전체 코드 정리 후속: Crosswalk validator를 SSH·컨테이너·앱·파일·bootstrap·tag 검증으로, VM bootstrap을 package·사용자·system·Compose·service로 분리했다. Studio 토폴로지 palette/results/toolbar와 CSS token/base/topology/delivery를 분리하고 Docker·Compose 생성기의 typed target 모델을 공유해 YAML 생성 후 재파싱을 제거했다. 파일 목록·편집 요청에 프로젝트 범위 취소/응답 식별을 적용하고, ZIP 입력/출력·staging 취소, SSH 명령 기본 10분 제한 및 context 실행, blob 참조 조회의 fail-closed 처리를 보강했다. React ref/effect와 모바일 상태 구독을 정리하고 SSH 취소·원격 롤백의 차이를 문서화했다.

- 2026-09-28 정리 검증: 오류 3가지(프로젝트 전환 후 지연 응답 유입, 취소 중 ZIP 읽기/쓰기 대기, DB 조회·manifest 오류 후 정상 blob 오삭제)를 방지 로직과 회귀 테스트로 검토했다. 프론트 69개 테스트·API drift·typecheck·앱/features/lib 및 모바일 훅 lint·production build, Studio backend 전체 race/vet, Crosswalk config/VM/docker/compose/dashboard/monitor/CLI 및 SSH 신규 취소 테스트 race와 변경 모듈 vet 통과. localhost Chrome E2E 6개 모두 통과(패키지 ZIP, 생성 재시도, 미저장 이동, 지연 파일 목록, 반응형/다크 모드/가져오기 초기화, 저장 충돌). CSS 최종 light/dark token 및 theme alias 값은 보존했다. 테스트 서버 종료를 확인했으며 실제 VM/Registry 접속은 하지 않았다.

- 다음 To-Do: 전체 `npm run lint`에는 공용 UI 템플릿 접근성/타입/React 진단 18개와 Node 테스트 등록의 no-floating-promises 69개가 남아 별도 정리가 필요하다(앱 범위 lint는 통과). 미사용 PackagePreview/preview 파일 삭제는 승인 대기이며 현재 보존한다. production bundle의 500 kB 초과 경고는 후속 lazy-loading 후보로 유지한다. M3/M7 외부 Docker·Registry와 M4 DEV/PRD 인수는 대상·권한 확정 후 진행하며 M5는 미착수다.

- 2026-09-28 M5 이전 로컬 후속 개발: M4 dashboard에 서버/앱별 최근 결과 표, 선택 앱의 read-only 상태 재조회, 확인 후 실패 제어/배포 재시도와 중복 작업 차단을 추가했다. 작업 확인 후 선택이 바뀌어도 원래 대상을 유지하고, 성공 재시도는 이전 실패를 대체하며 상태 갱신·이력 제한으로 재시도 정보를 잃지 않는다. 조회·작업·선택 설정을 파일별로 분리하고 VM Java local-file 규약 및 CLI tag 선택과의 차이를 문서화했다. Studio는 편집 동작·미저장 탐색·패키지 생성·파일 미리보기를 전용 훅으로 분리하고 polling의 취소/리스너 정리·다운로드 취소·생성/이력 재시도 직렬화를 보강했다. backend delivery는 artifact 처리/검사, generation, package 생성/조회로 분리했다. 오류 검토: 잘못된 앱 재시도, 부분 실패 후 중복 실행, 취소된 패키지 응답으로 화면 덮어쓰기 모두 방지 로직과 회귀 테스트로 검토했다. M3/M7 외부 Docker/Harbor 및 M4 DEV custom·PRD read-only 인수는 대상·권한 확정 후 진행하며 M5는 미착수로 유지한다.

- 2026-09-28 검증 결과: 프론트 66개 테스트·API drift·typecheck·변경 파일 lint·빌드, Studio backend 전체 race/test/vet, Crosswalk config/docker/compose/dashboard/monitor/CLI race/test 및 dashboard/monitor/CLI vet 통과. 가상 터미널에서 선택 재조회·확인 후 재시도와 대상 고정을 검증했고 실제 Chrome E2E 4개(artifact→package ZIP, snapshot 재시도, 미저장 이동 취소/버리기, 저장 충돌 시 이동 차단)가 통과했다. 브라우저 검증 중 자동 fitView가 저장된 viewport를 변경해 단순 로드가 dirty로 표시되는 문제를 발견해 저장 viewport로 시작하도록 수정했다. 실제 SSH fixture 및 외부 Docker/Registry·DEV/PRD 검증은 이번 실행에서 제외했다.

- 2026-09-28 복잡성 리뷰 후 1차 정리: Studio에서 화면에 사용하지 않는 VM 초안 생성 호출을 제거하고 JSON 미리보기·다운로드의 직렬화를 하나로 통일했다. Compose 실제 smoke target의 bash 진입점을 수정하고, Podman connection을 선택 머신의 rootless SSH endpoint와 검증한 뒤 Registry/image/정리 및 하위 smoke에 일관되게 적용했다. VM 내부 provider 준비를 별도 스크립트로 분리하고 arm64/amd64를 감지하며 정리 실패를 보고하고 배포 실패·보존 요청 시 진단 환경을 유지하도록 보강했다. 예상 오류 3가지(다른 머신 조작, engine 접속 실패를 자산 미존재로 오인, 정리 실패에도 성공 보고)를 회귀 테스트로 검토했다. shell mock 5개, 프론트 61개 테스트·typecheck·API drift, 변경 파일 lint·Studio build·shell syntax·diff 검사를 통과했으며 실제 VM 다운로드/배포 smoke는 실행하지 않았다. 다음은 Studio 편집·탐색 및 패키지 비동기 훅 분리, delivery service 책임 분리와 외부 Docker/Registry 인수다.

- 2026-09-23 M6.2 Studio Docker 생성기 완료: 격리 임시 Studio API/SQLite를 기동해 Registry·VM·Container topology 저장, package validation, 비동기 `crosswalk-docker` generation, metadata·ZIP 다운로드와 실제 `deploy docker --dry-run`까지 E2E를 실행했다. 동일 harness에서 기존 VM과 신규 Compose package도 각각 실제 Crosswalk dry-run으로 검증했고, Docker package가 `deploy.yml`만 포함하며 token 원문 없이 `token_env`, digest, port, environment 계약을 유지함을 확인했다. 테스트 종료 후 자체 API 프로세스와 `overpass-studio-e2e-*` 임시 디렉터리를 정리했으며 M6.2를 완료 처리했다. 실제 원격 Docker/Harbor smoke는 M3 Phase 5에 유지한다.

- 2026-09-23 M7 Docker Compose Phase 6 실제 Linux engine smoke: 로컬 Podman Linux arm64 VM에 checksum 검증한 공식 Docker Compose v2.29.7 provider와 실제 Registry v2를 임시 구성하고, `scratch` health image를 포트 포함 Registry에 push했다. 신규 적용, 동일 revision skip, v2 교체, unhealthy revision 실패 후 v2 symlink·container 복구가 모두 통과했다. 과정에서 포트 포함 Registry를 거부하던 image 정규식, Podman OCI 기본 포맷의 HEALTHCHECK 유실, push digest 출력 차이, OpenSSH/Go SSH known_hosts 분리, provider 배너의 구조화 출력 오염을 발견해 자동화와 명령 경계를 보강했다. 확인 후 smoke container/network/revision과 임시 provider/config/Registry를 정리하고 VM을 정지했다. 다음은 외부 Docker Engine VM과 조직 Registry에서 같은 자동화를 실행하는 최종 인수다.

- 2026-09-23 M7 Docker Compose Phase 5 실제 smoke 자동화: 외부 base image 없이 빌드되는 `scratch` 정적 health fixture와 전용 Registry push·manifest digest 추출, Crosswalk dry-run/preflight/신규 적용/동일 revision skip/정상 업그레이드/unhealthy rollback을 순차 검증하는 `make smoke-compose-real`을 추가했다. 필수 `SMOKE_*` 대상 검증, 기존 remote directory 재사용 거부, 임시 Docker credential 격리·정리와 원격 revision 보존 경계를 포함했다. 로컬 Docker daemon과 기존 Podman machine이 가용하지 않아 실제 image push·원격 workload 실행은 수행하지 않았으며, 다음은 승인된 격리 Linux VM/Registry 정보를 지정해 자동화를 실행하고 결과를 기록하는 단계다.

- 2026-09-23 M7 Docker Compose Phase 4 격리 통합 검증: 실제 loopback SSH session과 SFTP 전송 위에서 Docker/Compose/Registry만 상태 기반 fake CLI로 대체하는 fixture를 분리 구현했다. 신규 revision 적용, 동일 revision 재실행 skip, 정상 업그레이드, unhealthy 적용 실패 후 이전 revision symlink·service 복구를 하나의 수명주기로 검증하고 `make test-compose-fixture`를 추가했다. 외부 네트워크나 실제 VM/Registry는 사용하지 않았으며, 실제 daemon pull·network/container 동작을 확인할 운영 smoke runbook을 추가했다. 다음은 승인된 격리 Linux VM/전용 Registry에서 runbook을 실행해 M7 인수를 완료하는 단계다.

- 2026-09-22 M7 Docker Compose Phase 3 로컬 구현: `deploy compose --apply` 명시적 실행 경계, 서버 태그/project 필터와 제한 병렬, 생성기 Compose subset 검증, registry digest image 사전 pull/사후 검증, checksum revision 저장과 active 링크 전환, 모든 서비스 running/health 확인 및 이전 revision 복구를 추가. 첫 배포에서 미관리 Compose 컨테이너가 있으면 중단하고, 실패 시 첫 stack을 내리거나 이전 revision을 재적용한다. mock SSH 기반 성공·실패·rollback·이미지 검증 및 Crosswalk 전체 test/vet로 확인하고 실제 VM/Registry smoke는 대기. 다음은 격리 VM에서 신규/업그레이드/롤백/재실행 smoke와 운영 runbook 보강.

- 2026-09-22 Studio UI 가시성·계층 개선: 토폴로지는 미저장 시 저장을, 패키지 생성 후에는 다운로드를 주요 행동으로 강조하고 프로젝트 카드의 열기/수정 비중을 분리했다. 프로젝트·환경 옆에 저장 상태를 단계별 색상과 텍스트로 표시하고, 밝은/어두운 테마의 보조 글씨 대비·버튼 크기·미저장 팝업·모바일 배치를 조정했다. Studio 전체 테스트·타입 검사·변경 파일 lint·빌드 통과. 다음은 실제 화면에서 데스크톱/모바일 줄바꿈과 대비 확인.

- 2026-09-21 Studio 미저장 토폴로지 전환 보호: 프로젝트 또는 환경을 바꿀 때 현재 설계가 수정되었으면 `저장하고 이동`·`저장하지 않고 이동`·`취소` 팝업을 표시하고, 실제 로드 성공 전에는 화면을 전환하지 않도록 수정했다. 저장 실패·충돌 및 저장 요청 중 추가 편집이 발생하면 이동을 중단해 변경 유실을 방지하며 같은 프로젝트/환경으로 돌아갈 때는 현재 편집을 그대로 유지한다. 다음은 브라우저 상호작용 검증 및 M7 Phase 3.

- 2026-09-18 Studio 메뉴 계층·다크 모드: 프로젝트/파일을 공통 메뉴로 유지하고 현재 프로젝트 아래 토폴로지·배포 패키지를 묶었다. 현재 프로젝트 이름 클릭으로 열린 설계로 돌아가며 미선택 시 하위 메뉴는 비활성화한다. 테마 훅과 스타일을 분리해 시스템 테마 초기값, 수동 전환·localStorage 유지·탭 간 동기화와 모바일 메뉴를 추가했다. 저장소 접근 실패, 프로젝트 미선택, 모바일 하위 메뉴 접근을 검토했고 기존 프론트 57개 테스트·타입 검사·API drift를 통과했다. 파일 데이터는 기존 프로젝트별 API 범위를 유지한다. 다음은 브라우저에서 메뉴·테마 시각 검증 및 M7 Phase 3.

- 2026-09-18 M7 Docker Compose Phase 2 로컬 구현 완료: `deploy compose --preflight`가 SSH 연결 후 Docker daemon, Compose v2 plugin, Registry `password-stdin` 인증을 순서대로 확인하며 token 미설정 시 SSH 전에 중단하고 명령·로그·오류에 token을 노출하지 않는다. 명시적 `--sync`는 원격 디렉터리 `0700`, Compose 파일 `0600`을 보장하고 SHA-256이 다른 파일만 원자적 SFTP 전송한 뒤 `docker compose --project-name ... --file ... config --quiet`로 검증한다. 이 단계에서 `pull`/`up`은 호출하지 않고 옵션 없는 apply를 계속 차단한다. mock SSH/전송으로 명령 순서, Secret redaction, Compose 미설치, checksum skip, 권한, config 검증, 복수 서버 실패 집계와 CLI 라우팅을 검증했고 Crosswalk 전체 test/vet와 예제 dry-run을 통과했다. 다음은 Phase 3 digest pull, revision 전환, health 확인과 rollback이다.

- 2026-09-18 M7 Docker Compose Phase 1 완료: Crosswalk 설정에 서버별 `compose_projects` 계약과 project/local file/remote directory 검증을 추가하고, 비밀값 없이 engine·Compose v2·Registry 인증·파일 동기화·config 검증·pull·apply 순서를 보여주는 `deploy compose --dry-run`을 구현했다. dry-run 없는 apply는 Phase 2~3 전까지 명시적으로 차단한다. Studio에는 `crosswalk-compose@1.0.0` capability·validation·비동기 generation·package preview/download와 프론트 생성기 선택을 연결했으며, 기존 Container 토폴로지를 VM별 digest 고정 `compose.yml`로 변환한다. Compose ZIP과 Crosswalk dry-run을 잇는 통합 harness도 VM/Docker 흐름과 함께 확장했다. 프론트 57개 테스트·typecheck·API drift·애플리케이션 lint, Crosswalk 및 Studio 백엔드 전체 test/vet, diff 검사를 통과했다. 격리 임시 디렉터리를 삭제하는 통합 harness 실제 실행은 승인 대기이며, 다음은 Phase 2 SSH/Docker Compose preflight와 checksum 기반 파일 동기화다.

- 2026-09-18 Studio M6.2 Docker 생성기 진행: Blueprint v1에 Registry·Container·VM container placement와 digest 고정 image, host/container port, environment, restart, health timeout 계약을 추가하고 `crosswalk-docker@1.0.0` 생성기를 validation·비동기 generation·package metadata/preview/download lifecycle에 연결. 프론트 노드 팔레트·Inspector·배치·API round-trip·생성기 선택·이력 표시까지 반영했으며 Registry token 값은 저장하지 않고 환경변수 이름만 유지한다. 기존 `registry.secretRef` VM 설계 호환성과 Crosswalk Registry host/username 규칙을 회귀 검증했다. Docker VM 1대에 API/Web Container, Harbor, 외부 PostgreSQL을 구성한 import 가능 `Docker Commerce Demo` 예제를 추가했다. 프론트 55개 테스트·typecheck·API drift, 백엔드 전체 test/vet, 애플리케이션 lint와 diff 검사는 통과. VM/Docker 통합 dry-run harness는 구현했으며 격리 임시 디렉터리 삭제 승인 후 실행 예정.

- 2026-09-17 Crosswalk M4 모니터링 override 구현: 앱별 선택 설정 `monitor.process_match`와 `monitor.log_path`를 추가했다. 기본값은 기존과 동일하게 앱 이름을 포함하는 실제 Java 프로세스와 `{base_dir}/logs/{name}.log`를 사용하며, override process 문자열은 literal match로 처리하고 로그 경로는 원격 절대 경로만 허용한다. 설정 해석을 monitor resolver로 분리하고 shell quoting을 유지했으며, 로그 조회에서 `mkdir`/`touch`를 제거해 대시보드의 log open이 원격 파일을 변경하지 않도록 보강했다. 다음은 서버/앱별 작업 결과 요약과 재시도 가능한 SSH 실패 UX.

- 2026-09-17 Crosswalk M3 Docker Phase 5 로컬 구현: container `tags` 계약과 `--server-tag`/`--container-tag` OR 필터, `--parallel` 제한 서버 worker pool을 추가. 일치 대상이 없으면 오류로 중단하며 한 서버 내부 container 순서는 유지한다. 병렬 완료 순서와 무관하게 설정 순서로 done/skipped/failed 결과와 합계를 출력하고 복수 서버 실패를 모두 집계한다. pull/reconcile이 실제 변경 여부를 반환해 무변경 적용은 skipped로 분류하며 dry-run도 선택 대상 summary를 제공한다. 필터 불변성·no-match·동시성 상한·결과 순서·합계 테스트와 실제 SSH/Harbor smoke 및 backup 복구 runbook을 추가하고 Crosswalk 전체 test/vet, CLI build와 tag filter 예제 dry-run을 통과. 외부 daemon/Registry를 변경하지 않기 위해 실제 Podman/Docker SSH fixture와 Harbor smoke 실행 기록은 대기.

- 2026-09-17 Crosswalk M3 Docker Phase 4 구현: 옵션 없는 `deploy docker`가 preflight와 digest 준비 후 container reconcile을 실행한다. desired image/port/environment/restart를 결정적으로 정렬한 canonical spec hash와 실제 Docker inspect 상태를 비교해 managed·동일·healthy container는 교체하지 않는다. environment 값은 hash 비교 메모리와 권한 600 임시 env-file stdin에만 사용하고 명령·label·오류에는 남기지 않으며 label에는 managed marker와 key 목록만 기록한다. 신규/교체 container는 configurable `health_timeout_sec` 동안 running 또는 image HEALTHCHECK healthy를 확인한다. 다른 상태는 기존 container를 고정 backup 이름으로 보존하고 새 container 정상화 후 삭제하며, create/start/health 실패 시 이전 이름과 실행 상태를 복구한다. 기존 backup 충돌은 덮어쓰지 않고 중단. 신규 생성·full apply·무변경 skip·unhealthy rollback·backup 충돌·secret 전달 테스트를 추가하고 문서에 동일 port 교체의 짧은 중단 시간을 명시했으며 Crosswalk 전체 test/vet, CLI build와 예제 dry-run을 통과. 다음은 Phase 5 tag filter, 제한된 병렬 실행, 결과 요약, Docker fixture와 운영 runbook.

- 2026-09-17 Crosswalk M3 Docker Phase 3 구현: 명시적 `deploy docker --pull` 모드에서 Phase 2 preflight 후 실행 중 container의 image ID와 RepoDigests, 서버의 local image digest 목록을 조회한다. desired `repository@sha256`가 이미 로컬에 있으면 pull을 생략하고, 없을 때만 exact digest를 pull한 뒤 local RepoDigests에 동일 참조가 등록됐는지 재검증한다. 원격 pull 출력과 하위 오류는 credential 보호를 위해 반환하지 않으며 digest 불일치, 잘못된 image ID, 조회 실패 시 container 변경 전에 중단한다. inspect/pull/orchestration을 분리하고 skip·missing pull·사후 mismatch·오류 redaction·CLI routing 테스트를 추가했으며 Crosswalk 전체 test/vet와 CLI build를 통과. 옵션 없는 일반 apply와 container 변경은 Phase 4까지 계속 차단. 다음은 canonical container spec 비교, 동일 상태 skip, 안전한 교체와 rollback.

- 2026-09-17 Crosswalk M3 Docker Phase 2 구현: `deploy docker --preflight`를 추가해 서버별 SSH 연결, Docker CLI/daemon socket 접근, Registry 인증을 image pull이나 container 변경 전에 독립 검증. Registry token은 누락 시 SSH 전에 중단하며 원문을 `docker login --password-stdin`에만 전달하고 명령·로그·오류에는 포함하지 않는다. Docker 미설치/daemon 권한 오류는 자동 설치나 sudo를 시도하지 않고 조치 가능한 메시지로 반환하며, Registry 실패의 원격 출력도 credential 보호를 위해 redaction. orchestration, engine preflight, Registry 인증을 모듈로 분리하고 mock SSH 기반 오류·비밀값 회귀 테스트와 CLI flag 테스트를 추가했으며 Crosswalk 전체 test/vet, CLI build, 예제 dry-run을 통과. 다음은 Phase 3 실행 중 digest 상태 판정, 조건부 pull, pull 결과 digest 재검증.

- 2026-09-17 Crosswalk M3 Docker Phase 1 구현: `docker.registry`와 서버별 `containers[]` 계약을 추가하고 tag 없는 repository + SHA-256 digest, container name, restart policy, TCP/UDP port, 환경변수 key와 서버 범위 name/host-port 충돌을 선검증. Registry token은 YAML 값 대신 환경변수 이름만 선언하며, 새 `internal/docker` plan은 engine 확인→registry login→container inspect→digest pull→reconcile 순서를 결정적으로 만들되 환경변수 값은 로그에 포함하지 않는다. `deploy docker --dry-run --config ...`를 실제 CLI에 연결하고 apply는 후속 executor 전까지 명시적으로 거부. `docs/M3-PLAN.md`와 `deploy.docker.example.yml`을 추가하고 Crosswalk 전체 test/vet와 CLI build를 통과. 다음은 Phase 2 SSH Docker preflight와 `password-stdin` Registry 인증.

- 2026-09-17 Studio M6.1 프론트 계약 적용 완료: 프로젝트·토폴로지·검증·generation 요청 body를 OpenAPI 생성 DTO로 타입화하고, `crosswalk-vm`의 package output과 `crosswalk/v1` capability를 모두 지원할 때만 신규 생성과 실패 snapshot 재시도를 허용하도록 분리된 계약 판정 모듈을 추가. 구형 서버의 누락된 `chartFormats`와 미지원 포맷을 생성 전에 차단하고, package 응답 포맷도 런타임 재검사한다. 생성 화면에는 차트 포맷, `generator@version`, 환경·topology revision을 표시하고 모바일 메타데이터 배치를 보강. 프런트 49개 테스트·typecheck·production build 통과, 브라우저 E2E 명세에 계약 정보 표시 검증을 추가했으며 사용자 요청 범위에 따라 실제 브라우저 실행·호스팅은 수행하지 않음. 다음 주요 마일스톤은 M3 Docker 모드.

- 2026-09-17 Studio Phase 0 및 M6 완료: OpenAPI 3.1을 단일 기준으로 TypeScript 전체 API DTO와 Go request DTO를 결정적으로 생성하는 스크립트, stale 생성물 검사와 기본 test gate를 추가하고 handler/client의 수기 DTO를 생성 타입으로 전환. Go 필수 request 필드는 pointer로, Blueprint/layout은 raw JSON으로 보존해 누락 필드가 zero value로 숨지 않게 회귀 테스트를 추가. Crosswalk 새 차트는 `format: crosswalk/v1`을 출력하고 무표기 기존 차트는 `crosswalk/legacy`로 호환하며 알 수 없는 포맷은 거부한다. 배포 없이 같은 loader/validator를 쓰는 `deploy validate`와 `pkg/chart.ValidateFile`, chart format capability·package metadata 계약과 문서를 추가. 프런트 47개 테스트·typecheck·production build, Studio backend 전체 race/vet, Crosswalk 전체 test/vet, Studio→ZIP→Crosswalk dry-run E2E를 통과. 다음은 M3 Docker 모드 또는 Studio 생성기 확장 중 우선순위를 정해 별도 마일스톤으로 진행.

- 2026-09-17 Studio Phase 7-D 공유·배포 경계 구현: 기존 `local-user` 호환 `local` 모드와 SSO reverse proxy용 `trusted-header` 모드를 분리하고, proxy secret constant-time 검증·제한된 사용자 ID·공개 health check를 추가. 서비스의 고정 owner를 제거해 요청 context owner가 project/topology/artifact/generation/package 전체 query를 범위화하도록 변경했으며 worker는 queued project owner를 DB에서 재조회. Fetch Metadata와 Origin allowlist를 결합한 CSRF 방어, 사용자 A/B 간 목록·직접 ID 접근 격리 통합 테스트를 추가. release build target과 Vinext production start, loopback backend를 유지하는 Nginx auth_request, API/frontend systemd·환경 골격, 배포 전 백업·same-origin 검증·비파괴 롤백 runbook을 `studio/deploy/`에 추가. backend race/vet, frontend 46개 테스트·typecheck/build, release build·production frontend 응답과 Studio→Crosswalk HTTP E2E 통과. Chrome E2E 재실행은 동일 작업 폴더의 기존 Vinext 개발 서버가 있어 사용자 프로세스를 종료하지 않고 보류했으며 직전 Phase 7-B의 2개 시나리오 통과 기록은 유지. 다음은 M6 잔여 Phase 0 항목인 OpenAPI DTO 자동 생성과 Crosswalk chart version/공개 validation 경계 안정화.

- 2026-09-16 Studio Phase 7-C 운영 복구·보안 구현: `studio-admin`에 data directory 권한/symlink 검사, 실행 중 SQLite committed WAL을 포함하는 online snapshot, artifact/package 이중 checksum manifest 백업, archive verify와 기존 경로를 덮어쓰지 않는 staging restore를 추가. 복구 시 path traversal·중복 entry·symlink를 거부하고 SQLite integrity 및 `0700/0600` 권한을 재검사하며, API 시작 시 DB/WAL/SHM도 `0600`으로 고정. generation worker의 panic stack·내부 경로/raw 오류 기록을 안전한 failure code로 교체하고 access log query/header/body 비노출, 64개 동시 read smoke, race/vet 회귀를 추가. 실제 `.cache/studio-local` 49개 entry를 `/private/tmp` archive로 백업·검증·새 경로 복구한 뒤 별도 API에서 health, 프로젝트, generation 이력과 기존 package ZIP 전체 entry 무결성을 확인. 다음은 Phase 7-D 인증·CSRF·project 권한 middleware와 same-origin 배포/롤백 패키징.

- 2026-09-16 Studio 브라우저 E2E 자동화 구현: Playwright와 시스템 Chrome을 사용해 UI 프로젝트 생성, 유효 JAR·설정 업로드, topology JSON import, artifact 추천 매핑, revision 저장, generation 완료, 실제 ZIP 다운로드·내용 검증을 자동화하고, 실패 generation의 immutable snapshot 재시도 계보도 별도 시나리오로 검증. 실행별 data directory 격리, Vite 감시 경로 밖의 trace/screenshot/video 저장, 실제 프런트 포트 기반 Origin 허용, cold Go build readiness 대기를 추가했으며 브라우저 E2E 2개, 프런트 46개 테스트·타입 검사·production build, backend 전체 테스트, Studio→Crosswalk HTTP E2E를 통과. 다음은 Phase 7 운영 보강(DB backup/restore·권한 가이드·request/load smoke·로그 비밀정보 점검)과 JSON Pointer 오류 세분화.

- 2026-09-16 Studio generation 이력·snapshot 재시도 구현: 프로젝트/환경 범위의 최신순 cursor pagination API와 UI를 추가하고, 실패한 generation의 immutable snapshot을 새 generation으로 재등록하는 idempotent retry 경로를 구현. 원본 실패 이력을 보존하면서 `retryOfGenerationId`로 계보를 기록하며, 성공·진행 중 작업 재시도와 다른 환경 cursor 재사용을 차단. SQLite migration, repository/API/client 회귀 테스트와 backend 전체 테스트, 프론트 타입 검사·계약 테스트·production build를 통과. 다음은 실제 파일 fixture 기반 브라우저 조작 E2E와 JSON Pointer 오류 세분화.

- 2026-09-16 iSTOCK-MALL PRD Crosswalk 대시보드 구성: 운영 bastion `/home/ec2-user/overpass-istockmall-prd-build`에 `deploy-dashboard-20260916-r1`과 고정 서버 4대(`app1`, `app2`, `bo1`, `bo2`)의 Java 앱 12개 인스턴스를 담은 `deploy.yml`을 반영. AWS 대상 그룹에서 별도 확인한 Auto Scaling `fo-pcweb`, `fo-mobile`, `fo-api` 인스턴스는 교체 시 IP와 host key가 바뀌므로 고정 SSH 대상인 Crosswalk 설정에서 의도적으로 제외. 실제 앱 디렉터리·`server.sh`·기본 로그 경로를 읽기 전용으로 확인하고 Crosswalk 화면에서 `Status refresh completed: 12 application(s)`를 검증했으며 10개 `RUNNING`, 로그 파일이 없는 `bo1/bo2`의 `po-mobile` 2개는 `STOPPED`로 표시됨. 앱 시작·중지·재시작·배포는 실행하지 않았고 검증용 대시보드 및 tail 프로세스는 모두 종료. 다음은 필요 시 운영 `po-mobile`의 의도된 정지 여부 확인.

- 2026-09-15 Crosswalk 로그 앱 전환 오류 수정: 대시보드 행의 방향키/마우스 선택 변경을 즉시 내부 대상에 반영하고, 이전 앱의 지연된 SSH 로그 연결이 새 선택을 덮어쓰지 못하도록 스트림 세대 검증을 추가. 새 로그 시작 시 기존 출력을 비우며 `Activity and Logs: <app>` 제목을 고정 표시. iSTOCK-MALL DEV에서 `fo-pcweb` 로그 tail 생성, `fo-mobile` 선택 시 기존 tail 종료, 새 로그 요청 시 `/app/overpass/fo-mobile/logs/fo-mobile.log` tail 전환을 확인했고 검증용 원격 프로세스도 정리. 다음은 필요 시 앱별 로그 파일 선택과 검색/필터 기능 추가.

- 2026-09-15 iSTOCK-MALL DEV Crosswalk 대시보드 확장: bastion `/home/ec2-user/overpass-istockmall-dev-build/deploy.yml`과 `dashboard.dev.yml`에 `devwas` Java 앱 10개를 등록하고 기존 OA 전용 설정을 `dashboard.dev.yml.bak.20260915-all-apps`로 보존. 상태 조회가 자체 shell 명령을 Java 프로세스로 오인하던 `pgrep` 방식을 실제 `comm=java` 프로세스 필터로 교체해 `po-mobile=STOPPED` 등 실제 상태를 표시하도록 수정. Crosswalk 전체 테스트·race 검사와 DEV `Status refresh completed: 10 application(s)`를 확인했으며 앱 제어·로그 스트리밍·배포는 실행하지 않음. 다음은 필요 시 좁은 터미널에서도 10개 앱의 스크롤/선택 가시성을 개선.

- 2026-09-15 Studio 통신선·포트 라벨 배치 편집 구현: 기존 둥근 직각 `smoothstep` 형태를 유지한 채 중간 굴곡 축을 넓은 투명 조작 영역과 직접 클릭 가능한 중앙 표시점으로 드래그할 수 있는 custom edge로 확장. 조작점과 포트 라벨은 다른 Flow 요소와 같은 줌 배율로 확대·축소되고, 포트 라벨은 선 전체 길이의 진행률로 저장해 선을 따라서만 이동. 5px 드래그 임계값으로 드래그 종료와 단순 클릭의 연결 설정 동작을 분리하고, 위치는 배포 의미 Blueprint와 분리된 `layout.connections`에 저장·복원하며 프론트 import와 백엔드 strict validation까지 확장.

- 2026-09-15 Studio YAML/XML artifact 직접 편집 구현: 파일 메뉴에서 프로젝트 범위 `.yml`/`.yaml`/`.xml` 원문을 viewport 기반 대형 팝업으로 열고 문법 검증 후 기존 blob을 덮어쓰지 않는 새 immutable artifact 버전으로 저장. 선택 시 현재 토폴로지의 이전 artifact 참조를 새 ID로 교체하며 별도 설계 저장으로 확정. 서버에서 소유 범위·종류·UTF-8·크기·YAML/XML 문법을 재검사. 다음은 artifact 버전 계보/이력 표시와 diff 비교.

- 2026-09-15 Studio artifact 매핑 선택 가독성 개선: 토폴로지 파일 매핑에서 artifact ID 대신 원본 파일명을 주 정보로 표시하고, kind와 축약 ID를 연한 보조 정보로 유지해 동명 파일도 구분 가능하도록 공용 Choice 옵션을 확장.

- 2026-09-14 Studio Backend 비동기 generation worker 단계 구현: generation 요청은 immutable snapshot과 idempotency key를 SQLite에 `queued`로 먼저 저장하고, 프로세스 내 worker가 원 요청과 분리해 `running → succeeded/failed`로 처리. 비정상 종료된 running 작업은 시작 시 최대 3회 범위에서 재등록하고, 영구 artifact 오류는 즉시 실패 처리하며 worker panic을 작업 경계에서 격리. DB 참조가 없고 24시간 이상 지난 content-addressed blob과 upload 임시 파일만 시작 시 정리. 프론트에는 한국어 작업 상태와 시도 횟수를 표시하고 실제 비동기 HTTP→ZIP→Crosswalk dry-run E2E를 통과. 다음은 generation 이력/재진입 API와 브라우저 조작 E2E, JSON Pointer 오류 세분화.

- 2026-09-14 Studio artifact mapping 및 Crosswalk 실생성 E2E 단계 구현: 토폴로지의 Java JAR·설정·추가 파일·script values와 VM/hosted utility 파일 요구사항을 구조화해 프로젝트 artifact 선택·추천·명시적 적용 UI로 연결. 종류 불일치·다른 프로젝트 ID·stale mapping을 프론트에서 차단하고 `crosswalk-vm` 생성기도 JAR/config/archive/file/script-values 용도별 kind를 재검증. ZIP은 일반 전송만 허용하고 현재 Crosswalk extract 지원 형식인 TAR/TAR.GZ/TGZ만 압축 해제 매핑에 노출. 실제 HTTP 업로드→revision 저장→package 검증·생성→ZIP 다운로드→Crosswalk CLI dry-run을 잇는 로컬 E2E harness와 Make target 추가. 다음은 비동기 generation worker/recovery 및 orphan blob 정리.

- 2026-09-11 Studio topology JSON import 구현: 로컬 `.json` 파일 선택과 텍스트 편집기 붙여넣기, export envelope 및 raw Blueprint 정규화, layout 자동 생성, 2 MiB 제한, apiVersion/환경/편집 가능 구조 검증, 미저장 편집 교체 확인을 추가. 가져온 설계는 즉시 서버를 덮어쓰지 않고 dirty 상태로 캔버스에 반영. 다음은 브라우저 import→artifact 치환→저장→package E2E.

- 2026-09-11 iSTOCKMALL DEV Studio topology 샘플 추가: 현재 inventory 기준 `devwas` Java 앱 10개, `devapp` hosted Redis/Elasticsearch, 외부 PostgreSQL, bootstrap/bastion/배치/대표 연결과 편집 layout을 Blueprint v1 JSON으로 작성. 실제 artifact·secret ID는 안전한 교체 placeholder로 유지. 다음은 Studio에 실제 artifact 등록 후 ID 치환과 package ZIP dry-run E2E.

- 2026-09-11 Studio/Crosswalk 배포 표현 보강: 공통 bootstrap, Java app tags, canonical `local-file` script, 설정 파일 전용 전달 계약, hosted Redis/Elasticsearch의 Crosswalk utility 변환과 실제 artifact ZIP 배치를 구현. 전역/노드별 JSON 편집과 schema·capabilities를 맞추고 Crosswalk 실제 loader 호환 fixture를 추가. 다음은 iSTOCKMALL 자산을 등록한 생성 ZIP dry-run E2E와 비동기 worker/recovery.

- 2026-09-11 Studio 프론트 artifact/package 연동 구현: 프로젝트별 JAR·설정·일반 파일·archive·script-values 등록/목록과 JAR 노드 선택, package 목적 검증, `crosswalk-vm` capability 확인, topology revision 기반 generation 상태 조회, `deploy.yml`/텍스트 미리보기, 바이너리 metadata 및 실제 artifact 포함 ZIP 다운로드를 연결. 같은 생성 재시도는 Idempotency-Key를 재사용하고 프로젝트 전환 중 업로드 결과 오염을 차단. 프론트 27개 테스트·production build, backend 전체 테스트, localhost UI/API smoke와 화면 QA 통과. 다음은 실제 브라우저 업로드→저장→생성 E2E 자동화와 비동기 worker/recovery.

- 2026-09-11 Studio Backend artifact/package API 구현: JAR·설정·일반 파일·archive·script-values streaming 등록/목록, content-addressed blob 저장, JAR/ZIP/TAR/TGZ entry·UTF-8·크기 검증, project 범위 artifact resolver를 추가. package 목적 검증, revision·idempotency 보호 generation, immutable topology snapshot, package metadata·deploy.yml preview·실제 artifact 포함 ZIP download를 HTTP API에 연결하고 `crosswalk-vm` capability를 활성화. 다음은 프론트 업로드/선택·package 화면 연결, 비동기 worker/recovery 및 Crosswalk loader harness.

- 2026-09-10 Studio Crosswalk 배포 모델·생성기 core 구현: Blueprint에 공통 SSH/bastion/directory defaults, VM별 bootstrap·공통 파일·utility, Java별 JAR/설정/추가 파일·압축 해제·chmod·JVM/extra opts·script values를 추가. 프론트 API round-trip과 노드별 JSON 편집기를 연결했고, backend `crosswalk-vm` 생성기가 `deploy.yml` 및 실제 artifact ZIP 상대 경로를 결정적으로 생성하도록 구현. 경로 탈출/충돌과 패키징 중 size·SHA-256 변경을 차단하는 테스트를 추가. 다음은 immutable artifact 업로드/조회와 생성·패키지 다운로드 API 연결, Crosswalk 실제 loader 호환성 자동화.

## Completed

- 2026-09-10 목적별 커밋 정리: Crosswalk 순수 경로 이동, VM 멱등성·안전성, 터미널 대시보드, Studio API·백엔드, Studio 프론트 연동, 공통 개발 환경·문서로 분리. 작업 파일의 기존 변경을 보존했으며 로컬 DB·빌드 산출물·운영 자산은 제외. 원격 푸시는 수행하지 않음.
- 2026-09-10 Studio 저장·검증 화면 연동: 기존 SQLite/API 기반을 활용해 실제 프로젝트 생성·조회·수정, DEV/QA/PROD 설계 저장/복원, URL 선택 복원, revision 409 충돌 보호 및 재조회 확인 UI 구현. 서버 design 검증 API와 모델·배치·포트 충돌·Java 설정 검사, 노드/필드 오류 표시 추가. 현재 편집기 미지원 모델은 조용히 변환하지 않고 로드 거부.
- 2026-09-10 검증 결과: 프론트 타입 검사·테스트 20개·production build, Go backend 전체 race 테스트, SQLite 재오픈 복원 및 localhost 화면/API 프록시 smoke 통과. 검토 오류 케이스 3개: 동시 저장 충돌, 오래된 검증 결과, 저장/복원 중 배치·포트 정보 손실. 브라우저 직접 조작 QA는 미실시.
- 2026-09-10 Studio 로컬 통합 개발 환경 구성: `make dev-studio-local`로 Go API와 Vinext를 함께 기동하고 종료 시 자식 백엔드를 정리하며, Vite `/api`·`/healthz` proxy, 환경변수 기반 포트/data-dir 설정, read-only `make smoke-studio-local`, 양쪽 테스트를 묶은 `make test-studio-local` 및 실행 가이드를 추가. 실제 기존 프런트 + 신규 백엔드 조합에서 루트·health·capabilities proxy smoke 통과.
- 2026-09-10 Studio Backend Phase 2 구현: owner 범위 프로젝트 생성·목록(cursor pagination)·조회·revision 수정 API, DEV/QA/PROD별 topology GET/PUT과 transaction 기반 CAS 충돌 응답, 저장용 Blueprint/layout 구조 검사, React Flow ↔ API adapter 및 위치·배치·artifactRef round-trip을 추가. 동시 저장 시 1건 성공/1건 409, 환경 격리, UI 전용 필드 거부, 다른 owner 접근 차단을 테스트했고 backend race/vet/build, Studio 14개 테스트/build, Crosswalk test/build를 통과.
- 2026-09-10 Studio Backend Phase 1 구현: `studio/backend` 독립 Go API, private data directory, SQLite WAL/migration, loopback·Host/Origin/CORS 제한, request ID·구조화 access log·panic recovery·body limit, graceful shutdown, `/healthz`, `/api/v1/capabilities` 및 단위/통합 테스트 추가. backend test/race/vet/build, Studio 계약·기존 테스트/build, Crosswalk test/build 통과.
- 2026-09-10 Studio Backend Phase 0 진행: 전체 API의 OpenAPI 3.1 초안과 discriminated Blueprint v1 JSON Schema를 `studio/schemas`에 추가. Crosswalk 차트 version/공개 validation 경계와 DTO 생성은 후속.
- 2026-09-10 Studio Backend 구현 계획 수립: `studio/backend` Go 모듈, OpenAPI/Blueprint schema, SQLite·로컬 blob 저장, revision 충돌 제어, 안전한 업로드, 비동기 Crosswalk VM 패키지 생성, 프론트 전환 및 Phase별 테스트/완료 기준을 문서화. API 자체는 미구현.
- 2026-09-10 Studio API 계약 초안 문서화: 프로젝트·아티팩트·환경별 토폴로지·검증·비동기 생성·패키지 미리보기/다운로드, DTO 변환, revision 충돌, Secret/업로드 안전성 및 오류 케이스 3개 수용 기준 정리. API 자체는 미구현.
- 2026-09-10 분리 검증: Crosswalk 전체 go test 및 바이너리 빌드·version 실행, Studio 타입 검사·테스트 9개·production build, GoReleaser 설정 검사 통과. 검토한 오류 케이스: 모듈 이동 후 테스트/빌드 경로 단절, 릴리스·문서 상대 경로 단절, 기존 개발 서버의 web/ 경로 참조. 경로 보정 및 서버 재시작으로 대응. 이전 개발 서버가 재생성한 캐시는 .cache/studio-legacy-web-20260910에 보존.
- 2026-09-10 Deployer 하위 산출물 분리: 기존 web/을 studio/로, Go CLI 소스·모듈·샘플·테스트를 crosswalk/로 이동. 기존 변경 사항과 운영 자산 경로는 보존. 독립 빌드·검증 Makefile과 루트 안내 추가, GoReleaser 빌드 위치 보정. 기존 deploy 명령·업데이트 자산명은 호환성 유지.
- 2026-09-09 프론트 1차 구현: web/에 React Flow 기반 Overpass Studio 추가. 샘플 프로젝트, 노드 배치·typed connection, 설정 Sheet, 실시간 검증, 삭제 취소, Blueprint JSON 및 VM YAML 설계 초안·ZIP 미리보기를 구현. 기존 Go CLI/API 및 실제 JAR/Secret 저장·Helm/Terraform 생성은 아직 미연동.
- 2026-09-09 실행 계층 개선: TargetNode/placement 모듈 분리. VM/EC2 내부 다중 Java·Redis·Elasticsearch 배치, 실행 대상 변경, 외부 서비스 구분, 호스트 삭제 시 서비스 보존, 혼합 서비스 포트 충돌 검증 추가. 배치 관계는 JSON에 보존하고 통신선과 분리. TypeScript 검사·단위 테스트 9개·production build 통과. 로컬 실행 전용.
- 2026-09-09 리뷰 수정: SFTP POSIX rename 확장 기반 기존 파일 교체, 대시보드 UI 오류 경로의 동기 큐 대기 제거, 기존 디렉터리 소유권 재검증, bootstrap/서비스/파일 권한/압축 해제 결과 집계 보완 및 회귀 테스트 추가. ensure 명령 내부의 무변경 여부를 확인할 수 없는 경우 실행 성공을 보수적으로 done으로 집계.
- 2026-08-27 기준 `ops/stock_company/current/dev|qa|prd/deploy.yml`에 앱별 산출물 파라미터 정리 추가: 각 앱의 `application.yml`/env/logback 로컬-원격 경로, `server.sh` 템플릿/values 파일/렌더링 파라미터(`ActiveProfile`, Hamonica agent 설정, payments `SPRING_CONFIG_LOCATION`, batch JVM 옵션)를 로컬 스냅샷에 보강
- 2026-08-27 기준 `ops/stock_company/current`를 제외한 기존 환경별 배포 파일 `ops/stock_company/dev/deploy.yml`, `ops/stock_company/stage/deploy.yml`, `ops/stock_company/prod/deploy.yml` 삭제 완료: 현재 구성 기록은 `ops/stock_company/current/deploy.yml` 및 `current/dev|qa|prd/deploy.yml`만 남기도록 정리
- 2026-08-27 기준 iSTOCK-MALL 현재 구성 스냅샷을 환경별 로컬 파일로 분리: `ops/stock_company/current/dev/deploy.yml`, `ops/stock_company/current/qa/deploy.yml`, `ops/stock_company/current/prd/deploy.yml`을 추가해 DEV/QA(stage)/PRD별 bastion/target 서버, 앱 포트, 디렉터리, `server.sh`, 프로세스 관측 상태를 각각 독립적으로 확인할 수 있게 정리
- 2026-08-27 기준 iSTOCK-MALL DEV/QA(stage)/PRD 현재 구성 스냅샷을 로컬 전용 `ops/stock_company/current/deploy.yml`로 작성: 기존 `dev/stage/prod/deploy.yml`과 read-only SSH 조회 결과를 병합해 bastion/target 서버, 앱 포트, 앱 디렉터리, `server.sh`, Java 프로세스 관측 상태를 기록했으며 원격 파일 변경, 배포, 서비스 제어는 수행하지 않음
- 2026-08-26 기준 전체 코드 점검에서 확인한 P1을 보완: 대시보드 선택 앱 배포는 server bootstrap/common files/utils/bastion 동기화를 제외한 독립 설정만 사용하고, 실배포 전 전체 설정 검증을 다시 수행. SFTP 전송은 원격 임시 파일 write/close 성공 후 같은 디렉터리에서 atomic rename으로 교체해 중간 전송 실패가 기존 자산을 손상시키지 않도록 개선
- 2026-08-24 기준 M2 완료: Ubuntu/Debian `apt-get` bootstrap과 OS별 JDK 패키지 선택, `servers[].utils[]`의 디렉터리/파일/서비스 순차 배포 및 사전 검증, 서버별 `done/skipped/failed` 결과 요약을 구현. Podman SSH/SFTP 대상에서 Ubuntu 24.04/OpenJDK 21 및 Amazon Linux 2023/Corretto 17의 1차 배포와 무변경 2차 `skipped` 요약을 확인했고, 호스트 `go test ./...`/`go vet ./...`도 통과
- 2026-08-24 기준 `deploy dashboard`를 공식 `M4 — UX, Reliability & Dashboard` 마일스톤에 편입하고 `docs/M4-PLAN.md` 체크리스트를 추가: 기존 구현/검증 완료 항목과 로그 경로 override, local-file 상태 판별 규약, 결과 요약/재시도 UX 같은 후속 항목을 분리
- 2026-07-31 기준 `deploy dashboard` TUI 추가 완료: `deploy.yml`의 서버/앱 상태와 PID를 SSH로 read-only 조회하고, 기본 `logs/<app>.log` tail, start/stop/restart, 선택 앱 dry-run/실배포(확인 모달)와 기존 VM 배포 로그의 화면 표시를 구현; 선택 배포는 bastion 설정을 변경하지 않도록 분리하고, loopback mock SSH로 host key 등록/상태 조회/제어/로그 stream과 전체 Go 테스트를 검증
- 2026-07-31 기준 iSTOCK-MALL DEV 실제 스트리밍 검증 완료: DEV bastion의 테스트 전용 `deploy-dashboard-20260731` 바이너리와 monitoring 전용 설정으로 `devwas`의 `oa` 상태(`RUNNING`, PID `1665728`, 포트 `25205`)를 조회하고 `/app/overpass/oa/logs/oa.log`의 `tail -n 200 -F` 연결을 확인; 앱 start/stop/restart/deploy는 실행하지 않았고 테스트 대시보드 및 tail 프로세스 종료까지 확인
- 2026-07-31 기준 대시보드 입력 멈춤 및 로그 정리 보완: UI 이벤트 루프에서의 동기 화면 갱신을 비동기 요청으로 분리하고 SSH stdout/stderr 수집 경쟁 상태를 제거; 로그 tail은 `TERM` 신호로 종료하도록 보강하고 마우스 입력을 활성화했으며 DEV 테스트 전용 `deploy-dashboard-20260731-r3`에서 `Log` 클릭과 `l` 로그 표시, `q` 종료 후 원격 tail 미잔류를 검증
- 2026-07-31 기준 앱 제어 및 배포 확인 절차 강화: 확인창이 열린 동안 전역 단축키를 차단하고 모달에 포커스를 고정했으며 DEV 테스트 전용 `deploy-dashboard-20260731-r4`에서 `Stop`과 `Deploy`의 `Cancel`/`Confirm` 확인창 표시 후 `Cancel` 종료를 검증; 실제 앱 제어와 배포는 실행하지 않음
- 2026-07-27 기준 DEV `oa-exp.istockmall.com` Route53 Alias 전환 완료: 기존 `dev-external-alb-554037108.ap-northeast-2.elb.amazonaws.com`에서 `dev-oa-external-alb-1065898717.ap-northeast-2.elb.amazonaws.com`으로 A Alias 변경, Terraform state import 및 관리 전환 완료, 새 OA TG target `healthy`와 실제 `https://oa-exp.istockmall.com/` HTTP/2 200 검증 완료
- 2026-07-27 기준 DEV OA 전용 ALB 1차 생성 완료: Terraform apply로 `dev-oa-external-alb`/HTTPS 443 listener/`dev-oa-external-tg`/전용 SG 및 `dev_was:25205` ingress 생성 완료, 새 ALB 상태 `active`, `devwas(i-03b1b81af986a7fbe):25205` target health `healthy` 확인; `oa-exp.istockmall.com` Route53 Alias는 아직 기존 `dev-external-alb` 유지
- 2026-07-27 기준 DEV `oa-exp.istockmall.com`을 기존 `dev-external-alb`에서 전용 `dev-oa-external-alb`로 분리하기 위한 Terraform 초안 추가: 전용 ALB/HTTPS listener/TG/SG, `devwas:25205` ingress, Route53 hosted zone 조회 및 cutover용 output 구성을 분리 파일로 작성하고, 실제 Route53 Alias 전환은 새 ALB 검증 이후 별도 단계로 보류
- 2026-06-23 기준 서버 수동 변경 흡수 1차 구현 완료: `bootstrap.users[]`/`bootstrap.sysctl[]`, privileged archive staging/sudo extract/owner 선언, prod/prd `.env.sample` 배포 차단 validator, DEV `overpass-payments-api` 앱 선언, batch JVM `-Dbatch.server.no`와 DEV `fo-api` 1g heap 선언, Elasticsearch common archive 복구 및 DEV dry-run 검증 완료
- 2026-06-09 기준 DEV bastion(`54.116.159.65`) 경유 PostgreSQL 서버(`10.20.30.87`)에 PostgreSQL 17.6 custom dump를 신규 UTF-8 `istockdev` DB로 복원 완료: SHA-256/아카이브 검증, `pldbgapi` 소스 빌드 설치, USER MAPPING 제외, `--no-owner --no-acl --exit-on-error --jobs=4` 복원, 단계별 ANALYZE 및 DB/객체/invalid index/대형 테이블 행 수 검증 완료
- 2026-06-09 기준 DEV `batch-api1`/`batch-api2`를 `devapp(10.20.10.202)`에서 `devwas(10.20.10.41)`로 이전 완료: 배포 대상 선언 변경, 전체 파일 SHA256 일치, `server.sh` 문법과 `25251`/`25252` 포트 미사용 확인 후 devapp 원본 및 전송 임시파일 삭제
- 2026-06-09 기준 DEV `devwas`/`devapp`의 실제 앱 env 9개와 로컬 env sample의 MongoDB 주소를 `10.20.20.102:20001,10.20.20.46:20001/?replicaSet=istockdevReplicaSet`으로 변경하고 원격 파일별 백업 생성 완료; `fo-api` Hazelcast 노드는 `10.20.20.102:5701`, `10.20.20.46:5701`로 유지 및 sample에 명시
- 2026-06-09 기준 AK DEV Redis 구성을 iSTOCK DEV용으로 변환해 `devapp(10.20.10.202)`에 Redis 7 + Sentinel을 Docker Compose/systemd로 배포 완료: 컨테이너명 `istock-redis*`, 비밀번호 `istockdev`, Sentinel master `istockdev-cluster`, AOF/`volatile-lru`/keyspace notification 적용, `6379`/`26379` 리스너와 Redis 읽기·쓰기 및 `devwas`/`devapp` AUTH/PING 검증 완료
- 2026-06-09 기준 운영 FO 최종 실행 파일을 기준으로 DEV `fo-api`, `fo-mobile`, `fo-pcweb`의 `application.yml`, 서비스 env, `logback-spring.xml`, `global-bundle.pem`, `server.sh`를 DEV 프로필/주소로 변환해 `devwas`에 배포 완료: 주석 포함 운영 구조 유지, 기존 DEV 비밀값 보존, `.bak.20260609-130629` 백업, 원격 SHA256/ENV source/`bash -n`/권한 검증 완료
- 2026-06-09 기준 PRD 실제 실행 파일을 기준으로 DEV Overpass 공통 6개 앱(`batch-api1`, `batch-api2`, `bo-pcweb`, `po-pcweb`, `oa`, `po-mobile`)의 `application.yml` 구조와 주석, `server.sh`를 DEV 값/프로필로 변환해 `devwas`/`devapp`에 배포 완료: build 및 대용량 서버 자산/JAR 제외, 기존 DEV env 실값 보존, 배포 전 파일 백업, 원격 SHA256 및 `bash -n` 검증 완료
- 2026-06-09 기준 PRD bastion(`15.164.12.24`)에 `kibana-forward.service`를 등록해 공인 `0.0.0.0:5601` 트래픽을 `search1(10.10.50.106):5601`로 전달하고, systemd 자동 시작 및 공인 주소 HTTP 302 응답 검증 완료
- 2026-06-08 기준 iSTOCK-MALL PRD Valkey7 ElastiCache parameter group `prd-redis-elasticache-pg-cluster-on` 생성 완료: 기존 `prd-redis-elasticache-pg`의 사용자 설정(`cluster-allow-reads-when-down`, `latency-tracking`, `maxmemory-policy`, `notify-keyspace-events`)을 반영하고 `cluster-enabled: yes` 적용 검증
- 2026-06-05 기준 stock-company PRD Redis ElastiCache primary endpoint를 `prd-redis-elasticache.ayvhif.ng.0001.apn2.cache.amazonaws.com:6379`로 반영하고, Spring Boot 4/Spring Session Redis가 관리형 Redis에서 `CONFIG GET notify-keyspace-events`를 실행해 기동 실패하지 않도록 prod `NEW_yamls`에 `spring.session.data.redis.configure-action: none` 설정 추가
- 2026-06-05 기준 stock-company STG `fo-pcweb`/`fo-mobile` 배포 자산에 stage 전용 `application.yml`과 서비스별 env sample을 추가하고, `stage/deploy.yml`이 `application.yml`/`logback-spring.xml`/`<service>.env`를 함께 배포하도록 반영
- 2026-06-04 기준 stock-company PRD `bo-app1/bo-app2`용 nginx 설정 자산 추가: 정적 파일은 `/app/overpass/www-data/bo-webroot`에서 서빙하고 나머지는 BO PC web ALB로 프록시하는 `bo-app.conf`, 공통 proxy header include, bo-app 서버별 `/app/overpass/nginx` staging 배포 경로를 구성하고 dry-run 및 임시 `nginx -t` 문법 검증 완료
- 2026-06-04 기준 PRD payments API JAR 내부 `application-datasource-overpass.yml`의 Redis Sentinel 설정이 외부 `application.yml`과 병합되는 문제를 조치: `server.sh` 템플릿에 앱별 `SPRING_CONFIG_LOCATION` override를 추가하고 payments는 `classpath:/application-commons.yml,file:/app/overpass/overpass-payments-api/config/application.yml` 단독 로딩으로 전환, app1/app2 원격 `server.sh` 보정 및 `bash -n` 검증 완료
- 2026-06-04 기준 PRD payments API의 Spring Boot 4 Redis Sentinel 오인식 오류를 조치: prod `NEW_yamls` Redis 설정을 `spring.data.redis.url: ${SPRING_DATA_REDIS_URL}` 방식으로 전환하고 env sample에 `SPRING_DATA_REDIS_URL=redis://...:6379`를 추가, app1/app2 payments 및 bastion 운영 자산 반영 검증 완료
- 2026-06-04 기준 PRD payments API의 `${DATASOURCE_OVERPASS_URL}` 미치환 오류를 조치: shell source 대상 env sample의 URL 값을 quote 처리하고 `overpass-payments-api` DB SSL 인증서 경로를 `/app/overpass/overpass-payments-api/config/global-bundle.pem`로 보정, app1/app2 원격 env 및 bastion env 자산 반영 후 source 검증 완료
- 2026-06-04 기준 stock-company PRD Overpass `server.values.yml`의 `ActiveProfile`을 `NEW_yamls`의 `spring.config.activate.on-profile: prd`와 일치하도록 `prd`로 보정하고, bastion values 및 `app1/app2/bo1/bo2` 원격 `server.sh`의 `ACTIVE_PROFILE` 기본값 반영을 검증
- 2026-06-04 기준 stock-company PRD `app1/app2/bo1/bo2`의 Overpass `application.yml` 배포 소스를 `NEW_yamls` 기준으로 전환하고, bastion 및 대상 서버의 원격 checksum 일치를 검증: `batch-api1`, `batch-api2`, `overpass-payments-api`, `bo-pcweb`, `po-pcweb`, `oa`, `po-mobile` 반영 완료
- 2026-06-04 기준 PRD/DEV bastion 경유 read-only audit 완료 및 `docs/stock-company-bastion-audit-2026-06-04.md` 작성: PRD/DEV bastion deploy 파일이 로컬 최신 정책보다 오래되어 Kafka 항목과 구 `conf/` 경로가 남은 것을 확인하고, 실제 미반영 파일/부트스트랩(`app1/app2/bo1/bo2`, `devwas/devapp/devbuild`, search checksum diff 등)과 재배포 제외 대상(Kafka/ScyllaDB/Redis Sentinel)을 분리 정리
- 2026-06-04 기준 stock-company DEV/PRD 안전 배포 실행 완료: 최신 bastion 자산 동기화, 제외 항목(Kafka/Kafka Connect/ScyllaDB/Redis Sentinel) 검증, DEV `devwas/devapp/devbuild` 및 PRD `app1/app2/bo1/bo2` subset 배포, 전체 bastion alias 복구, read-only checksum/bootstrap audit 통과
- 2026-06-02 기준 `deploy vm --bastion-only` 옵션을 추가하고 stock-company prod `deploy.yml`의 prd bastion 접속 정보를 `15.164.12.24`/`ops/stock_company/keys/prd-istockmall.pem` 기준으로 보정한 뒤, bastion에 prod 서버 10개 alias(`cache1/cache2/app1/app2/bo1/bo2/bo-app1/bo-app2/search1/search2`) 등록 및 원격 managed block 검증 완료
- 2026-06-02 기준 stock-company dev/prod 서비스별 `NEW_yamls`와 `env/*.env.sample`의 Redis/MongoDB/overpass datasource env 파라미터명을 공통 YAML 기준(`SPRING_DATA_REDIS_*`, `SPRING_MONGODB_CACHE_NODES`, `DATASOURCE_OVERPASS_*`)으로 표준화
- 2026-06-02 기준 stock-company prod 서비스별 env sample과 `NEW_yamls` Redis 설정을 iSTOCK-MALL 리소스 기준으로 갱신: Elasticache primary endpoint, prd PostgreSQL/MySQL RDS endpoint, prd payment/api ALB DNS, prod cache/search IP, prod target group ARN 반영 및 Redis Sentinel 설정 제거
- 2026-05-29 기준 `infra/istock-mall/target_groups.tf` 추가 및 apply 완료: prd Elasticsearch(`9200`)와 Kibana(`5601`) 접근용 target group(`prd-tg-elasticsearch`, `prd-tg-kibana`), `search1/search2` attachment, prd search ALB -> prd search SG ingress rule을 Terraform으로 생성하고 `Service=NewProject` 태그 기준으로 정리
- 2026-05-29 기준 prd bastion(`15.164.12.24`, private `10.10.0.187`) 경유로 stock-company STG `stg-fo-api`/`stg-fo-mobile`/`stg-fo-pcweb` 배포 완료: STG target SG 3개에 prd bastion SG source `tcp/22` ingress를 추가하고, bootstrap/JDK 25/swap/app 파일 배포 및 재실행 checksum skip 검증 완료
- 2026-05-29 기준 전체 deploy 설정과 템플릿 values 샘플에서 JAR 원격 경로가 `bin/*.jar`로 남지 않도록 `lib/*.jar` 기준으로 재확인 및 정리
- 2026-05-29 기준 server.sh 템플릿 렌더링 테스트와 배포 예시의 JAR 경로 기준을 `bin/`에서 `lib/`로 정리해 생성 스크립트의 `JAR_FILE`도 `lib/*.jar`를 가리키도록 검증
- 2026-05-29 기준 stock-company dev/prod/stage deploy 설정의 Overpass jar 원격 경로를 `bin/`에서 `lib/`로 변경하고, 앱 기본 디렉터리 자동 생성에서 불필요한 `config` 디렉터리를 제거
- 2026-05-29 기준 devwas의 `/app/overpass/*/config` 중복 디렉터리를 삭제하고 앱별 `conf` 디렉터리만 남은 것을 검증
- 2026-05-28 기준 `/app` 하위 디렉토리 기본 소유권 정책을 코드 하드코딩 대신 `directory_defaults.rules` 설정으로 분리하고, dev/stage/prod/example 및 dev/prod bastion deploy 파일과 deploy 바이너리를 동기화한 뒤 dev/prod bastion dry-run 검증 완료
- 2026-05-28 기준 stock-company dev build 서버 제외 실배포 완료: `devwas`, `devbo-app`, `devapp`, `hamonica`, `hamonica-adm`, `postgresql`에 bootstrap/package/file 배포를 완료하고 nginx active/enabled, Docker Compose v2.29.7, PostgreSQL 17 패키지 및 주요 archive 파일 존재를 검증
- 2026-05-28 기준 stock-company dev deploy에 `devbo-app`(`dev-bo-nginx-ec2-01-a`, `10.20.10.149`) nginx 전용 서버 항목 추가: prd `bo-app`과 동일하게 JDK 설치 제외, `nginx` 패키지 설치 및 서비스 enable/start 선언
- 2026-05-27 기준 stock-company prod 배포 및 Elasticsearch 복구 중 발견한 문제를 `docs/stock-company-prod-deployment-issues.md`에 별도 트러블슈팅 문서로 정리
- 2026-05-27 기준 stock-company prod bo-app1/bo-app2에 nginx bootstrap 배포를 완료하고, nginx active/enabled 및 localhost HTTP 200 응답을 검증
- 2026-05-27 기준 기존 서버 원본 `infra/istock-mall/elasticsearch.tar`를 기반으로 `ops/stock_company/common|prod/extra_files/elasticsearch.tar.gz` 정리본 재생성 및 교체 완료: Elasticsearch/Kibana `data`/`logs`/`tmp`/`pid`/`optimize` 하위 런타임 파일과 macOS 메타데이터를 제거하고 JDK/실행 스크립트 보존, prod search dry-run 통과
- 2026-05-27 기준 `infra/istock-mall/elasticsearch.tar` 분석 및 prod search1/search2 수동 반영 완료: archive 내 `es-master`/`es-data` JDK와 Kibana 실행 자산 확인, bastion staging checksum 일치 검증, 각 search 서버 `/tmp/elasticsearch.tar` checksum 일치 후 `/app`에 sudo extract 및 `elasticsearch:elasticsearch` 소유권 보정 완료
- 2026-05-27 기준 stock-company prod search1/search2 배포 재실행 완료: `/app/elasticsearch/elasticsearch.tar.gz` checksum 일치로 재전송 없이 skip, `/app/elasticsearch` 추출 경로와 4G swap 활성화 검증, bastion alias sync 완료
- 2026-05-27 기준 stock-company prod app1/app2에 overpass 앱 배포 항목을 제외한 bootstrap/extra_files 배포를 완료하고, app2 `/home/ec2-user/software` 권한 충돌을 정리해 `software.tar` 재압축 해제까지 검증
- 상위 Git 저장소에서 `TEST/`를 로컬 전용 자산으로 분리: `.gitignore`로 제외하고, 공개용 샘플은 `samples/server-script.sample.sh`로 승격
- `TEST/`에 독립 Git 저장소용 기본 파일(`.gitignore`, `README.md`)을 추가해 상위 저장소와 테스트 이력을 분리할 준비 완료
- M1 핵심 골격 구현: `deploy vm --config ...` 진입점, 설정 로드, 순차 VM 배포 러너 연결
- `deploy docker` placeholder 경로 추가
- M1 배포 단계 구현: SSH 연결, 디렉토리 생성, jar 전송, 설정 파일 배포, 스크립트 배포, 실행 권한 부여
- jar/config/script 전송 시 SHA256 비교 기반 skip 로직 구현
- 내장 `server.sh` 템플릿 렌더링 및 임시 파일 기반 배포 구현
- 설정 기본값/필수값/파일 존재/포트 범위 검증 구현
- `extra_opts`를 안전한 배열 인자 방식으로 변경하고 문자열/배열 YAML 입력 모두 지원
- 설정 파싱 및 SSH quoting 관련 기본 테스트 추가
- `config/loader_test.go` 추가: 환경변수 치환, 기본값, `~/` 확장 검증
- `template/renderer_test.go` 추가: 내장 템플릿 변수 치환, 사용자 템플릿 우선 적용 검증
- `deploy.example.yml` 추가: 민감정보 없는 예시 설정 제공
- 로컬 캐시 기준 `go test ./...`, `go vet ./...` 통과
- SSH 호스트 키 검증을 `known_hosts` 기반으로 강화
- SSH 호스트 키 검증 모드화: `strict`, `accept-new`, `insecure`
- 원격 명령 경로 quoting 공통화로 셸 주입/공백 경로 오동작 방지
- 설정 선검증 강화: 포트 범위, 로컬 파일 존재, 템플릿 존재, `known_hosts` 확인
- `bastion` 설정 추가: VM 이름 기반 SSH alias(`~/.ssh/config`) 자동 생성
- bastion shell alias 동기화 추가: 서버 이름별 `ssh -F ... <alias>`를 `~/.bashrc` managed block으로 등록
- bastion SSH alias에 `IdentityFile`, `IdentitiesOnly`, `StrictHostKeyChecking`, `UserKnownHostsFile`를 함께 렌더링해 alias SSH가 실제 target 접속까지 가능하도록 보강
- bastion과 각 EC2의 `known_hosts` 동기화 로직 추가
- bastion alias/known_hosts 생성 로직 테스트 추가
- `deploy vm --dry-run --config ...` 추가: SSH/SFTP 및 원격 파일 변경 없이 서버별 예정 작업 로그 출력
- dry-run 경계 테스트 추가: CLI 옵션 라우팅, VM runner, SCP 전송, bastion host-key 등록
- M1 통합 검증 중 재현된 세 결함 수정: privileged 디렉토리 생성, 원격 파일 미존재 시 SHA 비교 우회, bastion `~/` 경로 렌더링 보정
- 관련 회귀 테스트 추가: `internal/vm/dirs_test.go`, `internal/scp/transfer_test.go`, `internal/vm/bastion_test.go`
- 2026-04-06 기준 Podman 통합 검증에서 최초 실행 happy path가 bastion 직전까지 통과하고, 원격 파일 미존재 상태에서도 jar/config/script 자동 전송이 동작함을 확인
- TEST Amazon Linux 이미지에 `openssh-clients`를 추가해 `ssh-keyscan`을 포함시킴
- 2026-04-06 기준 Podman 통합 검증에서 최초 실행 happy path, 원격 파일 미존재 시 자동 전송, bastion alias/known_hosts 등록까지 모두 완주
- `servers[].ssh_port` 설정 추가: 동일 호스트에 여러 테스트 VM을 포트로 구분해 연결 가능
- 서버별 SSH 포트를 VM 연결 및 bastion alias/known_hosts 동기화 로직에 반영
- TEST 환경을 two-container/multi-port 구조로 전환: auth `127.0.0.1:2222`, billing `127.0.0.1:2223`
- 2026-04-06 기준 multi-port 배포 검증에서 auth는 2222, billing은 2223으로 분리 접속되는 것을 확인
- TEST Amazon Linux entrypoint에서 `ec2-user` 계정을 테스트 전용으로 unlock 하도록 보완해 공개키 인증 실패를 제거
- 로컬 TEST 설정을 사용자 `~/.ssh/config`, `~/.ssh/known_hosts`와 분리: `/tmp/overpass-test-ssh_config`, `/tmp/overpass-test-known_hosts` 사용
- Podman 로컬 테스트용 bastion 대상 주소를 `host.containers.internal` + published port 방식으로 정리
- bastion sync가 `/tmp/...` 경로를 사용할 때 부모 디렉터리 권한 변경으로 실패하지 않도록 조건부 `chmod` 처리 추가
- `internal/vm/bastion_test.go`에 `/tmp` 기반 bastion sync 회귀 테스트 추가
- 2026-04-06 기준 로컬 Podman two-container 테스트에서 `go run ../cmd/deploy vm --config ./deploy.yml`가 두 컨테이너 배포와 bastion sync까지 완주
- AWS EC2/Bastion 스테이징 검증용 Terraform 구조 추가: `infra/aws-test`에 기존 `default-vpc` 네트워크 import, public bastion 1대, AZ별 private target EC2, 보안그룹, 출력값 문서화
- AWS EC2 smoke test 자산 추가: `TEST/aws-smoke`에 mock jar/config와 bastion 실행용 `deploy.aws-test.yml` 템플릿 추가
- AWS smoke test runbook 추가: `docs/aws-test-smoke.md`에 Terraform apply, bastion 스테이징, dry-run/실배포, 원격 파일 검증, cleanup 절차 문서화
- 통합 테스트 실행 가이드 추가: `docs/test-deployment-guide.md`에 로컬 Docker 테스트와 AWS smoke test 절차를 한 문서로 정리
- `infra/aws-test` 보강: bastion self-SSH/target `ssh-keyscan` 허용 규칙과 `bastion_private_ip` output 추가
- 2026-04-09 기준 실제 AWS smoke test 완주: `infra/aws-test` apply, bastion 스테이징, `deploy vm --dry-run`, 실배포, 원격 파일/SHA 검증, 재실행 skip 동작까지 확인
- 운영 샘플 `TEST/script-sample.sh`를 반영해 기본 `server.sh` 템플릿을 고도화: `restart/status/log`, PID 기반 상태 확인, startup 대기, 로그 tail 흐름 추가
- `script.values_file` 설정 추가: 실행 스크립트 템플릿 데이터도 외부 YAML 파일에서 읽고, 기본 앱 값 위에 템플릿별 override를 적용하도록 리팩토링
- `script.template`에 `embedded:server.sh.tmpl` 같은 선언형 참조를 지원하고, TEST/AWS smoke 샘플 설정을 새 구조로 갱신
- `.jar` 및 기타 업로드 파일에 대해 SFTP 전송률을 progress bar + 퍼센트로 표시하도록 개선하고, non-TTY 로그 환경에서도 10% 단위 진행 상태가 보이도록 보완
- 2026-04-10 기준 실제 AWS smoke test 재검증 완료: Linux/amd64 바이너리로 bastion에서 dry-run, 실배포, 재실행 skip, 원격 SHA256 일치와 progress 출력까지 확인하고 `docs/aws-test-smoke.md`를 최신 경로/빌드 절차에 맞게 보정
- 선언형 bootstrap 단계 추가: 전역 `bootstrap` + 서버별 override merge, dnf 기반 패키지 설치, Corretto JDK 설치, opt-in `dnf update` dry-run/실행 흐름과 검증 테스트 구현
- 릴리스 배포 구조 추가: `.goreleaser.yml`, `deploy version`, `deploy update`, `internal/buildinfo`, `internal/update` 도입
- GitHub Releases 기반 self-update 구현: latest/특정 태그 조회, 플랫폼별 asset 선택, checksum 검증, 현재 실행 파일 원자적 교체
- 릴리스/설치 문서 추가: `docs/release-distribution.md`, `docs/release-install.md`
- 릴리스 및 self-update 회귀 테스트 추가: CLI 라우팅, GitHub release 파싱, checksum 검증, 바이너리 교체 실패 케이스 검증
- `servers[].apps` 설정 지원 추가: 한 서버에서 여러 앱을 순차 배포할 수 있도록 스키마/validator/VM runner 확장
- 기존 `servers[].app` 설정과의 하위 호환 유지, `app`과 `apps` 동시 사용 시 검증 오류 처리 추가
- 다중 앱 설정 로드/검증/배포 순서 회귀 테스트 추가
- `TEST/stock_company/deploy.stock-company.dev.draft.yml` 초안 추가: `devwas` multi-app 배포 기준 샘플 작성
- `TEST/stock_company/deploy.stock-company.dev.draft.yml`에 `devapm1`, `devapm2` multi-app 초안 추가
- `script.mode` 확장 추가: `template` 기본 동작 유지 + `local-file` 모드에서 로컬 `server.sh`를 그대로 원격으로 복사 지원
- `script.local_path` 검증 및 스크립트 소스 선택 테스트 추가
- bootstrap-only 서버 지원 추가: `servers[].directories`와 서버 단위 bootstrap만으로도 유효한 설정 허용
- `TEST/stock_company/deploy.stock-company.dev.draft.yml`에 `devapp1`(디렉토리 생성), `devapp2`(Docker/Compose), `devdb`(PostgreSQL) 초안 추가
- `devdb` PostgreSQL 17 bootstrap 패키지 목록을 Amazon Linux 2023 실검증 값으로 구체화
- `extra_files[]` 설정 추가: `setEnv.sh` 같은 추가 스크립트/일반 파일을 앱별로 전송하고 선택적으로 chmod 적용 가능
- `extra_files[]` validator, dry-run, chmod helper, 배포 순서 테스트 및 예시 설정 추가
- 설정 키 정리: `config_files`와 `script`도 `local_path` / `remote_path` 기준으로 통일하고 기존 `local` / `remote` / `remote_dir`는 하위 호환 alias로 유지
- `servers[].extra_files[]` 설정 추가: app 밖에서도 tgz/보조 파일을 서버 단위로 전송 가능하도록 validator/VM runner/샘플 초안 확장
- `server.sh.tmpl`의 Hamonica 블록을 generic values merge 기반으로 정리: 코드 전용 필드 없이 `values_file`의 같은 이름 키(`HAMONICA_JAVA_AGENT`, `HAMONICA_CONFIG_FILE`)만 템플릿에 반영되도록 수정
- `TEST/stock_company/PLAN.md`에 실제 AWS test EC2 2대를 순환 사용하는 stock-company 전용 rotating smoke test 계획 추가: wave 분할, mock 자산 구조, IP 매핑, 검증 체크리스트 정리
- `TEST/stock_company/aws-rotating-smoke` 추가: wave별 `deploy.yml` 초안과 stock-company 경로 규칙을 따르는 mock jar/config/script/values/agent 자산 스캐폴딩 작성
- 2026-04-10 기준 AWS bastion에서 stock-company rotating smoke wave 1, wave 2 실행 완료: dry-run, 실배포, 재실행 skip, 원격 SHA256/권한, bastion alias/known_hosts 검증 및 보고서 작성
- 2026-04-10 기준 stock-company rotating smoke wave 3 실행 완료, wave 4 dry-run 완료: 교차 배치 재현성과 bootstrap-only 설정 유효성 검증 및 보고서 작성
- wave 1/wave 3의 간헐적 재전송 원인 분석 완료: 현재 원격 파일 hash는 모두 local과 일치했고, `internal/scp/transfer.go`의 remote checksum 파서를 stdout/stderr noise tolerant 하게 보강하고 회귀 테스트를 추가
- 선택 배포용 tag 필터 추가: `servers[].tags`, `apps[].tags`, CLI `--server-tag`, `--app-tag` 지원과 태그 기반 서버/앱 subset 배포 로직 및 회귀 테스트 구현
- `docs/M1-PLAN.md`, `PLAN.md`를 현재 코드 기준으로 정리해 obsolete 체크리스트를 실제 구현 형태로 교체하고 M1 상태를 완료로 승격
- 루트 `README.md` 추가: 운영/사용자 온보딩 중심으로 현재 지원 범위, 빠른 시작, 핵심 설정 구조, 주요 명령, 관련 문서 링크를 정리
- `ops/` 운영 자산 스캐폴딩 추가: 서비스 기준(`stock_company`, `sample_api`) 1차 분리 후 각 서비스 아래 `common/`, 환경별(`dev`, `stage`, `prod`) 디렉터리 구조를 생성하고, 상위 저장소와 분리된 독립 Git 저장소로 초기화
- stock-company dev 배포 자산 추가: `ops/stock_company/dev/deploy.yml`, Redis compose 파일, 공통 대용량 extra_files hard link 기준 정리 및 전체/태그별 dry-run 검증 완료
- stock-company dev ScyllaDB 배포는 보류 처리: `devapm2` 배포 대상에서 Scylla compose/bootstrap/directory를 제외하고 MongoDB/Hazelcast만 유지
- stock-company prod 배포 자산 추가: `cache1/cache2/app1/app2/app3/search1/search2` 서버 배치를 `ops/stock_company/prod/deploy.yml`로 이전하고 dev deploy는 `devapp1/devapp2/devapm1/devapm2`만 유지
- stock-company prod Redis Sentinel 설정 추가: `redis-sentinel.conf`를 prod app 서버 Redis 자산에 포함하고 compose 마운트 경로를 파일 단위로 보정
- 서버/app `extra_files[].extract` 옵션 추가: tar/tar.gz/tgz 자산 전송 후 원격 디렉터리에 압축 해제할 수 있게 하고 stock-company dev/prod 소프트웨어 tar에 적용
- istock dev build 서버용 Docker Compose 자산 추가: `ops/stock_company/dev/build/docker-compose.yml`에 Jenkins, GitLab EE, Nexus3 구성을 분리 배치
- istock dev build Jenkins 이미지를 `jenkins/jenkins:2.555.2-lts` 기반 전용 Dockerfile로 분리하고 컨테이너 내부 AWS CLI v2 설치를 공식 zip installer 방식으로 반영
- 기존 Jenkins 플러그인 목록을 `ops/stock_company/dev/build/jenkins/plugins.txt`에 고정하고 Dockerfile에서 `jenkins-plugin-cli`로 사전 설치하도록 반영
- `TEST/stock_company/aws-rotating-smoke/TEST-WAVE-1.md`, `TEST-WAVE-3.md` 복구: 남아 있던 wave 2/4 문서와 진행 기록 기준으로 실행 결과 문서 재생성
- stock-company dev/prod overpass 앱 배치 반영: dev `devwas/devapp`, prod `app1/app2/app3/bo1/bo2` 서버에 앱별 jar/config/server.sh/Hamonica agent 설정을 추가하고 `overpass-payments-api` 이름으로 통일
- stock-company stage overpass 앱 배치 추가: `stagewas` 서버에 `fo-pcweb`, `fo-mobile`, `fo-api` 배포 설정과 stage 전용 values 파일 반영
- AWS test Terraform을 stock-company dev 배치 기준으로 확장: target EC2를 `devwas/devapp/devapp1/devapp2/devapm1/devapm2` 6대 역할로 재구성하고 dev deploy placeholder 출력과 root volume 변수, 기존 2대 target state 이동 블록 추가
- AWS dev-test 인프라 생성 및 bastion 스테이징 완료: bastion 1대 + target 6대 EC2를 생성하고 stock-company dev deploy 바이너리/설정/대용량 자산을 bastion에 업로드한 뒤 overpass 태그 dry-run과 전체 dry-run 검증 완료
- extra_files 압축 해제 재발 방지 보강: `extract.remote_dir`를 압축 해제 직전에 sudo로 생성/소유권 보정하도록 deployer를 수정하고 stock-company dev software 서버의 `/home/ec2-user/software` 디렉터리 사전 생성을 반영
- AWS dev-test 재배포 준비: test EC2의 기존 배포 산출물을 원복하고, 압축 해제 권한 보정을 비재귀 chown으로 축소하며 tag 배포 후 bastion alias가 전체 서버 목록을 유지하도록 수정
- AWS dev-test 리소스 정리 완료: imported default VPC/subnet/route table은 유지하고, targeted Terraform destroy로 bastion 1대 + target 6대 EC2, key pair, bastion/target security group, SG rule 삭제 완료
- overpass 앱 디렉토리 표준을 `bin/config/logs`로 축소: jar와 `server.sh`는 `bin`, `application.yml`/`logback.xml`은 `config`, 실행 로그는 `logs` 기준으로 dev/stage/prod deploy 설정 반영
- bootstrap 확장: `deploy.yml`에서 OS update, 필수 패키지, Corretto 25, Asia/Seoul timezone, 4G swapfile 생성을 선언적으로 처리하도록 `timezone`/`swap` 설정과 idempotent 실행 로직 추가
- istock mall 운영 자산 반영: `ops/stock_company/dev|stage|prod/deploy.yml` 전역 bootstrap에 필수 패키지, Corretto 25, OS update, Asia/Seoul timezone, 4G swapfile 설정 적용 및 세 환경 dry-run 통과
- 배포 예상 시간 출력 추가: dry-run/실배포 시작 시 파일 크기, bootstrap, 압축 해제, 디렉터리/앱 단계 수를 기준으로 전체/서버별 예상 배포 시간을 로그에 표시
- 성능 개선: `--parallel` 서버 병렬 배포 옵션 추가, 서버별 원격 checksum 배치 조회, SFTP 세션 재사용으로 실제 배포 시 SSH 왕복/연결 생성 비용 축소
- AWS dev-test 인프라 재생성 및 stock-company dev 파일 전송 완료: bastion 1대 + target 6대 EC2를 다시 생성하고, bastion에 배포 바이너리/운영 자산/테스트 자산을 업로드한 뒤 private target 전체에 files-only 배포와 재실행 checksum skip 검증 완료
- 2026-05-18 기준 AWS dev-test 인프라 상태 확인 및 재생성 완료: `infra/aws-test` Terraform apply로 bastion 1대, private target 6대, key pair, bastion/target security group 생성 완료
- 2026-05-18 기준 AWS dev-test 인프라에서 `devapp` 역할을 제거하고 `devapp1`로 통합 완료: private target은 `devwas/devapp1/devapp2/devapm1/devapm2` 5대 구조로 반영
- 2026-05-18 기준 stock-company bootstrap 설정 검증 완료: Amazon Linux 2023에서 `nc` 명령 제공 패키지를 `nmap-ncat`으로 보정하고, dry-run으로 `nmap-ncat/net-tools/unzip/wget/vim-enhanced`, Corretto 25, `/swapfile` 4G 계획 확인
- 2026-05-18 기준 bastion 내부 deploy 바이너리로 AWS dev-test bootstrap 실검증 완료: 5대 target 모두 `nmap-ncat/net-tools/unzip/wget/vim-enhanced`, Corretto 25 설치와 `/swapfile` 4G 활성화 및 fstab 등록 확인
- bootstrap 실검증 중 발견한 원격 패키지 매니저 감지/스왑 idempotency 결함 보강: package manager 감지를 절대 경로 기반으로 단순화하고, 이미 활성화된 swapfile 재호출을 성공 처리하도록 회귀 테스트 추가
- 2026-05-18 기준 AWS dev-test 리소스 정리 완료: imported default VPC/subnet/route table은 유지하고 bastion 1대, target 5대, key pair, bastion/target security group, SG rule 삭제 완료
- 2026-05-21 기준 istock mall EC2 배치 설정 재정렬: dev `devapp` 단일화, `devbuild`/`hamonica`/`hamonica-adm`/`postgresql` 추가, stage FO 3대 분리, prod app 2대 구성 반영
- 서버별 `bootstrap.jdk.enabled: false` 옵션 추가: istock mall `postgresql`, `search1`, `search2`에서 Corretto 25 설치를 제외하고 cache 서버 JDK 설치는 유지
- ISTOCK-MALL 실계정 리소스 inventory Terraform 추가: `infra/istock-mall`에서 계정 guardrail, 기존 VPC/Subnet/EC2/SG/ALB/RDS data source, dev/stage/prod deploy placeholder output 정리
- ISTOCK-MALL inventory output 기준으로 `ops/stock_company/dev|stage|prod/deploy.yml` host/bastion placeholder를 실제 private IP로 동기화하고 dev/stage/prod 대표 태그 dry-run 통과
- `bootstrap.services[]` 선언형 서비스 관리 추가: 패키지 설치 이후 `systemctl enable/disable/start/stop/restart`를 idempotent하게 실행하고 nginx 예시와 회귀 테스트 반영
- stock-company prod 운영 deploy에 AWS 리소스명과 맞춘 `bo-app1`, `bo-app2` nginx 전용 서버 항목 추가: JDK 설치 제외, `nginx` 패키지 설치 및 서비스 enable/start 선언
- `bootstrap.docker_compose` 수동 설치 옵션 추가: 패키지 매니저 대신 지정 URL에서 Docker Compose v2 바이너리를 받아 CLI plugin 경로에 설치하고 버전/idempotency 검증을 수행하도록 구현
- istock mall dev Docker Compose 서버 설정 보정: `devbuild`/`devapp`에서 `docker-compose-plugin` 패키지 의존을 제거하고 Docker 서비스 enable/start 및 Compose v2 수동 설치를 선언, `devbuild`는 host JDK 설치 제외
- `infra/istock-mall` inventory에서 기존 `istockmall-VPC-prd`(`10.0.0.0/16`) 관련 VPC/NAT/Route Table/ALB/NLB/RDS 항목 제외
- `bastion.target_key_path` 설정 추가: 배스천 내부에 `.pem` 키가 있으면 shell alias를 `ssh -i <key> -p <port> <user>@<host>` 형태로 생성하고 iSTOCK dev/stage/prod 배포 설정에 반영
- 디렉토리 생성 단계 보수화: 기존 디렉토리는 소유권 변경 없이 그대로 두고, 없는 디렉토리만 생성 후 생성 지점만 소유권 보정
- swap bootstrap 보강: `/swapfile`이 없거나 swap 서명이 없을 때도 단일 sudo 단계에서 생성/권한 보정/mkswap/swapon을 보장하도록 수정
- stock-company dev/prod `OLD_yamls` 기준 application YAML env placeholder 분리본 추가: `NEW_yamls`에 Redis host/port/password/sentinel node, MongoDB nodes, datasource 접속 정보, 내부 API URL, Hazelcast/AWS 접속 값을 `${...}` 형태로 치환
- `NEW_yamls/env-parameters.yml` 갱신: dev/prod application placeholder 204개를 서비스별 YAML 구조로 정리하고, 공유 값은 `PROD_REDIS_*`, `PROD_REDIS_SENTINEL_NODES`, `DEV_BATCH_API_*`처럼 공통 파라미터명으로 통합해 unique 변수 79개로 축소
- `NEW_yamls/parameter-summary.yml` 추가: 공통 파라미터와 개별 파라미터를 분리해 사용 파일, NEW/OLD 위치, OLD 원본 값을 함께 정리
- `NEW_yamls/.env.sample` 추가: dev/prod 환경별 shared/individual 파라미터를 WAS `EnvironmentFile`에 넣기 쉬운 `KEY=value` 샘플로 정리
- stock-company dev/prod `env/` 디렉토리 추가: 각 환경별 서비스 단위 `*.env.sample`을 생성하고, 서비스 YAML에서 실제 참조하는 `${...}` 변수만 포함하도록 분리
- stock-company dev/prod 앱 배포에 `config/<service>.env` 전송을 추가하고, 공통 `server.sh` 템플릿이 `${BASE_DIR}/config/${APP_NAME}.env`를 로드한 뒤 Spring Boot를 실행하도록 반영
- stock-company dev/prod 배포 자산에서 미사용 메시지 브로커 compose/archive 참조와 application YAML의 관련 사용 플래그를 제거하고, 남은 관련 문자열이 없는지 dry-run 및 검색으로 검증
- iSTOCK-MALL AWS 인프라 Terraform 디렉토리(`infra/istock-mall/`)는 main 브랜치 추적 대상에서 제외하고, 별도 운영 브랜치/저장소에서 관리하는 방향으로 정리
- 2026-05-27 기준 prod bastion 경유로 `cache1/cache2/search1/search2` 파일 배포 완료: MongoDB/Hazelcast/Elasticsearch tar checksum 일치와 압축 해제 경로 확인
- Studio JSON 편집기 높이 안정화: 토폴로지 가져오기 다이얼로그를 viewport 안으로 제한하고, 헤더·하단 버튼을 유지한 채 본문과 고정 높이 JSON 입력창이 각각 내부 스크롤되도록 공통 편집기 컴포넌트 적용
- 2026-09-14 로컬 Studio `TEST` 프로젝트에 stock-company prod 안전 자산 18개 등록 완료: placeholder 기반 config-template 10개, script-values 7개, nginx 일반 파일 1개를 프로젝트 범위 artifact로 저장하고 목록 재조회 검증 완료
- 2026-09-14 iSTOCKMALL Studio 앱 자산 세트 보완 완료: application 환경변수와 1:1로 대응하는 비밀값 없는 env-example 10개, 앱별 logback 템플릿 10개, 누락된 prod values 3개와 비실행 dummy JAR 1개를 등록해 프로젝트 artifact 총 42개를 검증
- 2026-09-14 Studio `TEST` dev 토폴로지의 Java 앱 10개 artifact 매핑 완료: 공통 dummy JAR와 앱별 application/logback/env/values를 연결해 revision 5로 저장하고 package 검증에서 Java 관련 오류 0건 확인
- 2026-09-14 Redis·Elasticsearch mock artifact 기능 검증 완료: 안전한 Redis mock 설정 3개와 644-byte Elasticsearch mock TAR.GZ를 등록·매핑해 revision 6 package validation 오류 0건, 비동기 package 생성 55개 파일 및 29KB ZIP CRC 검증 통과
- 2026-09-14 Studio package file 식별자 범위 수정: `package_files` 기본키를 `(package_id, id)` 복합키로 마이그레이션해 여러 배포 패키지가 동일한 `file_deploy`/`file_0001` 식별자를 안전하게 재사용하도록 하고, 기존 데이터 보존·패키지별 조회 격리·실패 시 트랜잭션 rollback 회귀 테스트 추가
- 2026-09-15 Studio JSON 큰 화면 편집기 추가: 공통 Crosswalk 설정, 노드별 배포 상세, 토폴로지 가져오기 JSON에서 viewport 기반 대형 팝업 편집·JSON 정렬·줄/문자 수·Ctrl/⌘+Enter 적용·팝업 내부 문법 오류 표시 지원

## Next To-Do

- iSTOCKMALL 운영 배포 전 Redis mock 설정 3개를 실제 Secret 주입 방식·systemd unit과 맞는 운영 파일로 교체하고, Elasticsearch mock archive를 Object Storage/분할 정책이 확정된 실제 archive로 교체.
- iSTOCKMALL 운영 배포 전 현재 APP_NAME만 포함한 최소 logback 템플릿을 실제 appender/rolling 정책 파일로 교체하고, dummy JAR를 앱별 실제 빌드 artifact로 교체.
- stock-company prod 제외 자산 후속: 실제 비밀값이 포함된 env/OLD/요약 파일은 placeholder 템플릿으로 정제하고, 256MiB를 초과하는 archive는 용량 정책·분할/외부 Object Storage 방식을 확정한 뒤 등록.
- Studio UI 수동 확인: 작은 viewport와 대용량·단일 행 JSON 붙여넣기에서 가져오기 버튼이 계속 노출되고 편집기 내부 스크롤이 동작하는지 확인.
- Studio Backend 후속: JSON Pointer 단위 package 오류 세분화, worker 관측 지표와 재시도 보존·만료 정책.
- Studio 프론트 후속: 실제 파일 fixture를 사용하는 브라우저 업로드→artifact 선택→설계 저장→package 생성→ZIP 내용 검증 E2E와 설정·일반 파일용 구조화 편집기.
- Studio 계약 후속: 조직 단위 권한 모델, 외부 Secret 참조 방식, generator별 schema evolution 정책 확정.
- 생성기 후속: 환경별 overlay와 Helm·GitLab CI·Argo CD 생성기를 capability 기반으로 추가하고 Terraform은 이후 단계로 분리. Docker Compose 생성기는 구현·로컬 E2E 검증 완료이며 외부 Docker/Registry 인수를 유지.
- 실행 계층 후속: Redis/Elasticsearch 설치·실행 생성기, 포트 매핑 및 서비스별 복수 포트 모델, 실제 브라우저 조작 QA 추가.
- 리뷰 후속: POSIX rename 미지원 서버의 안전한 교체 전략과 bootstrap ensure 명령의 실제 변경 여부 반환 규약을 확장 검토.
- M3 Docker 후속: 구현된 Registry 인증·digest pull skip·컨테이너 교체/rollback을 지정 Docker SSH fixture와 Harbor에서 최종 smoke 검증.
- 복잡성 정리 후속: 완료된 Studio 훅·delivery 책임 분리와 Chrome E2E 이후 모바일·대용량 파일 시각 QA. M3/M7 외부 Docker/Registry smoke 및 M4 DEV custom·PRD read-only 인수는 별도 승인된 대상에서 수행.
- `deploy dashboard`의 문서화된 Java/local-file 규약과 구현된 monitor override를 DEV custom 앱·PRD read-only 환경에서 인수 검증.
- DEV OA 전용 ALB 전환 안정화 후 기존 `dev-external-alb` priority `50` OA rule 및 old ALB -> `dev_was:25205` SG rule cleanup 여부 결정
- PRD 운영 private env와 `global-bundle.pem`을 git 추적 밖 경로로 확보한 뒤 `ops/stock_company/prod/deploy.yml`의 `.env.sample` 참조를 실제 private 자산으로 교체하고 PRD `--server-tag search` dry-run을 재시도
- 원격 PRD/DEV running process와 배포 대상 checksum을 read-only로 재확인한 뒤 bastion deploy.yml checksum 동기화 여부를 승인받아 반영
- DEV `istockdev` 원격 JDBC 접속이 필요하면 접속 역할/비밀번호와 허용 CIDR을 확정한 뒤 `listen_addresses`, `pg_hba.conf`, 보안그룹을 별도 승인 하에 구성하고 애플리케이션 연결 테스트 수행
- DEV `istockdev`의 복원된 publication 5개를 실제 logical replication에 사용할 경우 `wal_level=logical` 전환과 publication table 매핑 범위를 확정한 뒤 재시작 영향 검토
- DEV `batch-api1`/`batch-api2`의 24바이트 mock JAR를 실제 DEV 빌드 산출물로 교체한 뒤 `devwas`에서 `25251`/`25252` 기동 및 로그/의존 서비스 smoke test 수행
- `hamonica`/`hamonica-adm`의 `cache-sh.service`를 `hz-start` 직접 실행 방식으로 정상화한 뒤 `devwas`/`devapp`에서 두 Hazelcast 노드의 `5701` 연결을 재검증
- DEV FO 3개 앱은 현재 중지 상태이므로 의존 서비스(PostgreSQL, Elasticsearch, payments API) 접근성을 확인하고 새 iSTOCK Redis 연결을 포함해 `fo-api` -> `fo-mobile`/`fo-pcweb` 순서로 기동 smoke test 수행
- DEV Overpass 6개 앱의 환경별 batch server 번호와 검색/AWS target group endpoint를 확정한 뒤 현재 주석 처리된 설정을 필요한 앱만 활성화하고 순차 기동 smoke test 수행
- PRD `fo-pcweb`/`fo-mobile` 등 Redis 세션 사용 앱에 갱신된 `application.yml`과 `<service>.env`를 배포한 뒤 `CONFIG GET notify-keyspace-events` 오류가 사라졌는지 기동 smoke test 수행
- STG `fo-pcweb`/`fo-mobile`의 `DATASOURCE_OVERPASS_USERNAME/PASSWORD` 실제 값을 확정하고, 필요하면 `global-bundle.pem` 인증서 배포 항목을 추가한 뒤 앱 기동 smoke test 수행
- `prd-bo-external-alb` 실제 DNS를 운영 계정에서 확인해 `ops/stock_company/prod/nginx/sites/bo-app.conf`의 `REPLACE_WITH_PRD_BO_PCWEB_ALB_DNS`를 교체하고, `/etc/nginx/conf.d/bo-app.conf` symlink/copy 및 `nginx -t && systemctl reload nginx` 반영 절차 자동화 여부 결정
- PRD Overpass 앱 재기동 전 `logback-spring.xml` appender 구성과 DB SSL `global-bundle.pem` 경로를 현재 `/app/overpass/*/config` 배포 구조에 맞게 확인한 뒤 payments API smoke test 수행
- 실제 DEV/PRD 서비스별 `config/<service>.env` 값을 확정한 뒤 앱 프로세스 start/restart 순서와 smoke test 범위 결정
- PRD `search1/search2` Elasticsearch archive checksum diff는 의도한 운영 artifact 기준으로 별도 승인 후 반영 여부 결정
- prd bastion에서 새 셸 또는 `source ~/.bashrc` 후 등록된 prod alias로 각 target SSH 접속 smoke test를 필요 범위만 선별해 확인
- prd search ALB listener rule의 host/path 조건을 확정해 Elasticsearch/Kibana target group을 실제 ALB 트래픽 경로에 연결
- `TEST/` 독립 저장소의 원격(origin) 연결 여부와 ignore 규칙을 실제 팀 운영 방식에 맞게 확정
- 실제 저장소 owner/repo에 맞는 `RELEASE_OWNER`, `RELEASE_REPO` CI 주입값 확정
- 실제 GitHub Release 태그 기준으로 `goreleaser release --clean` 한 번 검증
- GitLab Release asset 또는 Package Registry 업로드 CI 단계를 별도로 정리
- `.gitignore`를 M1 체크리스트 기대 항목과 맞추거나 기준 자체를 현실화
- 실제 원격 환경에서 `deploy vm --config deploy.yml`, `deploy vm --dry-run --config deploy.yml` 실행 검증
- TEST 가이드에 Podman 기준 실행 순서와 `/tmp/overpass-test-*` 정리 방법 문서화
- NAT 없는 private subnet에서 bootstrap 패키지 설치가 필요한 서버용으로 오프라인 패키지/내부 미러/NAT 중 운영 방식을 확정
- bastion shell alias가 새 셸 또는 `source ~/.bashrc` 후 실제로 노출되는지 smoke test로 재확인
- AWS test cleanup 문서에 imported network가 state에 남는 구조와 targeted destroy 명령을 명시
- 샘플 운영 스크립트에서 아직 설정 스키마로 일반화되지 않은 항목(예: context path, 별도 healthcheck path, Java agent 전용 옵션)을 템플릿 입력으로 승격할지 검토
- `deploy.example.yml`과 함께 제공할 템플릿 value 파일 샘플을 저장소 표준 위치로 정리할지 결정
- 실제 bastion/VM 환경에서 대용량 `.jar` 전송 시 progress bar 출력이 운영 터미널에서도 자연스럽게 갱신되는지 확인
- bootstrap 후속: 구현된 Ubuntu/Debian apt-get 및 RPM package manager 분기를 신규 지원 OS 조합에서 추가 검증.
- 실제 Amazon Linux 서버에서 `bootstrap.timezone`/`bootstrap.swap` dry-run 및 실배포를 검증하고 `nc` 패키지명이 환경별로 맞는지 확인
- `deploy.example.yml`와 운영 문서에 `servers[].apps` 예시 추가
- `TEST/stock_company/PLAN.md` 기준 `devwas`용 실제 multi-app `deploy.yml` 샘플 작성
- `TEST/stock_company/PLAN.md` 기준 rotating smoke test 자산 디렉토리와 wave별 `deploy.yml` 초안 생성
- `aws-rotating-smoke` 자산 기준으로 bastion 업로드/placeholder 치환 후 실제 dry-run 및 wave별 smoke 실행 검증
- wave 3 교차 분산 smoke 실행 및 wave 4 bootstrap dry-run 결과 문서화
- 수정된 checksum 파서로 bastion 재배포 후 wave 1/wave 3 재실행 시 false-positive 재전송이 사라지는지 재검증
- `deploy.example.yml` 또는 운영 문서에 `tags` / `--server-tag` / `--app-tag` 사용 예시 추가
- logger 출력 helper 확장을 실제로 할지, 아니면 현재 API를 기준으로 문서를 유지할지 결정
- 초안 YAML의 `TBD.server.values.yml`, Hamonica agent 배포 규칙, devapm/devapp/devdb 반영 방식 확정
- 운영 샘플/문서에서 `script.mode: local-file`를 어디에 적용할지 결정
- devdb PostgreSQL 17 실제 패키지명/리포지토리/서비스 enable 절차 확정
- `TEST/stock_company` 초안에서 app별 `extra_files[]`로 배포할 보조 스크립트/agent 파일 목록 정리
- 서버 단위 `extra_files[]`를 사용할 운영 케이스에서 압축 해제/설치 후속 단계가 필요한지 결정
- README의 Docker/M2 관련 설명은 실제 구현 범위가 확정될 때 다시 보강
- `ops/` 하위에 실제 운영 샘플 `deploy.yml`, values, config, script 템플릿을 어떤 서비스부터 채울지 우선순위 확정
- stock-company dev 운영 자산의 실제 host/bastion 값을 확정하고 placeholder를 치환한 뒤 bastion에서 dry-run/실배포 검증
- stock-company prod 운영 자산의 실제 host/bastion 값을 확정하고 placeholder를 치환한 뒤 bastion에서 dry-run/실배포 검증
- stock-company overpass 운영 자산의 실제 서버별 기동 순서/포트 충돌 여부 검증
- STG target SG에 추가한 prd bastion source `tcp/22` ingress를 `infra/istock-mall` Terraform 명시 rule로 정리
- AWS dev-test 검증 완료 후 test 리소스 destroy 범위가 dev-test 리소스만 포함하는지 재확인
- 새로 생성된 AWS dev-test 인프라에 stock-company dev placeholder를 반영하고 bastion dry-run/실배포 smoke test 재검증
- `devapp` 단일 구조와 `devbuild`/`hamonica`/`hamonica-adm`/`postgresql` 기준으로 stock-company dev placeholder를 다시 치환하고 bastion에서 재검증
- stock-company dev 서버별 Redis/MongoDB/Hazelcast/Elasticsearch 압축 해제 및 실행 절차를 후속 스크립트로 정리
- istock dev build 서버에서 Docker Compose 수동 설치 후 `docker compose version`, Docker 권한(`jenkins` user/group), Jenkins `aws --version`, GitLab external URL/IP, 80/443/8080/8081/2222 포트 충돌 여부를 실제 서버 기준으로 검증
- 보류 중인 ScyllaDB compose/config/manager-agent 배포 파일 구성을 확정한 뒤 `devapm2`에 재반영
- bo-app EC2 2대에 nginx bootstrap을 `--server-tag bo-app` 기준으로 dry-run/실배포 검증
- WAS가 실행되는 EC2별 실제 `config/<service>.env` 값을 서비스별 `env/*.env.sample` 기준으로 채운 뒤 Spring Boot 프로세스에서 placeholder가 정상 해석되는지 검증
- iSTOCK-MALL AWS 인프라 Terraform 코드는 main이 아닌 별도 운영 브랜치/저장소에서 state/import 정책과 함께 관리
- search1/search2에서 target-side bastion `known_hosts` 등록 warning의 네트워크/보안그룹 원인을 확인하고 필요 시 예외 처리 또는 SG 규칙을 정리
- `/app/elasticsearch`처럼 서비스 유저가 소유한 경로에 대용량 archive를 반영할 때 deployer가 staging path + sudo extract + 최종 chown 흐름을 선언적으로 지원하도록 개선
