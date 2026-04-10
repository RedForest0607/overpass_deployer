package update

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func extractBinaryFromArchive(archivePath, binaryName string) (string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return "", fmt.Errorf("opening archive %s: %w", archivePath, err)
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return "", fmt.Errorf("opening gzip archive %s: %w", archivePath, err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	tempDir, err := os.MkdirTemp("", "deploy-update-archive-*")
	if err != nil {
		return "", fmt.Errorf("creating temp extraction directory: %w", err)
	}

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			os.RemoveAll(tempDir)
			return "", fmt.Errorf("reading archive contents: %w", err)
		}

		if header.Typeflag != tar.TypeReg {
			continue
		}

		if filepath.Base(header.Name) != binaryName {
			continue
		}

		extractedPath := filepath.Join(tempDir, binaryName)
		outFile, err := os.OpenFile(extractedPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			os.RemoveAll(tempDir)
			return "", fmt.Errorf("creating extracted binary %s: %w", extractedPath, err)
		}

		if _, err := io.Copy(outFile, tarReader); err != nil {
			outFile.Close()
			os.RemoveAll(tempDir)
			return "", fmt.Errorf("writing extracted binary %s: %w", extractedPath, err)
		}

		if err := outFile.Close(); err != nil {
			os.RemoveAll(tempDir)
			return "", fmt.Errorf("closing extracted binary %s: %w", extractedPath, err)
		}

		return extractedPath, nil
	}

	os.RemoveAll(tempDir)
	return "", fmt.Errorf("binary %q not found in archive", binaryName)
}
