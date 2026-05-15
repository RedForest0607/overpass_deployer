package scp

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go-deployer/internal/ssh"
	"go-deployer/pkg/logger"

	"github.com/pkg/sftp"
)

const missingRemoteSHA256Marker = "__OVERPASS_REMOTE_FILE_MISSING__"

const (
	progressBarWidth          = 28
	progressUpdateEvery       = 250 * time.Millisecond
	progressLogStepPct        = 10
	bytesPerMiB         int64 = 1024 * 1024
)

// calculateLocalSHA256은 로컬 파일 내용을 기준으로 전송 전 비교용 SHA256 해시를 계산한다.
func calculateLocalSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// calculateRemoteSHA256은 원격 파일이 있을 때 SHA256을 가져오고 없으면 빈 해시로 처리한다.
func calculateRemoteSHA256(runner ssh.Runner, path string) (string, error) {
	command := fmt.Sprintf(
		"sh -lc %s",
		ssh.ShellQuote(
			fmt.Sprintf("if [ -f %s ]; then sha256sum %s; else echo %s; fi",
				ssh.ShellQuote(path),
				ssh.ShellQuote(path),
				missingRemoteSHA256Marker,
			),
		),
	)

	out, err := runner.Run(command)
	if err != nil {
		return "", fmt.Errorf("remote sha256sum failed (out: %s): %w", out, err)
	}

	if out == missingRemoteSHA256Marker+"\n" || out == missingRemoteSHA256Marker {
		return "", nil
	}

	return parseRemoteSHA256Output(out), nil
}

// parseRemoteSHA256Output은 원격 명령 출력에서 실제 SHA256 값만 추출한다.
func parseRemoteSHA256Output(out string) string {
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if trimmed == missingRemoteSHA256Marker {
			return ""
		}

		fields := strings.Fields(trimmed)
		if len(fields) == 0 {
			continue
		}
		if isSHA256Hex(fields[0]) {
			return strings.ToLower(fields[0])
		}
	}

	return ""
}

// isSHA256Hex는 문자열이 SHA256 해시 길이와 16진수 문자만 갖는지 확인한다.
func isSHA256Hex(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}

	for _, r := range value {
		switch {
		case '0' <= r && r <= '9':
		case 'a' <= r && r <= 'f':
		case 'A' <= r && r <= 'F':
		default:
			return false
		}
	}

	return true
}

type progressWriter struct {
	host            string
	fileName        string
	totalBytes      int64
	written         int64
	lastLoggedPct   int
	lastRenderAt    time.Time
	render          func(state progressState)
	finishLine      func(state progressState)
	isInteractive   bool
	progressLineSet bool
}

type progressState struct {
	host       string
	fileName   string
	totalBytes int64
	written    int64
}

var progressOutputMu sync.Mutex

// newProgressWriter는 터미널 환경에 맞춰 파일 전송 진행률 출력기를 초기화한다.
func newProgressWriter(host, fileName string, totalBytes int64) *progressWriter {
	return &progressWriter{
		host:          host,
		fileName:      fileName,
		totalBytes:    totalBytes,
		render:        renderProgress,
		finishLine:    finishProgress,
		isInteractive: stdoutIsInteractive(),
	}
}

// Write는 전송된 바이트 수를 누적하고 필요한 시점에 진행률을 출력한다.
func (pw *progressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.written += int64(n)

	state := progressState{
		host:       pw.host,
		fileName:   pw.fileName,
		totalBytes: pw.totalBytes,
		written:    pw.written,
	}
	if pw.shouldRenderProgress() {
		pw.render(state)
		pw.progressLineSet = true
	}

	return n, nil
}

// shouldRenderProgress는 대화형/비대화형 출력 환경별 진행률 갱신 주기를 결정한다.
func (pw *progressWriter) shouldRenderProgress() bool {
	if pw.totalBytes <= 0 {
		return false
	}

	if pw.written >= pw.totalBytes {
		return true
	}

	if pw.isInteractive {
		now := time.Now()
		if pw.lastRenderAt.IsZero() || now.Sub(pw.lastRenderAt) >= progressUpdateEvery {
			pw.lastRenderAt = now
			return true
		}
		return false
	}

	currentPct := progressPercent(pw.written, pw.totalBytes)
	if currentPct >= pw.lastLoggedPct+progressLogStepPct {
		pw.lastLoggedPct = currentPct - (currentPct % progressLogStepPct)
		return true
	}
	return false
}

// Finish는 전송 완료 시 최종 진행률 줄을 보장한다.
func (pw *progressWriter) Finish() {
	if pw.totalBytes <= 0 {
		return
	}

	state := progressState{
		host:       pw.host,
		fileName:   pw.fileName,
		totalBytes: pw.totalBytes,
		written:    pw.totalBytes,
	}
	if !pw.progressLineSet {
		pw.render(state)
	}
	pw.finishLine(state)
}

// stdoutIsInteractive는 진행률을 덮어쓸 수 있는 터미널인지 확인한다.
func stdoutIsInteractive() bool {
	info, err := os.Stdout.Stat()
	if err != nil {
		return false
	}

	return (info.Mode() & os.ModeCharDevice) != 0
}

// renderProgress는 현재 전송 상태를 진행률 한 줄로 출력한다.
func renderProgress(state progressState) {
	progressOutputMu.Lock()
	defer progressOutputMu.Unlock()

	line := formatProgressLine(state)
	if stdoutIsInteractive() {
		fmt.Fprintf(os.Stdout, "\r%s", line)
		return
	}

	fmt.Fprintln(os.Stdout, line)
}

// finishProgress는 완료된 진행률 줄을 줄바꿈과 함께 마무리한다.
func finishProgress(state progressState) {
	progressOutputMu.Lock()
	defer progressOutputMu.Unlock()

	line := formatProgressLine(state)
	if stdoutIsInteractive() {
		fmt.Fprintf(os.Stdout, "\r%s\n", line)
		return
	}

	fmt.Fprintln(os.Stdout, line)
}

// formatProgressLine은 호스트, 파일명, 퍼센트, 용량을 포함한 진행률 문자열을 만든다.
func formatProgressLine(state progressState) string {
	hostPrefix := ""
	if state.host != "" {
		hostPrefix = fmt.Sprintf("[%s] ", state.host)
	}

	return fmt.Sprintf(
		"%s%s %s %5.1f%% (%s/%s)",
		hostPrefix,
		state.fileName,
		progressBar(state.written, state.totalBytes),
		float64(progressPercent(state.written, state.totalBytes)),
		formatMiB(state.written),
		formatMiB(state.totalBytes),
	)
}

// progressBar는 전송 바이트 비율을 고정 폭 ASCII 막대로 변환한다.
func progressBar(written, total int64) string {
	if total <= 0 {
		return "[" + strings.Repeat("-", progressBarWidth) + "]"
	}

	filled := int(float64(clampBytes(written, total)) / float64(total) * float64(progressBarWidth))
	if filled > progressBarWidth {
		filled = progressBarWidth
	}

	return "[" + strings.Repeat("#", filled) + strings.Repeat("-", progressBarWidth-filled) + "]"
}

// progressPercent는 전송 바이트 비율을 0에서 100 사이 정수 퍼센트로 계산한다.
func progressPercent(written, total int64) int {
	if total <= 0 {
		return 0
	}

	return int(float64(clampBytes(written, total)) / float64(total) * 100)
}

// clampBytes는 진행률 계산 중 음수나 총량 초과 값이 표시되지 않도록 보정한다.
func clampBytes(written, total int64) int64 {
	if written < 0 {
		return 0
	}
	if written > total {
		return total
	}
	return written
}

// formatMiB는 바이트 수를 사람이 읽기 쉬운 MiB 문자열로 변환한다.
func formatMiB(size int64) string {
	return fmt.Sprintf("%.1f MiB", float64(size)/float64(bytesPerMiB))
}

type TransferOptions struct {
	DryRun  bool
	Host    string
	Session *TransferSession
}

type TransferSession struct {
	client       *ssh.Client
	sftpClient   *sftp.Client
	remoteHashes map[string]string
}

func NewTransferSession(client *ssh.Client, remoteHashes map[string]string) (*TransferSession, error) {
	if client == nil {
		return nil, fmt.Errorf("ssh client is required")
	}

	sftpClient, err := sftp.NewClient(client.RawClient())
	if err != nil {
		return nil, fmt.Errorf("creating sftp client: %w", err)
	}

	return &TransferSession{
		client:       client,
		sftpClient:   sftpClient,
		remoteHashes: remoteHashes,
	}, nil
}

func (s *TransferSession) Close() error {
	if s == nil || s.sftpClient == nil {
		return nil
	}
	return s.sftpClient.Close()
}

// Transfer는 SHA256 비교로 변경 여부를 확인한 뒤 필요한 경우에만 SFTP로 파일을 전송한다.
func Transfer(client *ssh.Client, localPath, remotePath string, opts TransferOptions) error {
	if opts.Session != nil {
		client = opts.Session.client
	}
	host := opts.Host
	if client != nil {
		host = client.Host()
	}
	fileName := filepath.Base(localPath)

	localInfo, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("stat local file %s: %w", localPath, err)
	}

	logger.Info(host, "Checking checksum for %s...", fileName)
	localHash, err := calculateLocalSHA256(localPath)
	if err != nil {
		return fmt.Errorf("calculating local sha256 for %s: %w", localPath, err)
	}

	if opts.DryRun {
		logger.Info(host, "DRY-RUN: would compare local sha256 %s for %s against %s", localHash, fileName, remotePath)
		logger.Info(host, "DRY-RUN: would upload %s to %s if checksum differs (%.1f MB)", fileName, remotePath, float64(localInfo.Size())/1024/1024)
		return nil
	}
	if client == nil {
		return fmt.Errorf("ssh client is required")
	}

	remoteHash, ok := opts.SessionRemoteHash(remotePath)
	if !ok {
		var err error
		remoteHash, err = calculateRemoteSHA256(client, remotePath)
		if err != nil {
			return fmt.Errorf("calculating remote sha256 for %s: %w", remotePath, err)
		}
	}
	if localHash == remoteHash {
		logger.Skip(host, "%s unchanged", fileName)
		return nil
	}

	sftpClient := opts.SessionSFTPClient()
	closeSFTP := false
	if sftpClient == nil {
		var err error
		sftpClient, err = sftp.NewClient(client.RawClient())
		if err != nil {
			return fmt.Errorf("creating sftp client: %w", err)
		}
		closeSFTP = true
	}
	if closeSFTP {
		defer sftpClient.Close()
	}

	remoteDir := filepath.ToSlash(filepath.Dir(remotePath))
	if err := sftpClient.MkdirAll(remoteDir); err != nil {
		return fmt.Errorf("creating remote directory %s: %w", remoteDir, err)
	}

	remoteFile, err := sftpClient.Create(remotePath)
	if err != nil {
		return fmt.Errorf("creating remote file %s: %w", remotePath, err)
	}
	defer remoteFile.Close()

	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("opening local file %s: %w", localPath, err)
	}
	defer localFile.Close()

	logger.Info(host, "Transferring %s...", fileName)
	pw := newProgressWriter(host, fileName, localInfo.Size())

	writer := io.MultiWriter(remoteFile, pw)
	if _, err := io.Copy(writer, localFile); err != nil {
		return fmt.Errorf("uploading file %s: %w", fileName, err)
	}
	pw.Finish()

	logger.Ok(host, "Transferred %s (%.1f MB)", fileName, float64(localInfo.Size())/1024/1024)
	return nil
}

func (opts TransferOptions) SessionRemoteHash(remotePath string) (string, bool) {
	if opts.Session == nil || opts.Session.remoteHashes == nil {
		return "", false
	}
	hash, ok := opts.Session.remoteHashes[remotePath]
	return hash, ok
}

func (opts TransferOptions) SessionSFTPClient() *sftp.Client {
	if opts.Session == nil {
		return nil
	}
	return opts.Session.sftpClient
}
