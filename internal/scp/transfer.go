package scp

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go-deployer/internal/ssh"
	"go-deployer/pkg/logger"

	"github.com/pkg/sftp"
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
	out, err := runner.Run(fmt.Sprintf("sha256sum %s", ssh.ShellQuote(path)))
	if err != nil {
		// Only ignore if the file genuinely does not exist
		if strings.Contains(out, "No such file") {
			return "", nil
		}
		return "", fmt.Errorf("remote sha256sum failed (out: %s): %w", out, err)
	}
	parts := strings.Fields(out)
	if len(parts) > 0 {
		return parts[0], nil
	}
	return "", nil
}

type progressWriter struct {
	host       string
	fileName   string
	totalBytes int64
	written    int64
	lastReport int64
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.written += int64(n)
	// Report every 10MB
	if pw.written-pw.lastReport >= 10*1024*1024 {
		logger.Info(pw.host, "Transferring %s... (%.1f MB)", pw.fileName, float64(pw.written)/1024/1024)
		pw.lastReport = pw.written
	}
	return n, nil
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
	pw := &progressWriter{
		host:       host,
		fileName:   fileName,
		totalBytes: localInfo.Size(),
		written:    0,
		lastReport: 0,
	}

	writer := io.MultiWriter(remoteFile, pw)
	if _, err := io.Copy(writer, localFile); err != nil {
		return fmt.Errorf("uploading file %s: %w", fileName, err)
	}

	logger.Ok(host, "Transferred %s (%.1f MB)", fileName, float64(localInfo.Size())/1024/1024)
	return nil
}
