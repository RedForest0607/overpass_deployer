package vm

import (
	"strings"
	"testing"
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
