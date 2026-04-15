package vm

import (
	"strings"
	"testing"

	"go-deployer/internal/config"
)

func TestRenderBastionSSHConfigBlockSortsAndBuildsAliases(t *testing.T) {
	servers := []config.ServerConfig{
		{Name: "zeta", Host: "10.0.0.12", SSHPort: 2202, BastionHost: "zeta-vm", BastionSSHPort: 22},
		{Name: "alpha", Host: "10.0.0.11", SSHPort: 2201, BastionHost: "alpha-vm", BastionSSHPort: 22},
	}

	block := renderBastionSSHConfigBlock(servers, "ec2-user", "~/.ssh/overpass.pem", "accept-new", "~/.overpass-smoke/ssh/target_known_hosts")

	if !strings.Contains(block, "Host alpha\n  HostName alpha-vm\n  User ec2-user\n  Port 22\n  IdentityFile ~/.ssh/overpass.pem\n  IdentitiesOnly yes\n  StrictHostKeyChecking accept-new\n  UserKnownHostsFile ~/.overpass-smoke/ssh/target_known_hosts") {
		t.Fatalf("expected block to contain alpha alias, got %q", block)
	}
	if strings.Index(block, "Host alpha") > strings.Index(block, "Host zeta") {
		t.Fatalf("expected aliases to be sorted by name, got %q", block)
	}
}

func TestHostKeyConfigLinesSupportsModes(t *testing.T) {
	tests := []struct {
		name string
		mode string
		want string
	}{
		{name: "accept-new", mode: "accept-new", want: "  StrictHostKeyChecking accept-new\n  UserKnownHostsFile ~/.ssh/known_hosts"},
		{name: "strict", mode: "strict", want: "  StrictHostKeyChecking yes\n  UserKnownHostsFile ~/.ssh/known_hosts"},
		{name: "insecure", mode: "insecure", want: "  StrictHostKeyChecking no\n  UserKnownHostsFile /dev/null"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := strings.Join(hostKeyConfigLines(tc.mode, "~/.ssh/known_hosts"), "\n")
			if got != tc.want {
				t.Fatalf("unexpected host key config lines: got %q want %q", got, tc.want)
			}
		})
	}
}

func TestRenderBastionShellAliasBlockSortsAndBuildsAliases(t *testing.T) {
	servers := []config.ServerConfig{
		{Name: "zeta"},
		{Name: "alpha"},
	}

	block := renderBastionShellAliasBlock(servers, "~/.overpass-smoke/ssh/config")

	if !strings.Contains(block, "alias alpha='ssh -F ~/.overpass-smoke/ssh/config alpha'") {
		t.Fatalf("expected block to contain alpha shell alias, got %q", block)
	}
	if strings.Index(block, "alias alpha=") > strings.Index(block, "alias zeta=") {
		t.Fatalf("expected shell aliases to be sorted by name, got %q", block)
	}
}

func TestBuildKnownHostRegistrationCommandTargetsExpectedPath(t *testing.T) {
	command := buildKnownHostRegistrationCommand("10.0.0.10", 22, "~/.ssh/known_hosts")

	for _, fragment := range []string{
		`touch "$HOME/.ssh/known_hosts"`,
		`ssh-keygen -R '10.0.0.10' -f "$HOME/.ssh/known_hosts"`,
		`ssh-keyscan -H -p 22 '10.0.0.10' >> "$HOME/.ssh/known_hosts"`,
	} {
		if !strings.Contains(command, fragment) {
			t.Fatalf("expected command to contain %q, got %q", fragment, command)
		}
	}
}

func TestUpsertManagedBlockCommandIncludesMarkers(t *testing.T) {
	command := upsertManagedBlockCommand("~/.ssh/config", bastionAliasBlockStart+"\nHost app\n"+bastionAliasBlockEnd)

	for _, fragment := range []string{
		"awk -v start='# BEGIN overpass-deployer managed aliases'",
		"-v end='# END overpass-deployer managed aliases'",
		"printf '%s\\n' '# BEGIN overpass-deployer managed aliases",
		`touch "$HOME/.ssh/config"`,
	} {
		if !strings.Contains(command, fragment) {
			t.Fatalf("expected command to contain %q, got %q", fragment, command)
		}
	}
}

func TestUpsertManagedBlockCommandWithCustomMarkersIncludesMarkers(t *testing.T) {
	command := upsertManagedBlockCommandWithMarkers("~/.bashrc", bastionShellBlockStart+"\nalias app='ssh app'\n"+bastionShellBlockEnd, bastionShellBlockStart, bastionShellBlockEnd)

	for _, fragment := range []string{
		"awk -v start='# BEGIN overpass-deployer managed shell aliases'",
		"-v end='# END overpass-deployer managed shell aliases'",
		"printf '%s\\n' '# BEGIN overpass-deployer managed shell aliases",
		`touch "$HOME/.bashrc"`,
	} {
		if !strings.Contains(command, fragment) {
			t.Fatalf("expected command to contain %q, got %q", fragment, command)
		}
	}
}

func TestUpsertManagedBlockCommandSkipsDirChmodWhenNotOwner(t *testing.T) {
	command := upsertManagedBlockCommand("/tmp/overpass-test-ssh_config", bastionAliasBlockStart+"\nHost app\n"+bastionAliasBlockEnd)

	if !strings.Contains(command, "{ [ ! -O '/tmp' ] || chmod 700 '/tmp'; }") {
		t.Fatalf("expected command to conditionally chmod parent dir, got %q", command)
	}
}

func TestBuildKnownHostRegistrationCommandSkipsDirChmodWhenNotOwner(t *testing.T) {
	command := buildKnownHostRegistrationCommand("billing-vm", 2222, "/tmp/overpass-test-known_hosts")

	if !strings.Contains(command, "{ [ ! -O '/tmp' ] || chmod 700 '/tmp'; }") {
		t.Fatalf("expected command to conditionally chmod parent dir, got %q", command)
	}
}

func TestEnsureServerKnowsBastionDryRunSkipsRunner(t *testing.T) {
	t.Helper()

	runner := &panicRunner{}
	err := ensureServerKnowsBastion(runner, config.BastionConfig{
		Host: "bastion.internal",
		Port: 22,
	}, RunOptions{DryRun: true}, "app-01")
	if err != nil {
		t.Fatalf("expected dry-run bastion registration to succeed, got %v", err)
	}
}

type panicRunner struct{}

func (p *panicRunner) Run(cmd string) (string, error) {
	panic("Run should not be called during dry-run")
}

func (p *panicRunner) RunSudo(cmd string) (string, error) {
	panic("RunSudo should not be called during dry-run")
}

func (p *panicRunner) Host() string {
	return "panic-runner"
}

func (p *panicRunner) Close() error {
	return nil
}
