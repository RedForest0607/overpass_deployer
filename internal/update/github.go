package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type githubRelease struct {
	TagName    string        `json:"tag_name"`
	HTMLURL    string        `json:"html_url"`
	Prerelease bool          `json:"prerelease"`
	Assets     []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type githubClient struct {
	baseURL    string
	repoOwner  string
	repoName   string
	httpClient *http.Client
}

// newGitHubClient는 저장소 메타데이터와 API base URL을 검증해 릴리즈 조회 클라이언트를 만든다.
func newGitHubClient(cfg Config) (*githubClient, error) {
	if cfg.RepoOwner == "" || cfg.RepoName == "" {
		return nil, fmt.Errorf("release repository is not configured; set build metadata or %s/%s", ownerEnvKey, repoEnvKey)
	}

	baseURL, err := normalizeBaseURL(cfg.GitHubAPIBase)
	if err != nil {
		return nil, fmt.Errorf("normalizing github api base url: %w", err)
	}

	return &githubClient{
		baseURL:    baseURL,
		repoOwner:  cfg.RepoOwner,
		repoName:   cfg.RepoName,
		httpClient: cfg.HTTPClient,
	}, nil
}

// latestRelease는 GitHub latest release API에서 최신 안정 릴리즈 정보를 가져온다.
func (c *githubClient) latestRelease(ctx context.Context) (*githubRelease, error) {
	return c.fetchRelease(ctx, fmt.Sprintf("%s/repos/%s/%s/releases/latest", c.baseURL, c.repoOwner, c.repoName))
}

// releaseByTag는 v 접두사 유무를 모두 시도해 사용자가 요청한 릴리즈 태그를 찾는다.
func (c *githubClient) releaseByTag(ctx context.Context, tag string) (*githubRelease, error) {
	var lastErr error
	for _, candidate := range tagCandidates(tag) {
		release, err := c.fetchRelease(ctx, fmt.Sprintf("%s/repos/%s/%s/releases/tags/%s", c.baseURL, c.repoOwner, c.repoName, url.PathEscape(candidate)))
		if err == nil {
			return release, nil
		}
		lastErr = err
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("release tag %q not found", tag)
	}

	return nil, lastErr
}

// fetchRelease는 GitHub 릴리즈 API 응답을 검증하고 prerelease 업데이트를 차단한다.
func (c *githubClient) fetchRelease(ctx context.Context, endpoint string) (*githubRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating github request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting github release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("github release request failed with status %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("decoding github release response: %w", err)
	}

	if release.Prerelease {
		return nil, fmt.Errorf("github returned prerelease %q; prerelease updates are not supported in M1", release.TagName)
	}

	return &release, nil
}

// normalizeBaseURL은 GitHub API base URL의 끝 슬래시를 제거해 endpoint 조립을 안정화한다.
func normalizeBaseURL(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", err
	}

	return strings.TrimRight(parsed.String(), "/"), nil
}

// tagCandidates는 사용자가 입력한 태그와 v 접두사 변형을 중복 없이 만든다.
func tagCandidates(tag string) []string {
	trimmed := strings.TrimSpace(tag)
	if trimmed == "" {
		return nil
	}

	candidates := []string{trimmed}
	if strings.HasPrefix(trimmed, "v") {
		candidates = append(candidates, strings.TrimPrefix(trimmed, "v"))
	} else {
		candidates = append(candidates, "v"+trimmed)
	}

	return dedupeStrings(candidates)
}

// dedupeStrings는 문자열 목록의 최초 등장 순서를 유지하며 중복을 제거한다.
func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))

	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}

	return result
}
