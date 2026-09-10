package update

import (
	"net/http"
	"os"
	"time"
)

const (
	defaultGitHubAPIBaseURL = "https://api.github.com"
	ownerEnvKey             = "DEPLOY_RELEASE_OWNER"
	repoEnvKey              = "DEPLOY_RELEASE_REPO"
	apiBaseEnvKey           = "DEPLOY_GITHUB_API_URL"
)

type Config struct {
	CurrentVersion string
	RepoOwner      string
	RepoName       string
	GitHubAPIBase  string
	ExecutablePath string
	HTTPClient     *http.Client
}

type Options struct {
	CheckOnly     bool
	TargetVersion string
}

type Result struct {
	CurrentVersion string
	TargetVersion  string
	ExecutablePath string
	AssetName      string
	ReleaseURL     string
	UpToDate       bool
	Updated        bool
}

// withRuntimeDefaults는 릴리즈 저장소와 HTTP 클라이언트 설정을 환경 변수와 기본값으로 보정한다.
func (c Config) withRuntimeDefaults() Config {
	if c.RepoOwner == "" {
		c.RepoOwner = os.Getenv(ownerEnvKey)
	}
	if c.RepoName == "" {
		c.RepoName = os.Getenv(repoEnvKey)
	}
	if c.GitHubAPIBase == "" {
		c.GitHubAPIBase = os.Getenv(apiBaseEnvKey)
	}
	if c.GitHubAPIBase == "" {
		c.GitHubAPIBase = defaultGitHubAPIBaseURL
	}
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}

	return c
}
