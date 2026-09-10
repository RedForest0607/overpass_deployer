package scp

import (
	"bytes"
	"errors"
	"io"
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
		output: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef  /remote/app.jar\n",
	}

	hash, err := calculateRemoteSHA256(runner, "/remote/app.jar")
	if err != nil {
		t.Fatalf("expected checksum parsing to succeed, got %v", err)
	}
	if hash != "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" {
		t.Fatalf("expected parsed hash to match sha256 output, got %q", hash)
	}
}

func TestCalculateRemoteSHA256IgnoresNoiseBeforeHash(t *testing.T) {
	t.Helper()

	runner := &stubRunner{
		output: "warning: pseudo-terminal will not be allocated because stdin is not a terminal\nabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd  /remote/app.jar\n",
	}

	hash, err := calculateRemoteSHA256(runner, "/remote/app.jar")
	if err != nil {
		t.Fatalf("expected noisy checksum parsing to succeed, got %v", err)
	}
	if hash != "abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd" {
		t.Fatalf("expected parser to ignore leading noise, got %q", hash)
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

func TestUploadFileAtomicallyReplacesTargetOnlyAfterSuccessfulClose(t *testing.T) {
	fs := newFakeRemoteFileSystem(map[string]string{"/remote/app.jar": "old"})

	if err := uploadFileAtomically(fs, strings.NewReader("new"), "/remote/app.jar", io.Discard); err != nil {
		t.Fatalf("uploadFileAtomically() returned error: %v", err)
	}
	if got := fs.contents("/remote/app.jar"); got != "new" {
		t.Fatalf("remote target = %q, want replacement contents", got)
	}
	if fs.temporaryFileCount() != 0 {
		t.Fatalf("temporary remote file was not removed: %#v", fs.files)
	}
}

func TestUploadFileAtomicallyKeepsTargetWhenWriteFails(t *testing.T) {
	fs := newFakeRemoteFileSystem(map[string]string{"/remote/app.jar": "old"})
	fs.writeErr = errors.New("connection reset")

	err := uploadFileAtomically(fs, strings.NewReader("new"), "/remote/app.jar", io.Discard)
	if err == nil || !strings.Contains(err.Error(), "connection reset") {
		t.Fatalf("expected write failure, got %v", err)
	}
	if got := fs.contents("/remote/app.jar"); got != "old" {
		t.Fatalf("remote target changed after failed upload: %q", got)
	}
	if fs.temporaryFileCount() != 0 {
		t.Fatalf("temporary remote file was not removed: %#v", fs.files)
	}
}

func TestUploadFileAtomicallyKeepsTargetWhenCloseFails(t *testing.T) {
	fs := newFakeRemoteFileSystem(map[string]string{"/remote/app.jar": "old"})
	fs.closeErr = errors.New("remote flush failed")

	err := uploadFileAtomically(fs, strings.NewReader("new"), "/remote/app.jar", io.Discard)
	if err == nil || !strings.Contains(err.Error(), "remote flush failed") {
		t.Fatalf("expected close failure, got %v", err)
	}
	if got := fs.contents("/remote/app.jar"); got != "old" {
		t.Fatalf("remote target changed after failed close: %q", got)
	}
	if fs.temporaryFileCount() != 0 {
		t.Fatalf("temporary remote file was not removed: %#v", fs.files)
	}
}

type fakeRemoteFileSystem struct {
	files    map[string][]byte
	writeErr error
	closeErr error
}

func newFakeRemoteFileSystem(files map[string]string) *fakeRemoteFileSystem {
	converted := make(map[string][]byte, len(files))
	for path, contents := range files {
		converted[path] = []byte(contents)
	}
	return &fakeRemoteFileSystem{files: converted}
}

func (s *fakeRemoteFileSystem) MkdirAll(string) error { return nil }

func (s *fakeRemoteFileSystem) OpenFile(path string, flags int) (remoteWriteFile, error) {
	if _, exists := s.files[path]; exists && flags&os.O_EXCL != 0 {
		return nil, os.ErrExist
	}
	file := &fakeRemoteWriteFile{fileSystem: s, path: path}
	s.files[path] = nil
	return file, nil
}

func (s *fakeRemoteFileSystem) Rename(oldPath, newPath string) error {
	contents, exists := s.files[oldPath]
	if !exists {
		return os.ErrNotExist
	}
	s.files[newPath] = contents
	delete(s.files, oldPath)
	return nil
}

func (s *fakeRemoteFileSystem) Remove(path string) error {
	delete(s.files, path)
	return nil
}

func (s *fakeRemoteFileSystem) contents(path string) string {
	return string(s.files[path])
}

func (s *fakeRemoteFileSystem) temporaryFileCount() int {
	count := 0
	for path := range s.files {
		if strings.Contains(path, ".deploy-upload-") {
			count++
		}
	}
	return count
}

type fakeRemoteWriteFile struct {
	fileSystem *fakeRemoteFileSystem
	path       string
	buffer     bytes.Buffer
}

func (f *fakeRemoteWriteFile) Write(p []byte) (int, error) {
	if f.fileSystem.writeErr != nil {
		return 0, f.fileSystem.writeErr
	}
	return f.buffer.Write(p)
}

func (f *fakeRemoteWriteFile) Close() error {
	if f.fileSystem.closeErr != nil {
		return f.fileSystem.closeErr
	}
	f.fileSystem.files[f.path] = append([]byte(nil), f.buffer.Bytes()...)
	return nil
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
