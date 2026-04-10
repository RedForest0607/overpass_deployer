package update

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io"
	"strings"
)

func parseChecksums(r io.Reader) (map[string]string, error) {
	scanner := bufio.NewScanner(r)
	checksums := make(map[string]string)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			return nil, fmt.Errorf("invalid checksum line %q", line)
		}

		filename := strings.TrimPrefix(fields[len(fields)-1], "*")
		checksums[filename] = fields[0]
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading checksum file: %w", err)
	}

	return checksums, nil
}

func verifySHA256(expected string, r io.Reader) error {
	hash := sha256.New()
	if _, err := io.Copy(hash, r); err != nil {
		return fmt.Errorf("calculating sha256: %w", err)
	}

	actual := fmt.Sprintf("%x", hash.Sum(nil))
	if !strings.EqualFold(actual, strings.TrimSpace(expected)) {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expected, actual)
	}

	return nil
}
