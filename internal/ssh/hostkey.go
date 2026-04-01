package ssh

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"

	"go-deployer/internal/config"

	sshlib "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

var knownHostsMu sync.Mutex

func NewHostKeyCallback(mode, knownHostsPath string) (sshlib.HostKeyCallback, error) {
	switch mode {
	case config.HostKeyStrict:
		return knownhosts.New(knownHostsPath)
	case config.HostKeyAcceptNew:
		return newAcceptNewHostKeyCallback(knownHostsPath)
	case config.HostKeyInsecure:
		return sshlib.InsecureIgnoreHostKey(), nil
	default:
		return nil, fmt.Errorf("unsupported host key checking mode: %s", mode)
	}
}

func newAcceptNewHostKeyCallback(knownHostsPath string) (sshlib.HostKeyCallback, error) {
	if err := ensureKnownHostsFile(knownHostsPath); err != nil {
		return nil, err
	}

	strictCallback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		return nil, fmt.Errorf("loading known_hosts: %w", err)
	}

	return func(hostname string, remote net.Addr, key sshlib.PublicKey) error {
		err := strictCallback(hostname, remote, key)
		if err == nil {
			return nil
		}

		var keyErr *knownhosts.KeyError
		if !errors.As(err, &keyErr) {
			return err
		}
		if len(keyErr.Want) > 0 {
			return err
		}

		if err := appendKnownHost(knownHostsPath, hostname, remote.String(), key); err != nil {
			return err
		}
		return nil
	}, nil
}

func ensureKnownHostsFile(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating known_hosts directory: %w", err)
	}

	file, err := os.OpenFile(path, os.O_RDONLY|os.O_CREATE, 0o600)
	if err != nil {
		return fmt.Errorf("creating known_hosts file: %w", err)
	}
	return file.Close()
}

func appendKnownHost(path, hostname, remoteAddress string, key sshlib.PublicKey) error {
	knownHostsMu.Lock()
	defer knownHostsMu.Unlock()

	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		return fmt.Errorf("opening known_hosts for append: %w", err)
	}
	defer file.Close()

	line := knownhosts.Line(hostKeyAddresses(hostname, remoteAddress), key) + "\n"
	if _, err := file.WriteString(line); err != nil {
		return fmt.Errorf("writing known_hosts entry: %w", err)
	}
	return nil
}

func hostKeyAddresses(hostname, remoteAddress string) []string {
	trimmed := make([]string, 0, 2)
	seen := make(map[string]struct{}, 2)

	for _, candidate := range []string{hostname, remoteAddress} {
		if candidate == "" {
			continue
		}
		normalized := knownhosts.Normalize(candidate)
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		trimmed = append(trimmed, candidate)
	}

	return trimmed
}
