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

func (c *githubClient) latestRelease(ctx context.Context) (*githubRelease, error) {
	return c.fetchRelease(ctx, fmt.Sprintf("%s/repos/%s/%s/releases/latest", c.baseURL, c.repoOwner, c.repoName))
}

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

func normalizeBaseURL(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", err
	}

	return strings.TrimRight(parsed.String(), "/"), nil
}

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
