package vm

import (
	"strings"
	"testing"

	"go-deployer/internal/config"
)

func TestBuildPrivilegedDirectorySetupCommandWrapsEntireCommandInSudoShell(t *testing.T) {
	command := buildPrivilegedDirectorySetupCommand("/opt/auth-service")

	for _, fragment := range []string{
		"sh -lc",
		`owner_name="${SUDO_USER:-$(id -un)}"`,
		`owner_group="$(id -gn "${owner_name}")"`,
		`if [ ! -d "${base_dir}" ]; then mkdir -p "${base_dir}" && chown "${owner_name}:${owner_group}" "${base_dir}"; fi`,
	} {
		if !strings.Contains(command, fragment) {
			t.Fatalf("expected command to contain %q, got %q", fragment, command)
		}
	}
}

func TestBuildPrivilegedDirectorySetupCommandDoesNotTouchExistingDirectory(t *testing.T) {
	command := buildPrivilegedDirectorySetupCommand("/opt/auth-service")

	if strings.Contains(command, "chown -R") {
		t.Fatalf("expected command not to recursively chown existing directory, got %q", command)
	}
	if !strings.Contains(command, `[ ! -d "${base_dir}" ]`) {
		t.Fatalf("expected command to guard creation when directory exists, got %q", command)
	}
}

func TestBuildPrivilegedDirectorySetupCommandAppliesConfiguredOwnershipRule(t *testing.T) {
	command := buildPrivilegedDirectorySetupCommandWithRules("/app/overpass/sample", []config.DirectoryOwnershipRule{
		{Prefix: "/app", Owner: "ec2-user", Group: "ec2-user"},
	})

	for _, fragment := range []string{
		`owner_spec=`,
		`ec2-user:ec2-user`,
		`mkdir -p "${base_dir}"`,
		`/app`,
		`/app/overpass`,
		`/app/overpass/sample`,
		`chown "${owner_spec}" "${managed_dir}"`,
	} {
		if !strings.Contains(command, fragment) {
			t.Fatalf("expected command to contain %q, got %q", fragment, command)
		}
	}
	if strings.Contains(command, "chown -R") {
		t.Fatalf("expected command not to recursively chown /app directories, got %q", command)
	}
}

func TestManagedDirectoriesForRuleIncludesIntermediateDirectories(t *testing.T) {
	got := managedDirectoriesForRule("/app", "/app/software/hamonica2-agent/lib")
	want := []string{
		"/app",
		"/app/software",
		"/app/software/hamonica2-agent",
		"/app/software/hamonica2-agent/lib",
	}

	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("unexpected managed dirs: got %v want %v", got, want)
	}
}

func TestSelectDirectoryOwnershipRuleUsesLongestMatchingPrefix(t *testing.T) {
	got, ok := selectDirectoryOwnershipRule("/app/elasticsearch/data", []config.DirectoryOwnershipRule{
		{Prefix: "/app", Owner: "ec2-user", Group: "ec2-user"},
		{Prefix: "/app/elasticsearch", Owner: "elasticsearch", Group: "elasticsearch"},
	})
	if !ok {
		t.Fatal("expected ownership rule to match")
	}
	if got.Owner != "elasticsearch" || got.Group != "elasticsearch" {
		t.Fatalf("expected longest matching rule, got %#v", got)
	}
}

func TestCreateDirectoriesUsesCompactAppLayout(t *testing.T) {
	runner := &directoryRunner{}

	err := CreateDirectories(runner, &config.AppConfig{
		Name:    "sample",
		BaseDir: "/app/overpass/sample",
	}, RunOptions{
		DirectoryRules: []config.DirectoryOwnershipRule{
			{Prefix: "/app", Owner: "ec2-user", Group: "ec2-user"},
		},
	}, "sample-host")
	if err != nil {
		t.Fatalf("expected directory creation to succeed, got %v", err)
	}

	want := []string{
		"mkdir -p '/app/overpass/sample/bin'",
		"mkdir -p '/app/overpass/sample/config'",
		"mkdir -p '/app/overpass/sample/lib'",
		"mkdir -p '/app/overpass/sample/logs'",
	}
	if strings.Join(runner.commands, "\n") != strings.Join(want, "\n") {
		t.Fatalf("unexpected directories: got %v want %v", runner.commands, want)
	}
	if len(runner.sudoCommands) != 1 ||
		!strings.Contains(runner.sudoCommands[0], `owner_spec=`) ||
		!strings.Contains(runner.sudoCommands[0], `ec2-user:ec2-user`) {
		t.Fatalf("expected configured /app setup command to use ec2-user ownership, got %v", runner.sudoCommands)
	}
}

type directoryRunner struct {
	commands     []string
	sudoCommands []string
}

func (r *directoryRunner) Run(cmd string) (string, error) {
	r.commands = append(r.commands, cmd)
	return "", nil
}

func (r *directoryRunner) RunSudo(cmd string) (string, error) {
	r.sudoCommands = append(r.sudoCommands, cmd)
	return "", nil
}

func (r *directoryRunner) Host() string {
	return "sample-host"
}

func (r *directoryRunner) Close() error {
	return nil
}
