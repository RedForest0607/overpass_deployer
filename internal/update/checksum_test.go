package update

import (
	"strings"
	"testing"
)

func TestParseChecksums(t *testing.T) {
	t.Parallel()

	content := "abc123  deploy_v1.2.3_linux_amd64.tar.gz\n" +
		"def456 *checksums.txt\n"

	got, err := parseChecksums(strings.NewReader(content))
	if err != nil {
		t.Fatalf("expected checksum parsing to succeed, got %v", err)
	}

	if got["deploy_v1.2.3_linux_amd64.tar.gz"] != "abc123" {
		t.Fatalf("expected archive checksum to be parsed, got %#v", got)
	}
	if got["checksums.txt"] != "def456" {
		t.Fatalf("expected checksums.txt entry to be parsed, got %#v", got)
	}
}

func TestSameVersionIgnoresLeadingV(t *testing.T) {
	t.Parallel()

	if !sameVersion("v1.2.3", "1.2.3") {
		t.Fatalf("expected versions with and without v prefix to match")
	}
}
