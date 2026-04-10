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

func (i Info) Repository() string {
	if i.RepoOwner == "" || i.RepoName == "" {
		return "unconfigured"
	}

	return fmt.Sprintf("%s/%s", i.RepoOwner, i.RepoName)
}

func valueOrDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}

	return value
}
