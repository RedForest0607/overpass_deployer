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

	parts := strings.Fields(out)
	if len(parts) > 0 {
		return parts[0], nil
	}
	return "", nil
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

func stdoutIsInteractive() bool {
	info, err := os.Stdout.Stat()
	if err != nil {
		return false
	}

	return (info.Mode() & os.ModeCharDevice) != 0
}

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

func progressPercent(written, total int64) int {
	if total <= 0 {
		return 0
	}

	return int(float64(clampBytes(written, total)) / float64(total) * 100)
}

func clampBytes(written, total int64) int64 {
	if written < 0 {
		return 0
	}
	if written > total {
		return total
	}
	return written
}

func formatMiB(size int64) string {
	return fmt.Sprintf("%.1f MiB", float64(size)/float64(bytesPerMiB))
}

type TransferOptions struct {
	DryRun bool
	Host   string
}

// Transfer transfers a local file to a remote path via SFTP, skipping if the SHA256 checksum matches.
func Transfer(client *ssh.Client, localPath, remotePath string, opts TransferOptions) error {
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

	remoteHash, err := calculateRemoteSHA256(client, remotePath)
	if err != nil {
		return fmt.Errorf("calculating remote sha256 for %s: %w", remotePath, err)
	}
	if localHash == remoteHash {
		logger.Skip(host, "%s unchanged", fileName)
		return nil
	}

	sftpClient, err := sftp.NewClient(client.RawClient())
	if err != nil {
		return fmt.Errorf("creating sftp client: %w", err)
	}
	defer sftpClient.Close()

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
