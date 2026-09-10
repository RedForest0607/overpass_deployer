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

// Execute는 GitHub 릴리즈 확인부터 체크섬 검증, 바이너리 교체까지 self-update 흐름을 실행한다.
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

// requestedRelease는 명시된 버전이 있으면 해당 태그를, 없으면 최신 릴리즈를 조회한다.
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

// resolveExecutablePath는 테스트용 명시 경로나 현재 실행 파일 경로를 업데이트 대상으로 결정한다.
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

// selectArchiveAsset은 현재 OS/아키텍처와 버전에 맞는 릴리즈 압축 파일을 선택한다.
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

// selectChecksumAsset은 릴리즈 자산 중 checksums.txt 다운로드 URL을 찾는다.
func selectChecksumAsset(release *githubRelease) (string, error) {
	for _, asset := range release.Assets {
		if asset.Name == checksumAsset {
			return asset.BrowserDownloadURL, nil
		}
	}

	return "", fmt.Errorf("release %s does not include %s", release.TagName, checksumAsset)
}

// downloadChecksums는 원격 체크섬 파일을 내려받아 파일명별 SHA256 맵으로 파싱한다.
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

// validateDownloadedArchive는 다운로드한 압축 파일의 SHA256이 릴리즈 체크섬과 일치하는지 검증한다.
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

// downloadFile은 릴리즈 자산을 임시 파일로 저장하고 저장된 경로를 반환한다.
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

// downloadBytes는 컨텍스트 취소와 HTTP 상태 검사를 적용해 원격 바이트를 내려받는다.
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

// sameVersion은 v 접두사 차이를 무시하고 현재 버전과 대상 버전이 같은지 비교한다.
func sameVersion(currentVersion, targetVersion string) bool {
	current := normalizeVersion(currentVersion)
	target := normalizeVersion(targetVersion)
	if current == "" || target == "" {
		return false
	}

	return current == target
}

// normalizeVersion은 버전 비교를 위해 공백과 선행 v 접두사를 제거한다.
func normalizeVersion(version string) string {
	trimmed := strings.TrimSpace(version)
	if trimmed == "" {
		return ""
	}

	return strings.TrimPrefix(trimmed, "v")
}
