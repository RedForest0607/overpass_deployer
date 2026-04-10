package vm

import (
	"fmt"
	"strings"
	"testing"

	"go-deployer/internal/config"
)

func TestBootstrapHostDryRunPlansOSUpdateAndInstall(t *testing.T) {
	t.Helper()

	enabled := true
	headless := true
	bootstrap := config.BootstrapConfig{
		Packages: []string{"git"},
		JDK: config.JDKConfig{
			Vendor:   "corretto",
			Major:    21,
			Headless: &headless,
		},
		OSUpdate: config.OSUpdateConfig{
			Enabled: &enabled,
		},
	}

	if err := BootstrapHost(nil, bootstrap, RunOptions{DryRun: true}, "app-01"); err != nil {
		t.Fatalf("expected dry-run bootstrap to succeed, got %v", err)
	}
}

func TestBootstrapHostSkipsWhenPackagesAlreadyInstalled(t *testing.T) {
	t.Helper()

	headless := true
	runner := &bootstrapRunner{
		runResults: map[string]commandResult{
			"sh -lc 'if command -v dnf >/dev/null 2>&1; then echo dnf; elif command -v apt-get >/dev/null 2>&1; then echo apt; fi'": {output: "dnf\n"},
			"rpm -q 'git'": {output: "git-1.0.0\n"},
			"rpm -q 'java-21-amazon-corretto-headless'": {output: "java-21-amazon-corretto-headless-1.0.0\n"},
		},
	}

	err := BootstrapHost(runner, config.BootstrapConfig{
		Packages: []string{"git"},
		JDK: config.JDKConfig{
			Vendor:   "corretto",
			Major:    21,
			Headless: &headless,
		},
	}, RunOptions{}, "app-01")
	if err != nil {
		t.Fatalf("expected bootstrap skip to succeed, got %v", err)
	}
	if len(runner.sudoCommands) != 0 {
		t.Fatalf("expected no sudo commands when packages are already installed, got %v", runner.sudoCommands)
	}
}

func TestBootstrapHostInstallsOnlyMissingPackages(t *testing.T) {
	t.Helper()

	runner := &bootstrapRunner{
		runResults: map[string]commandResult{
			"sh -lc 'if command -v dnf >/dev/null 2>&1; then echo dnf; elif command -v apt-get >/dev/null 2>&1; then echo apt; fi'": {output: "dnf\n"},
			"rpm -q 'git'":   {err: fmt.Errorf("package git is not installed")},
			"rpm -q 'unzip'": {output: "unzip-1.0.0\n"},
		},
	}

	err := BootstrapHost(runner, config.BootstrapConfig{
		Packages: []string{"git", "unzip"},
	}, RunOptions{}, "app-01")
	if err != nil {
		t.Fatalf("expected bootstrap install to succeed, got %v", err)
	}

	if len(runner.sudoCommands) != 1 {
		t.Fatalf("expected one sudo install command, got %v", runner.sudoCommands)
	}
	if got := runner.sudoCommands[0]; got != "dnf install -y 'git'" {
		t.Fatalf("unexpected install command: %q", got)
	}
}

func TestBootstrapHostReturnsErrorWhenDNFIsUnavailable(t *testing.T) {
	t.Helper()

	runner := &bootstrapRunner{
		runResults: map[string]commandResult{
			"sh -lc 'if command -v dnf >/dev/null 2>&1; then echo dnf; elif command -v apt-get >/dev/null 2>&1; then echo apt; fi'": {output: "apt\n"},
		},
	}

	err := BootstrapHost(runner, config.BootstrapConfig{
		Packages: []string{"git"},
	}, RunOptions{}, "app-01")
	if err == nil || !strings.Contains(err.Error(), "bootstrap requires dnf-compatible host; apt support is not implemented yet") {
		t.Fatalf("expected unsupported package manager error, got %v", err)
	}
}

type bootstrapRunner struct {
	runResults   map[string]commandResult
	commands     []string
	sudoCommands []string
}

type commandResult struct {
	output string
	err    error
}

func (b *bootstrapRunner) Run(cmd string) (string, error) {
	b.commands = append(b.commands, cmd)
	if result, ok := b.runResults[cmd]; ok {
		return result.output, result.err
	}
	return "", fmt.Errorf("unexpected command: %s", cmd)
}

func (b *bootstrapRunner) RunSudo(cmd string) (string, error) {
	b.sudoCommands = append(b.sudoCommands, cmd)
	return "", nil
}

func (b *bootstrapRunner) Host() string {
	return "bootstrap-host"
}

func (b *bootstrapRunner) Close() error {
	return nil
}
