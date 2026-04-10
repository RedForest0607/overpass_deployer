package update

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestReplaceExecutableReplacesTargetContents(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	targetPath := filepath.Join(dir, "deploy")
	sourcePath := filepath.Join(dir, "new-deploy")

	if err := os.WriteFile(targetPath, []byte("old-binary"), 0o755); err != nil {
		t.Fatalf("writing target executable: %v", err)
	}
	if err := os.WriteFile(sourcePath, []byte("new-binary"), 0o755); err != nil {
		t.Fatalf("writing source executable: %v", err)
	}

	if err := replaceExecutable(targetPath, sourcePath); err != nil {
		t.Fatalf("expected executable replacement to succeed, got %v", err)
	}

	content, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("reading replaced executable: %v", err)
	}
	if string(content) != "new-binary" {
		t.Fatalf("expected replaced executable contents, got %q", string(content))
	}
}

func TestReplaceExecutableFailsWithoutDirectoryWritePermission(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("directory permission semantics differ on windows")
	}

	dir := t.TempDir()
	targetPath := filepath.Join(dir, "deploy")
	sourcePath := filepath.Join(dir, "new-deploy")

	if err := os.WriteFile(targetPath, []byte("old-binary"), 0o755); err != nil {
		t.Fatalf("writing target executable: %v", err)
	}
	if err := os.WriteFile(sourcePath, []byte("new-binary"), 0o755); err != nil {
		t.Fatalf("writing source executable: %v", err)
	}

	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod directory read-only: %v", err)
	}
	defer os.Chmod(dir, 0o755)

	if err := replaceExecutable(targetPath, sourcePath); err == nil {
		t.Fatalf("expected replacement to fail without write permission")
	}
}
