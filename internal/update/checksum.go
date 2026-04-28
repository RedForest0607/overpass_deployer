package update

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io"
	"strings"
)

// parseChecksums는 checksums.txt 형식의 줄을 파일명별 SHA256 맵으로 변환한다.
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

// verifySHA256은 스트림의 실제 SHA256과 기대 체크섬을 대소문자 무시하고 비교한다.
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
