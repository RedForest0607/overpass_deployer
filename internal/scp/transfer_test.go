package scp

import (
	"os"
	"path/filepath"
	"testing"
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
