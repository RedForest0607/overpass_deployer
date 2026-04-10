package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReleaseByTagFallsBackToVPrefix(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/acme/deploy/releases/tags/1.2.3":
			http.NotFound(w, r)
		case "/repos/acme/deploy/releases/tags/v1.2.3":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tag_name":"v1.2.3","html_url":"https://example.com/releases/v1.2.3","assets":[]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := newGitHubClient(Config{
		RepoOwner:     "acme",
		RepoName:      "deploy",
		GitHubAPIBase: server.URL,
		HTTPClient:    server.Client(),
	})
	if err != nil {
		t.Fatalf("expected github client to be created, got %v", err)
	}

	release, err := client.releaseByTag(context.Background(), "1.2.3")
	if err != nil {
		t.Fatalf("expected tag lookup to succeed, got %v", err)
	}
	if release.TagName != "v1.2.3" {
		t.Fatalf("expected v-prefixed tag, got %q", release.TagName)
	}
}
