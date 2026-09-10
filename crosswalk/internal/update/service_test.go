package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestExecuteCheckOnlyReturnsAvailableUpdate(t *testing.T) {
	t.Parallel()

	archiveName := fmt.Sprintf("deploy_v1.2.3_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	goreleaserArchiveName := fmt.Sprintf("deploy_1.2.3_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	server := newReleaseServer(t, archiveName, []byte("updated-binary"))
	defer server.Close()

	result, err := Execute(context.Background(), Config{
		CurrentVersion: "v1.2.2",
		RepoOwner:      "acme",
		RepoName:       "deploy",
		GitHubAPIBase:  server.URL,
		HTTPClient:     server.Client(),
		ExecutablePath: filepath.Join(t.TempDir(), "deploy"),
	}, Options{CheckOnly: true})
	if err != nil {
		t.Fatalf("expected check-only update to succeed, got %v", err)
	}

	if result.UpToDate {
		t.Fatalf("expected update to be available")
	}
	if result.TargetVersion != "v1.2.3" {
		t.Fatalf("expected target version v1.2.3, got %q", result.TargetVersion)
	}
	if result.AssetName != archiveName {
		t.Fatalf("expected asset name %q, got %q", archiveName, result.AssetName)
	}

	serverWithGoreleaserName := newReleaseServer(t, goreleaserArchiveName, []byte("updated-binary"))
	defer serverWithGoreleaserName.Close()

	result, err = Execute(context.Background(), Config{
		CurrentVersion: "v1.2.2",
		RepoOwner:      "acme",
		RepoName:       "deploy",
		GitHubAPIBase:  serverWithGoreleaserName.URL,
		HTTPClient:     serverWithGoreleaserName.Client(),
		ExecutablePath: filepath.Join(t.TempDir(), "deploy"),
	}, Options{CheckOnly: true})
	if err != nil {
		t.Fatalf("expected goreleaser-named asset to succeed, got %v", err)
	}

	if result.AssetName != goreleaserArchiveName {
		t.Fatalf("expected goreleaser asset name %q, got %q", goreleaserArchiveName, result.AssetName)
	}
}

func TestExecuteUpdatesBinary(t *testing.T) {
	t.Parallel()

	archiveName := fmt.Sprintf("deploy_1.2.3_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	server := newReleaseServer(t, archiveName, []byte("updated-binary"))
	defer server.Close()

	dir := t.TempDir()
	targetPath := filepath.Join(dir, "deploy")
	if err := os.WriteFile(targetPath, []byte("old-binary"), 0o755); err != nil {
		t.Fatalf("writing old executable: %v", err)
	}

	result, err := Execute(context.Background(), Config{
		CurrentVersion: "v1.2.2",
		RepoOwner:      "acme",
		RepoName:       "deploy",
		GitHubAPIBase:  server.URL,
		HTTPClient:     server.Client(),
		ExecutablePath: targetPath,
	}, Options{})
	if err != nil {
		t.Fatalf("expected update to succeed, got %v", err)
	}

	if !result.Updated {
		t.Fatalf("expected result to indicate update applied")
	}

	content, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("reading updated executable: %v", err)
	}
	if string(content) != "updated-binary" {
		t.Fatalf("expected updated executable contents, got %q", string(content))
	}
}

func TestExecuteFailsWhenAssetMissingForPlatform(t *testing.T) {
	t.Parallel()

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/acme/deploy/releases/latest" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"tag_name":"v1.2.3",
			"html_url":"https://example.com/releases/v1.2.3",
			"assets":[
					{"name":"deploy_1.2.3_linux_arm64.tar.gz","browser_download_url":"https://example.com/linux-arm64.tar.gz"},
					{"name":"checksums.txt","browser_download_url":"https://example.com/checksums.txt"}
				]
			}`))
	}))
	defer server.Close()

	_, err := Execute(context.Background(), Config{
		CurrentVersion: "v1.2.2",
		RepoOwner:      "acme",
		RepoName:       "deploy",
		GitHubAPIBase:  server.URL,
		HTTPClient:     server.Client(),
		ExecutablePath: filepath.Join(t.TempDir(), "deploy"),
	}, Options{CheckOnly: true})
	if err == nil || !strings.Contains(err.Error(), "does not include asset") {
		t.Fatalf("expected missing asset error, got %v", err)
	}
}

func TestExecuteFailsOnChecksumMismatch(t *testing.T) {
	t.Parallel()

	archiveName := fmt.Sprintf("deploy_1.2.3_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	archiveBytes := makeArchive(t, []byte("updated-binary"))

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/acme/deploy/releases/latest":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(fmt.Sprintf(`{
				"tag_name":"v1.2.3",
				"html_url":"https://example.com/releases/v1.2.3",
				"assets":[
					{"name":"%s","browser_download_url":"%s/assets/%s"},
					{"name":"checksums.txt","browser_download_url":"%s/assets/checksums.txt"}
				]
			}`, archiveName, server.URL, archiveName, server.URL)))
		case "/assets/" + archiveName:
			_, _ = w.Write(archiveBytes)
		case "/assets/checksums.txt":
			_, _ = w.Write([]byte("deadbeef  " + archiveName + "\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	dir := t.TempDir()
	targetPath := filepath.Join(dir, "deploy")
	if err := os.WriteFile(targetPath, []byte("old-binary"), 0o755); err != nil {
		t.Fatalf("writing old executable: %v", err)
	}

	_, err := Execute(context.Background(), Config{
		CurrentVersion: "v1.2.2",
		RepoOwner:      "acme",
		RepoName:       "deploy",
		GitHubAPIBase:  server.URL,
		HTTPClient:     server.Client(),
		ExecutablePath: targetPath,
	}, Options{})
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("expected checksum mismatch error, got %v", err)
	}
}

func newReleaseServer(t *testing.T, archiveName string, binaryContent []byte) *httptest.Server {
	t.Helper()

	archiveBytes := makeArchive(t, binaryContent)
	checksum := fmt.Sprintf("%x", sha256.Sum256(archiveBytes))

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/acme/deploy/releases/latest":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(fmt.Sprintf(`{
				"tag_name":"v1.2.3",
				"html_url":"https://example.com/releases/v1.2.3",
				"assets":[
					{"name":"%s","browser_download_url":"%s/assets/%s"},
					{"name":"checksums.txt","browser_download_url":"%s/assets/checksums.txt"}
				]
			}`, archiveName, server.URL, archiveName, server.URL)))
		case "/assets/" + archiveName:
			_, _ = w.Write(archiveBytes)
		case "/assets/checksums.txt":
			_, _ = w.Write([]byte(fmt.Sprintf("%s  %s\n", checksum, archiveName)))
		default:
			http.NotFound(w, r)
		}
	}))

	return server
}

func makeArchive(t *testing.T, binaryContent []byte) []byte {
	t.Helper()

	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)

	header := &tar.Header{
		Name: "deploy",
		Mode: 0o755,
		Size: int64(len(binaryContent)),
	}

	if err := tarWriter.WriteHeader(header); err != nil {
		t.Fatalf("writing tar header: %v", err)
	}
	if _, err := tarWriter.Write(binaryContent); err != nil {
		t.Fatalf("writing tar contents: %v", err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("closing tar writer: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("closing gzip writer: %v", err)
	}

	return buffer.Bytes()
}
