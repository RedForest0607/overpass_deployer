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
		`mkdir -p "${base_dir}"`,
		`chown -R "${owner_name}:${owner_group}" "${base_dir}"`,
	} {
		if !strings.Contains(command, fragment) {
			t.Fatalf("expected command to contain %q, got %q", fragment, command)
		}
	}
}

func TestCreateDirectoriesUsesCompactAppLayout(t *testing.T) {
	runner := &directoryRunner{}

	err := CreateDirectories(runner, &config.AppConfig{
		Name:    "sample",
		BaseDir: "/app/overpass/sample",
	}, RunOptions{}, "sample-host")
	if err != nil {
		t.Fatalf("expected directory creation to succeed, got %v", err)
	}

	want := []string{
		"mkdir -p '/app/overpass/sample/bin'",
		"mkdir -p '/app/overpass/sample/config'",
		"mkdir -p '/app/overpass/sample/logs'",
	}
	if strings.Join(runner.commands, "\n") != strings.Join(want, "\n") {
		t.Fatalf("unexpected directories: got %v want %v", runner.commands, want)
	}
}

type directoryRunner struct {
	commands []string
}

func (r *directoryRunner) Run(cmd string) (string, error) {
	r.commands = append(r.commands, cmd)
	return "", nil
}

func (r *directoryRunner) RunSudo(cmd string) (string, error) {
	return "", nil
}

func (r *directoryRunner) Host() string {
	return "sample-host"
}

func (r *directoryRunner) Close() error {
	return nil
}
