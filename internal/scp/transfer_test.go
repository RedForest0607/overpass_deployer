package scp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestFormatProgressLineIncludesPercentAndBar(t *testing.T) {
	t.Helper()

	line := formatProgressLine(progressState{
		host:       "app-01",
		fileName:   "auth-service.jar",
		totalBytes: 200,
		written:    100,
	})

	if !strings.Contains(line, "[app-01] auth-service.jar") {
		t.Fatalf("expected host and file name in progress line, got %q", line)
	}
	if !strings.Contains(line, "50.0%") {
		t.Fatalf("expected percentage in progress line, got %q", line)
	}
	if !strings.Contains(line, "[##############--------------]") {
		t.Fatalf("expected progress bar in progress line, got %q", line)
	}
}

func TestProgressBarClampsAtHundredPercent(t *testing.T) {
	t.Helper()

	bar := progressBar(150, 100)
	if bar != "[############################]" {
		t.Fatalf("expected full bar for overshoot, got %q", bar)
	}

	if got := progressPercent(150, 100); got != 100 {
		t.Fatalf("expected percent to clamp at 100, got %d", got)
	}
}

func TestProgressWriterNonInteractiveReportsByTenPercentSteps(t *testing.T) {
	t.Helper()

	renderCount := 0
	pw := newProgressWriter("app-01", "config.yml", 100)
	pw.isInteractive = false
	pw.render = func(state progressState) {
		renderCount++
	}
	pw.finishLine = func(state progressState) {}

	pw.Write(make([]byte, 5))
	pw.Write(make([]byte, 5))
	pw.Write(make([]byte, 9))
	pw.Write(make([]byte, 1))

	if renderCount != 2 {
		t.Fatalf("expected renders at 10%% and 20%% boundaries, got %d", renderCount)
	}
}

func TestProgressWriterInteractiveReportsAfterInterval(t *testing.T) {
	t.Helper()

	renderCount := 0
	pw := newProgressWriter("app-01", "config.yml", 100)
	pw.isInteractive = true
	pw.render = func(state progressState) {
		renderCount++
	}
	pw.finishLine = func(state progressState) {}

	pw.Write(make([]byte, 5))
	pw.Write(make([]byte, 5))
	pw.lastRenderAt = time.Now().Add(-progressUpdateEvery)
	pw.Write(make([]byte, 5))

	if renderCount != 2 {
		t.Fatalf("expected initial render and interval-based render, got %d", renderCount)
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
