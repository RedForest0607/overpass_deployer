package vm

import (
	"fmt"
	"strings"

	"go-deployer/internal/config"
	"go-deployer/internal/ssh"
)

func collectRemoteFilePaths(server config.ServerConfig) []string {
	seen := make(map[string]struct{})
	var paths []string

	add := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		if _, ok := seen[path]; ok {
			return
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}

	for _, ef := range server.ExtraFiles {
		add(ef.RemotePath)
	}
	for _, app := range server.EffectiveApps() {
		add(app.Jar.RemotePath)
		for _, cf := range app.ConfigFiles {
			cf.Normalize()
			add(cf.RemotePath)
		}
		for _, ef := range app.ExtraFiles {
			add(ef.RemotePath)
		}
		app.Script.Normalize(app.BaseDir)
		add(app.Script.RemotePath)
	}

	return paths
}

func fetchRemoteSHA256Batch(runner ssh.Runner, remotePaths []string) (map[string]string, error) {
	hashes := make(map[string]string, len(remotePaths))
	if len(remotePaths) == 0 {
		return hashes, nil
	}

	quotedPaths := make([]string, 0, len(remotePaths))
	for _, path := range remotePaths {
		hashes[path] = ""
		quotedPaths = append(quotedPaths, ssh.ShellQuote(path))
	}

	innerCommand := "sha256sum " + strings.Join(quotedPaths, " ") + " 2>/dev/null || true"
	out, err := runner.Run("sh -lc " + ssh.ShellQuote(innerCommand))
	if err != nil {
		return nil, fmt.Errorf("batch remote sha256: %w", err)
	}

	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || !isSHA256(fields[0]) {
			continue
		}
		path := strings.Join(fields[1:], " ")
		if _, ok := hashes[path]; ok {
			hashes[path] = fields[0]
		}
	}

	return hashes, nil
}

func isSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, r := range value {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') {
			continue
		}
		return false
	}
	return true
}
