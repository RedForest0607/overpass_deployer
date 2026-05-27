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
			packageDetectCommandForTest(packageManagerDNF): {output: ""},
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
			packageDetectCommandForTest(packageManagerDNF): {output: ""},
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

func TestPackagesForBootstrapSkipsDisabledJDK(t *testing.T) {
	t.Helper()

	disabled := false
	headless := false
	packages, err := packagesForBootstrap(config.BootstrapConfig{
		Packages: []string{"postgresql17"},
		JDK: config.JDKConfig{
			Enabled:  &disabled,
			Vendor:   "corretto",
			Major:    25,
			Headless: &headless,
		},
	})
	if err != nil {
		t.Fatalf("expected disabled jdk to skip package resolution, got %v", err)
	}
	if got, want := strings.Join(packages, ","), "postgresql17"; got != want {
		t.Fatalf("unexpected packages: got %q want %q", got, want)
	}
}

func TestBootstrapHostDetectsPackageManagerWithNoisyOutput(t *testing.T) {
	t.Helper()

	runner := &bootstrapRunner{
		runResults: map[string]commandResult{
			packageDetectCommandForTest(packageManagerDNF): {output: "remote warning before command\n"},
			"rpm -q 'git'": {err: fmt.Errorf("package git is not installed")},
		},
	}

	err := BootstrapHost(runner, config.BootstrapConfig{
		Packages: []string{"git"},
	}, RunOptions{}, "app-01")
	if err != nil {
		t.Fatalf("expected noisy package manager detection to succeed, got %v", err)
	}

	if len(runner.sudoCommands) != 1 || runner.sudoCommands[0] != "dnf install -y 'git'" {
		t.Fatalf("unexpected install command: %v", runner.sudoCommands)
	}
}

func TestBootstrapHostReturnsErrorWhenDNFIsUnavailable(t *testing.T) {
	t.Helper()

	runner := &bootstrapRunner{
		runResults: map[string]commandResult{
			packageDetectCommandForTest(packageManagerDNF): {err: fmt.Errorf("dnf missing")},
			packageDetectCommandForTest(packageManagerYUM): {err: fmt.Errorf("yum missing")},
			packageDetectCommandForTest("apt"):             {output: ""},
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
			packageDetectCommandForTest(packageManagerDNF): {err: fmt.Errorf("dnf missing")},
			packageDetectCommandForTest(packageManagerYUM): {output: ""},
			"rpm -q 'nc'": {err: fmt.Errorf("package nc is not installed")},
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
	runner := &bootstrapRunner{
		runResults: map[string]commandResult{
			"swapon --noheadings --show=NAME": {output: ""},
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

	wantPrefixes := []string{"sh -lc ", "sh -lc "}
	if len(runner.sudoCommands) != len(wantPrefixes) {
		t.Fatalf("unexpected sudo command count: got %v", runner.sudoCommands)
	}
	for i, prefix := range wantPrefixes {
		if !strings.HasPrefix(runner.sudoCommands[i], prefix) {
			t.Fatalf("expected sudo command %d to start with %q, got %q", i, prefix, runner.sudoCommands[i])
		}
	}
	if !strings.Contains(runner.sudoCommands[1], "/swapfile none swap sw 0 0") {
		t.Fatalf("expected fstab command to include swap line, got %q", runner.sudoCommands[1])
	}
}

func TestBuildSwapEnsureCommandCreatesInitializesAndEnablesSwap(t *testing.T) {
	command := buildSwapEnsureCommand(config.SwapConfig{Path: "/swapfile", Size: "4G"})

	for _, fragment := range []string{
		`if [ ! -f "${swap_path}" ]; then fallocate -l "${swap_size}" "${swap_path}"; fi`,
		`chmod 600 "${swap_path}"`,
		`file -s "${swap_path}" | grep -qi`,
		`mkswap "${swap_path}"`,
		`swapon "${swap_path}" || swapon --noheadings --show=NAME | grep -Fxq "${swap_path}"`,
	} {
		if !strings.Contains(command, fragment) {
			t.Fatalf("expected swap ensure command to contain %q, got %q", fragment, command)
		}
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

func TestBootstrapHostTreatsSwaponAlreadyActiveAsSuccess(t *testing.T) {
	t.Helper()

	swapPath := "/swapfile"
	runner := &bootstrapRunner{
		runResults: map[string]commandResult{
			"swapon --noheadings --show=NAME": {output: "", err: nil},
		},
		sudoResults: map[string]commandResult{
			buildSwapEnsureCommand(config.SwapConfig{Path: swapPath, Size: "4G"}): {err: fmt.Errorf("already active")},
		},
	}
	runner.runSequence = map[string][]commandResult{
		"swapon --noheadings --show=NAME": {
			{output: ""},
			{output: "/swapfile\n"},
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
		t.Fatalf("expected already-active swapon to be treated as success, got %v", err)
	}
}

func TestBootstrapHostEnablesAndStartsServices(t *testing.T) {
	t.Helper()

	runner := &bootstrapRunner{
		runResults: map[string]commandResult{
			"systemctl is-enabled 'nginx'": {output: "disabled\n", err: fmt.Errorf("disabled")},
			"systemctl is-active 'nginx'":  {output: "inactive\n", err: fmt.Errorf("inactive")},
		},
	}

	err := BootstrapHost(runner, config.BootstrapConfig{
		Services: []config.ServiceConfig{{Name: "nginx"}},
	}, RunOptions{}, "web-01")
	if err != nil {
		t.Fatalf("expected service bootstrap to succeed, got %v", err)
	}

	want := []string{"systemctl enable 'nginx'", "systemctl start 'nginx'"}
	if strings.Join(runner.sudoCommands, ",") != strings.Join(want, ",") {
		t.Fatalf("unexpected service commands: got %v want %v", runner.sudoCommands, want)
	}
}

func TestBootstrapHostInstallsDockerComposeWhenMissing(t *testing.T) {
	t.Helper()

	runner := &bootstrapRunner{
		runResults: map[string]commandResult{
			"test -x '/usr/local/lib/docker/cli-plugins/docker-compose' && docker compose version --short": {
				err: fmt.Errorf("missing"),
			},
			"docker compose version": {output: "Docker Compose version v2.29.7\n"},
		},
	}

	err := BootstrapHost(runner, config.BootstrapConfig{
		DockerCompose: config.DockerComposeConfig{
			Enabled:     testBoolPtr(true),
			Version:     "v2.29.7",
			SourceURL:   "https://github.com/docker/compose/releases/download/v2.29.7/docker-compose-linux-x86_64",
			InstallPath: "/usr/local/lib/docker/cli-plugins/docker-compose",
		},
	}, RunOptions{}, "build-01")
	if err != nil {
		t.Fatalf("expected docker compose bootstrap to succeed, got %v", err)
	}

	if len(runner.sudoCommands) != 1 {
		t.Fatalf("expected one sudo command, got %v", runner.sudoCommands)
	}
	command := runner.sudoCommands[0]
	for _, expected := range []string{
		"mkdir -p",
		"curl -fL",
		"https://github.com/docker/compose/releases/download/v2.29.7/docker-compose-linux-x86_64",
		"/usr/local/lib/docker/cli-plugins/docker-compose",
		"chmod 0755",
	} {
		if !strings.Contains(command, expected) {
			t.Fatalf("expected install command to contain %q, got %q", expected, command)
		}
	}
}

func TestBootstrapHostSkipsExistingDockerComposeVersion(t *testing.T) {
	t.Helper()

	runner := &bootstrapRunner{
		runResults: map[string]commandResult{
			"test -x '/usr/local/lib/docker/cli-plugins/docker-compose' && docker compose version --short": {
				output: "2.29.7\n",
			},
		},
	}

	err := BootstrapHost(runner, config.BootstrapConfig{
		DockerCompose: config.DockerComposeConfig{
			Enabled:     testBoolPtr(true),
			Version:     "v2.29.7",
			SourceURL:   "https://github.com/docker/compose/releases/download/v2.29.7/docker-compose-linux-x86_64",
			InstallPath: "/usr/local/lib/docker/cli-plugins/docker-compose",
		},
	}, RunOptions{}, "build-01")
	if err != nil {
		t.Fatalf("expected existing docker compose to skip, got %v", err)
	}
	if len(runner.sudoCommands) != 0 {
		t.Fatalf("expected no sudo commands, got %v", runner.sudoCommands)
	}
}

func TestBootstrapHostSkipsActiveEnabledService(t *testing.T) {
	t.Helper()

	runner := &bootstrapRunner{
		runResults: map[string]commandResult{
			"systemctl is-enabled 'nginx'": {output: "enabled\n"},
			"systemctl is-active 'nginx'":  {output: "active\n"},
		},
	}

	err := BootstrapHost(runner, config.BootstrapConfig{
		Services: []config.ServiceConfig{{Name: "nginx", State: config.ServiceStateStarted}},
	}, RunOptions{}, "web-01")
	if err != nil {
		t.Fatalf("expected service bootstrap to succeed, got %v", err)
	}
	if len(runner.sudoCommands) != 0 {
		t.Fatalf("expected no service commands, got %v", runner.sudoCommands)
	}
}

func TestBootstrapHostStopsServiceWhenDisabledWithoutState(t *testing.T) {
	t.Helper()

	disabled := false
	runner := &bootstrapRunner{
		runResults: map[string]commandResult{
			"systemctl is-enabled 'nginx'": {output: "enabled\n"},
			"systemctl is-active 'nginx'":  {output: "active\n"},
		},
	}

	err := BootstrapHost(runner, config.BootstrapConfig{
		Services: []config.ServiceConfig{{Name: "nginx", Enabled: &disabled}},
	}, RunOptions{}, "web-01")
	if err != nil {
		t.Fatalf("expected service bootstrap to succeed, got %v", err)
	}

	want := []string{"systemctl disable 'nginx'", "systemctl stop 'nginx'"}
	if strings.Join(runner.sudoCommands, ",") != strings.Join(want, ",") {
		t.Fatalf("unexpected service commands: got %v want %v", runner.sudoCommands, want)
	}
}

type bootstrapRunner struct {
	runResults   map[string]commandResult
	runSequence  map[string][]commandResult
	sudoResults  map[string]commandResult
	commands     []string
	sudoCommands []string
}

type commandResult struct {
	output string
	err    error
}

func (b *bootstrapRunner) Run(cmd string) (string, error) {
	b.commands = append(b.commands, cmd)
	if sequence := b.runSequence[cmd]; len(sequence) > 0 {
		result := sequence[0]
		b.runSequence[cmd] = sequence[1:]
		return result.output, result.err
	}
	if result, ok := b.runResults[cmd]; ok {
		return result.output, result.err
	}
	return "", fmt.Errorf("unexpected command: %s", cmd)
}

func (b *bootstrapRunner) RunSudo(cmd string) (string, error) {
	b.sudoCommands = append(b.sudoCommands, cmd)
	if result, ok := b.sudoResults[cmd]; ok {
		return result.output, result.err
	}
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

func packageDetectCommandForTest(manager string) string {
	switch manager {
	case packageManagerDNF:
		return "test -x /usr/bin/dnf || test -x /bin/dnf"
	case packageManagerYUM:
		return "test -x /usr/bin/yum || test -x /bin/yum"
	case "apt":
		return "test -x /usr/bin/apt-get || test -x /bin/apt-get"
	default:
		return ""
	}
}
