package vm

import (
	"strings"
	"testing"

	"go-deployer/internal/config"
)

func TestRenderBastionSSHConfigBlockSortsAndBuildsAliases(t *testing.T) {
	servers := []config.ServerConfig{
		{Name: "zeta", Host: "10.0.0.12"},
		{Name: "alpha", Host: "10.0.0.11"},
	}

	block := renderBastionSSHConfigBlock(servers, "ec2-user", 22)

	if !strings.Contains(block, "Host alpha\n  HostName 10.0.0.11\n  User ec2-user\n  Port 22") {
		t.Fatalf("expected block to contain alpha alias, got %q", block)
	}
	if strings.Index(block, "Host alpha") > strings.Index(block, "Host zeta") {
		t.Fatalf("expected aliases to be sorted by name, got %q", block)
	}
}

func TestBuildKnownHostRegistrationCommandTargetsExpectedPath(t *testing.T) {
	command := buildKnownHostRegistrationCommand("10.0.0.10", 22, "~/.ssh/known_hosts")

	for _, fragment := range []string{
		"ssh-keygen -R '10.0.0.10' -f $HOME/'.ssh/known_hosts'",
		"ssh-keyscan -H -p 22 '10.0.0.10' >> $HOME/'.ssh/known_hosts'",
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
		"touch $HOME/'.ssh/config'",
	} {
		if !strings.Contains(command, fragment) {
			t.Fatalf("expected command to contain %q, got %q", fragment, command)
		}
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
