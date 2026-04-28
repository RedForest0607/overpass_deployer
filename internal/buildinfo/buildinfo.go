package buildinfo

import (
	"fmt"
	"runtime"
)

const (
	defaultVersion       = "dev"
	defaultCommit        = "none"
	defaultDate          = "unknown"
	defaultBuiltBy       = "unknown"
	defaultGitHubAPIBase = "https://api.github.com"
)

var (
	Version       = defaultVersion
	Commit        = defaultCommit
	Date          = defaultDate
	BuiltBy       = defaultBuiltBy
	RepoOwner     string
	RepoName      string
	GitHubAPIBase = defaultGitHubAPIBase
)

type Info struct {
	Version       string
	Commit        string
	Date          string
	BuiltBy       string
	RepoOwner     string
	RepoName      string
	GitHubAPIBase string
	GOOS          string
	GOARCH        string
}

// Current는 링커 플래그로 주입된 빌드 정보와 현재 런타임 플랫폼 정보를 합쳐 반환한다.
func Current() Info {
	return Info{
		Version:       valueOrDefault(Version, defaultVersion),
		Commit:        valueOrDefault(Commit, defaultCommit),
		Date:          valueOrDefault(Date, defaultDate),
		BuiltBy:       valueOrDefault(BuiltBy, defaultBuiltBy),
		RepoOwner:     RepoOwner,
		RepoName:      RepoName,
		GitHubAPIBase: valueOrDefault(GitHubAPIBase, defaultGitHubAPIBase),
		GOOS:          runtime.GOOS,
		GOARCH:        runtime.GOARCH,
	}
}

// Repository는 업데이트 확인에 사용할 GitHub 저장소 식별자를 사람이 읽기 좋은 형태로 만든다.
func (i Info) Repository() string {
	if i.RepoOwner == "" || i.RepoName == "" {
		return "unconfigured"
	}

	return fmt.Sprintf("%s/%s", i.RepoOwner, i.RepoName)
}

// valueOrDefault는 빌드 메타데이터가 비어 있을 때 개발용 기본값을 보정한다.
func valueOrDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}

	return value
}
