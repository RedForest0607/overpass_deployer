package update

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// replaceExecutable은 기존 실행 파일 권한을 보존하면서 새 바이너리로 원자적 교체를 시도한다.
func replaceExecutable(targetPath, extractedBinaryPath string) error {
	targetInfo, err := os.Stat(targetPath)
	if err != nil {
		return fmt.Errorf("stat current executable %s: %w", targetPath, err)
	}

	targetDir := filepath.Dir(targetPath)
	tempFile, err := os.CreateTemp(targetDir, "deploy-update-*")
	if err != nil {
		return fmt.Errorf("creating temp executable in %s: %w", targetDir, err)
	}

	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	sourceFile, err := os.Open(extractedBinaryPath)
	if err != nil {
		tempFile.Close()
		return fmt.Errorf("opening extracted binary %s: %w", extractedBinaryPath, err)
	}

	if _, err := io.Copy(tempFile, sourceFile); err != nil {
		sourceFile.Close()
		tempFile.Close()
		return fmt.Errorf("copying new executable into %s: %w", tempPath, err)
	}

	if err := sourceFile.Close(); err != nil {
		tempFile.Close()
		return fmt.Errorf("closing extracted binary %s: %w", extractedBinaryPath, err)
	}

	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("closing temp executable %s: %w", tempPath, err)
	}

	if err := os.Chmod(tempPath, targetInfo.Mode()); err != nil {
		return fmt.Errorf("setting permissions on %s: %w", tempPath, err)
	}

	if err := os.Rename(tempPath, targetPath); err != nil {
		return fmt.Errorf("replacing executable %s: %w", targetPath, err)
	}

	return nil
}
