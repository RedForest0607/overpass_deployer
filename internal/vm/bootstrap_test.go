package vm

import (
	"fmt"
	"strings"
	"testing"

	"go-deployer/internal/config"
	"go-deployer/internal/ssh"
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
			packageDetectCommandForTest():               {output: "dnf\n"},
			"rpm -q 'git'":                              {output: "git-1.0.0\n"},
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
			packageDetectCommandForTest(): {output: "dnf\n"},
			"rpm -q 'git'":                {err: fmt.Errorf("package git is not installed")},
			"rpm -q 'unzip'":              {output: "unzip-1.0.0\n"},
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
			packageDetectCommandForTest(): {output: "apt\n"},
		},
	}

	err := BootstrapHost(runner, config.BootstrapConfig{
		Packages: []string{"git"},
	}, RunOptions{}, "app-01")
	if err == nil || !strings.Contains(err.Error(), "bootstrap requires yum/dnf-compatible host; apt support is not implemented yet") {
		t.Fatalf("expected unsupported package manager error, got %v", err)
	}
}

func TestBootstrapHostInstallsPackagesViaYumWhenDNFIsUnavailable(t *testing.T) {
	t.Helper()

	runner := &bootstrapRunner{
		runResults: map[string]commandResult{
			packageDetectCommandForTest(): {output: "yum\n"},
			"rpm -q 'nc'":                 {err: fmt.Errorf("package nc is not installed")},
		},
	}

	err := BootstrapHost(runner, config.BootstrapConfig{
		Packages: []string{"nc"},
	}, RunOptions{}, "app-01")
	if err != nil {
		t.Fatalf("expected yum bootstrap install to succeed, got %v", err)
	}

	if len(runner.sudoCommands) != 1 || runner.sudoCommands[0] != "yum install -y 'nc'" {
		t.Fatalf("unexpected yum install command: %v", runner.sudoCommands)
	}
}

func TestBootstrapHostSetsTimezoneWhenDifferent(t *testing.T) {
	t.Helper()

	runner := &bootstrapRunner{
		runResults: map[string]commandResult{
			"timedatectl show -p Timezone --value": {output: "UTC\n"},
		},
	}

	err := BootstrapHost(runner, config.BootstrapConfig{
		Timezone: config.TimezoneConfig{Name: "Asia/Seoul"},
	}, RunOptions{}, "app-01")
	if err != nil {
		t.Fatalf("expected timezone bootstrap to succeed, got %v", err)
	}

	want := "timedatectl set-timezone 'Asia/Seoul'"
	if len(runner.sudoCommands) != 1 || runner.sudoCommands[0] != want {
		t.Fatalf("unexpected timezone sudo commands: got %v want [%s]", runner.sudoCommands, want)
	}
}

func TestBootstrapHostCreatesAndPersistsSwapIdempotently(t *testing.T) {
	t.Helper()

	swapPath := "/swapfile"
	fileCheckCommand := "sh -lc " + ssh.ShellQuote("if test -f "+ssh.ShellQuote(swapPath)+"; then echo exists; else echo missing; fi")
	runner := &bootstrapRunner{
		runResults: map[string]commandResult{
			"swapon --noheadings --show=NAME": {output: ""},
			fileCheckCommand:                  {output: "missing\n"},
		},
	}

	err := BootstrapHost(runner, config.BootstrapConfig{
		Swap: config.SwapConfig{
			Enabled: testBoolPtr(true),
			Path:    swapPath,
			Size:    "4G",
		},
	}, RunOptions{}, "app-01")
	if err != nil {
		t.Fatalf("expected swap bootstrap to succeed, got %v", err)
	}

	wantPrefixes := []string{
		"fallocate -l '4G' '/swapfile'",
		"chmod 600 '/swapfile'",
		"mkswap '/swapfile'",
		"swapon '/swapfile'",
		"sh -lc ",
	}
	if len(runner.sudoCommands) != len(wantPrefixes) {
		t.Fatalf("unexpected sudo command count: got %v", runner.sudoCommands)
	}
	for i, prefix := range wantPrefixes {
		if !strings.HasPrefix(runner.sudoCommands[i], prefix) {
			t.Fatalf("expected sudo command %d to start with %q, got %q", i, prefix, runner.sudoCommands[i])
		}
	}
	if !strings.Contains(runner.sudoCommands[4], "/swapfile none swap sw 0 0") {
		t.Fatalf("expected fstab command to include swap line, got %q", runner.sudoCommands[4])
	}
}

func TestBootstrapHostSkipsActiveSwapButEnsuresFstab(t *testing.T) {
	t.Helper()

	runner := &bootstrapRunner{
		runResults: map[string]commandResult{
			"swapon --noheadings --show=NAME": {output: "/swapfile\n"},
		},
	}

	err := BootstrapHost(runner, config.BootstrapConfig{
		Swap: config.SwapConfig{
			Enabled: testBoolPtr(true),
			Path:    "/swapfile",
			Size:    "4G",
		},
	}, RunOptions{}, "app-01")
	if err != nil {
		t.Fatalf("expected active swap bootstrap to succeed, got %v", err)
	}
	if len(runner.sudoCommands) != 1 || !strings.Contains(runner.sudoCommands[0], "/swapfile none swap sw 0 0") {
		t.Fatalf("expected only fstab sudo command, got %v", runner.sudoCommands)
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

func testBoolPtr(value bool) *bool {
	return &value
}

func packageDetectCommandForTest() string {
	return "sh -lc " + ssh.ShellQuote("if command -v dnf >/dev/null 2>&1; then echo dnf; elif command -v yum >/dev/null 2>&1; then echo yum; elif command -v apt-get >/dev/null 2>&1; then echo apt; fi")
}
