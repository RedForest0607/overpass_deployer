package scp

import (
	"os"
	"path/filepath"
	"testing"

	"go-deployer/internal/ssh"
)

func TestTransferDryRunWithoutClient(t *testing.T) {
	t.Helper()

	tempDir := t.TempDir()
	localPath := filepath.Join(tempDir, "app.jar")
	if err := os.WriteFile(localPath, []byte("jar-bytes"), 0o644); err != nil {
		t.Fatalf("write local file: %v", err)
	}

	if err := Transfer(nil, localPath, "/remote/app.jar", TransferOptions{
		DryRun: true,
		Host:   "app-01",
	}); err != nil {
		t.Fatalf("expected dry-run transfer to succeed without client, got %v", err)
	}
}

func TestCalculateRemoteSHA256ReturnsEmptyForMissingFile(t *testing.T) {
	t.Helper()

	runner := &stubRunner{
		output: missingRemoteSHA256Marker + "\n",
	}

	hash, err := calculateRemoteSHA256(runner, "/remote/app.jar")
	if err != nil {
		t.Fatalf("expected missing remote file to be treated as non-error, got %v", err)
	}
	if hash != "" {
		t.Fatalf("expected empty hash for missing remote file, got %q", hash)
	}
}

func TestCalculateRemoteSHA256ParsesHashOutput(t *testing.T) {
	t.Helper()

	runner := &stubRunner{
		output: "abc123  /remote/app.jar\n",
	}

	hash, err := calculateRemoteSHA256(runner, "/remote/app.jar")
	if err != nil {
		t.Fatalf("expected checksum parsing to succeed, got %v", err)
	}
	if hash != "abc123" {
		t.Fatalf("expected parsed hash abc123, got %q", hash)
	}
}

type stubRunner struct {
	output string
	err    error
}

func (s *stubRunner) Run(cmd string) (string, error) {
	return s.output, s.err
}

func (s *stubRunner) RunSudo(cmd string) (string, error) {
	return "", nil
}

func (s *stubRunner) Host() string {
	return "stub"
}

func (s *stubRunner) Close() error {
	return nil
}

var _ ssh.Runner = (*stubRunner)(nil)
