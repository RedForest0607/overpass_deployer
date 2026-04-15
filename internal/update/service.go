package update

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	binaryName    = "deploy"
	checksumAsset = "checksums.txt"
)

func Execute(ctx context.Context, cfg Config, opts Options) (*Result, error) {
	cfg = cfg.withRuntimeDefaults()

	client, err := newGitHubClient(cfg)
	if err != nil {
		return nil, err
	}

	executablePath, err := resolveExecutablePath(cfg.ExecutablePath)
	if err != nil {
		return nil, err
	}

	release, err := requestedRelease(ctx, client, opts.TargetVersion)
	if err != nil {
		return nil, err
	}

	result := &Result{
		CurrentVersion: strings.TrimSpace(cfg.CurrentVersion),
		TargetVersion:  release.TagName,
		ExecutablePath: executablePath,
		ReleaseURL:     release.HTMLURL,
	}

	if sameVersion(result.CurrentVersion, release.TagName) {
		result.UpToDate = true
		return result, nil
	}

	archiveAsset, err := selectArchiveAsset(release)
	if err != nil {
		return nil, err
	}

	checksumURL, err := selectChecksumAsset(release)
	if err != nil {
		return nil, err
	}

	result.AssetName = archiveAsset.Name
	if opts.CheckOnly {
		return result, nil
	}

	archivePath, err := downloadFile(ctx, cfg.HTTPClient, archiveAsset.BrowserDownloadURL, archiveAsset.Name)
	if err != nil {
		return nil, fmt.Errorf("downloading release archive %s: %w", archiveAsset.Name, err)
	}
	defer os.Remove(archivePath)

	checksumMap, err := downloadChecksums(ctx, cfg.HTTPClient, checksumURL)
	if err != nil {
		return nil, err
	}

	expectedChecksum, ok := checksumMap[archiveAsset.Name]
	if !ok {
		return nil, fmt.Errorf("checksum for %s not found in %s", archiveAsset.Name, checksumAsset)
	}

	if err := validateDownloadedArchive(archivePath, expectedChecksum); err != nil {
		return nil, err
	}

	extractedBinaryPath, err := extractBinaryFromArchive(archivePath, binaryName)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(filepath.Dir(extractedBinaryPath))

	if err := replaceExecutable(executablePath, extractedBinaryPath); err != nil {
		return nil, fmt.Errorf("installing updated binary: %w", err)
	}

	result.Updated = true
	return result, nil
}

func requestedRelease(ctx context.Context, client *githubClient, targetVersion string) (*githubRelease, error) {
	if strings.TrimSpace(targetVersion) != "" {
		release, err := client.releaseByTag(ctx, targetVersion)
		if err != nil {
			return nil, fmt.Errorf("fetching release %q: %w", targetVersion, err)
		}

		return release, nil
	}

	release, err := client.latestRelease(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching latest release: %w", err)
	}

	return release, nil
}

func resolveExecutablePath(explicitPath string) (string, error) {
	if explicitPath != "" {
		return explicitPath, nil
	}

	executablePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locating current executable: %w", err)
	}

	return executablePath, nil
}

func selectArchiveAsset(release *githubRelease) (githubAsset, error) {
	normalizedVersion := normalizeVersion(release.TagName)
	expectedNames := []string{
		fmt.Sprintf("deploy_%s_%s_%s.tar.gz", normalizedVersion, runtime.GOOS, runtime.GOARCH),
		fmt.Sprintf("deploy_%s_%s_%s.tar.gz", release.TagName, runtime.GOOS, runtime.GOARCH),
	}
	for _, asset := range release.Assets {
		for _, expectedName := range expectedNames {
			if asset.Name == expectedName {
				return asset, nil
			}
		}
	}

	return githubAsset{}, fmt.Errorf("release %s does not include asset %s for %s/%s", release.TagName, expectedNames[0], runtime.GOOS, runtime.GOARCH)
}

func selectChecksumAsset(release *githubRelease) (string, error) {
	for _, asset := range release.Assets {
		if asset.Name == checksumAsset {
			return asset.BrowserDownloadURL, nil
		}
	}

	return "", fmt.Errorf("release %s does not include %s", release.TagName, checksumAsset)
}

func downloadChecksums(ctx context.Context, client *http.Client, checksumURL string) (map[string]string, error) {
	body, err := downloadBytes(ctx, client, checksumURL)
	if err != nil {
		return nil, fmt.Errorf("downloading %s: %w", checksumAsset, err)
	}

	checksumMap, err := parseChecksums(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", checksumAsset, err)
	}

	return checksumMap, nil
}

func validateDownloadedArchive(archivePath, expectedChecksum string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("opening downloaded archive %s: %w", archivePath, err)
	}
	defer file.Close()

	if err := verifySHA256(expectedChecksum, file); err != nil {
		return fmt.Errorf("verifying release archive %s: %w", filepath.Base(archivePath), err)
	}

	return nil
}

func downloadFile(ctx context.Context, client *http.Client, url, fileName string) (string, error) {
	body, err := downloadBytes(ctx, client, url)
	if err != nil {
		return "", err
	}

	tempFile, err := os.CreateTemp("", "deploy-update-download-*"+filepath.Ext(fileName))
	if err != nil {
		return "", fmt.Errorf("creating temp download file: %w", err)
	}

	if _, err := tempFile.Write(body); err != nil {
		tempFile.Close()
		os.Remove(tempFile.Name())
		return "", fmt.Errorf("writing temp download file: %w", err)
	}

	if err := tempFile.Close(); err != nil {
		os.Remove(tempFile.Name())
		return "", fmt.Errorf("closing temp download file: %w", err)
	}

	return tempFile.Name(), nil
}

func downloadBytes(ctx context.Context, client *http.Client, downloadURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating download request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending download request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("download failed with status %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading download response: %w", err)
	}

	return body, nil
}

func sameVersion(currentVersion, targetVersion string) bool {
	current := normalizeVersion(currentVersion)
	target := normalizeVersion(targetVersion)
	if current == "" || target == "" {
		return false
	}

	return current == target
}

func normalizeVersion(version string) string {
	trimmed := strings.TrimSpace(version)
	if trimmed == "" {
		return ""
	}

	return strings.TrimPrefix(trimmed, "v")
}
