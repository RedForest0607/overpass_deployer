# Release Distribution Guide

`overpass-deployer`는 단일 바이너리 CLI입니다. M1 기준 릴리스 표준은 `goreleaser`로 플랫폼별 tar.gz와 `checksums.txt`를 만들고, GitHub Releases를 1차 소스로 사용하며 GitLab에는 같은 산출물을 보조 채널로 업로드하는 방식입니다.

## 1. Release metadata

`deploy version`은 아래 빌드 메타데이터를 출력합니다.

- version
- commit
- built at
- built by
- platform
- repository

`goreleaser`는 아래 ldflags를 주입합니다.

- `Version`
- `Commit`
- `Date`
- `BuiltBy`
- `RepoOwner`
- `RepoName`
- `GitHubAPIBase`

GitHub owner/repo를 바이너리에 심으려면 릴리스 시 아래 환경변수를 설정합니다.

```bash
export RELEASE_OWNER=<github-org-or-user>
export RELEASE_REPO=overpassDeployer
```

이미 `DEPLOY_RELEASE_OWNER`, `DEPLOY_RELEASE_REPO`를 쓰고 있다면 같은 값으로 대체해도 됩니다.

## 2. Build and publish

스냅샷 빌드:

```bash
goreleaser build --snapshot --clean
```

정식 릴리스:

```bash
git tag v1.0.0
goreleaser release --clean
```

산출물 이름 규칙:

- `deploy_VERSION_linux_amd64.tar.gz`
- `deploy_VERSION_linux_arm64.tar.gz`
- `deploy_VERSION_darwin_amd64.tar.gz`
- `deploy_VERSION_darwin_arm64.tar.gz`
- `checksums.txt`

GitHub는 `goreleaser release`의 기본 릴리스 경로를 사용합니다. GitLab은 같은 파일명을 유지한 채 Release asset 또는 Package Registry로 업로드합니다.

## 3. Manual install on a remote server

GitHub Release asset에서 직접 설치하는 기본 흐름입니다.

```bash
curl -L -o deploy.tar.gz https://github.com/<owner>/<repo>/releases/download/v1.0.0/deploy_1.0.0_linux_amd64.tar.gz
curl -L -o checksums.txt https://github.com/<owner>/<repo>/releases/download/v1.0.0/checksums.txt
sha256sum -c checksums.txt --ignore-missing
tar -xzf deploy.tar.gz
chmod +x deploy
mkdir -p ~/bin
mv ./deploy ~/bin/deploy
~/bin/deploy version
```

GitLab도 동일하게 tar.gz와 `checksums.txt`를 제공하면 같은 설치 절차를 따릅니다.

기본 권장 설치 위치는 root 권한이 필요 없는 `~/bin/deploy`입니다.

## 4. Self-update

`deploy update`는 GitHub Release API를 조회해 현재 플랫폼의 새 아카이브를 내려받고 checksum 검증 후 현재 실행 파일을 교체합니다.

```bash
deploy update
deploy update --check
deploy update --version v1.0.0
```

동작 규칙:

1. 현재 버전 확인
2. latest release 또는 지정 태그 조회
3. 현재 OS/ARCH용 asset 선택
4. `checksums.txt` 다운로드
5. SHA256 검증
6. 현재 실행 파일 경로를 원자적으로 교체 시도

self-update가 동작하려면 릴리스 저장소 정보가 필요합니다. 일반적으로 `goreleaser` ldflags로 주입되며, 개발/수동 빌드 환경에서는 아래 환경변수로 덮어쓸 수 있습니다.

```bash
export DEPLOY_RELEASE_OWNER=<github-org-or-user>
export DEPLOY_RELEASE_REPO=overpassDeployer
deploy update --check
```

GitHub Enterprise API를 쓰는 경우에는 아래 환경변수로 API base URL을 바꿀 수 있습니다.

```bash
export DEPLOY_GITHUB_API_URL=https://github.example.com/api/v3
```

이 값이 비어 있으면 릴리스 빌드와 런타임 모두 기본값 `https://api.github.com`을 사용합니다.

## 5. Rollback

특정 버전으로 롤백하려면 수동 설치 또는 self-update의 태그 지정 모드를 사용합니다.

```bash
deploy update --version v1.0.0
```

권한 문제로 self-update가 실패하면 수동 다운로드 방식으로 `~/bin/deploy`를 교체합니다.
