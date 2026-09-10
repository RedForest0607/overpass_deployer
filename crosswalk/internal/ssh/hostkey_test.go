package ssh

import (
	"crypto/rand"
	"crypto/rsa"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-deployer/internal/config"

	sshlib "golang.org/x/crypto/ssh"
)

func TestNewHostKeyCallbackAcceptNewAddsUnknownHost(t *testing.T) {
	key, err := generatePublicKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	knownHostsPath := filepath.Join(t.TempDir(), "nested", "known_hosts")
	callback, err := NewHostKeyCallback(config.HostKeyAcceptNew, knownHostsPath)
	if err != nil {
		t.Fatalf("NewHostKeyCallback failed: %v", err)
	}

	remote := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 22}
	if err := callback("example.com:22", remote, key); err != nil {
		t.Fatalf("accept-new callback failed: %v", err)
	}

	content, err := os.ReadFile(knownHostsPath)
	if err != nil {
		t.Fatalf("read known_hosts: %v", err)
	}
	if !strings.Contains(string(content), "example.com") {
		t.Fatalf("expected known_hosts to contain hostname, got %q", string(content))
	}
	if !strings.Contains(string(content), "127.0.0.1") {
		t.Fatalf("expected known_hosts to contain remote address, got %q", string(content))
	}
}

func TestNewHostKeyCallbackStrictRequiresExistingFile(t *testing.T) {
	_, err := NewHostKeyCallback(config.HostKeyStrict, filepath.Join(t.TempDir(), "missing", "known_hosts"))
	if err == nil {
		t.Fatal("expected strict callback creation to fail for missing known_hosts")
	}
}

func TestHostKeyAddressesDeduplicatesEntries(t *testing.T) {
	actual := hostKeyAddresses("example.com:22", "example.com")
	expected := []string{"example.com:22"}

	if len(actual) != len(expected) || actual[0] != expected[0] {
		t.Fatalf("unexpected host key addresses: got %#v want %#v", actual, expected)
	}
}

func generatePublicKey() (sshlib.PublicKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	signer, err := sshlib.NewSignerFromKey(privateKey)
	if err != nil {
		return nil, err
	}
	return signer.PublicKey(), nil
}
