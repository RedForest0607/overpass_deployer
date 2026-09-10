package vm

import (
	"fmt"
	"strconv"
	"strings"

	"go-deployer/internal/ssh"
)

// ensureRemoteFileMode observes permission changes independently of content
// checksums. Scripts retain existing permission bits while becoming executable.
func ensureRemoteFileMode(runner ssh.Runner, path, mode string, outcomes *operationOutcomes) error {
	output, err := runner.Run("stat -c '%a' " + ssh.ShellQuote(path))
	if err != nil {
		return fmt.Errorf("reading file permissions: %w", err)
	}
	current, err := strconv.ParseUint(strings.TrimSpace(output), 8, 32)
	if err != nil {
		return fmt.Errorf("invalid remote file permissions: %w", err)
	}
	desired := current | 0111
	if mode != "+x" {
		desired, err = strconv.ParseUint(mode, 8, 32)
		if err != nil {
			return fmt.Errorf("invalid requested file permissions: %w", err)
		}
	}
	if current == desired {
		outcomes.skippedOperation()
		return nil
	}
	if err := applyRemoteFileMode(runner, path, fmt.Sprintf("%o", desired)); err != nil {
		return err
	}
	outcomes.changedOperation()
	return nil
}
