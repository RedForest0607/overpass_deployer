package vm

import "go-deployer/internal/ssh"

// mutationTrackingRunner counts successful privileged mutations after the
// bootstrap/service helpers have performed their read-only prerequisite checks.
// An ensure command is conservatively considered work even if its internal
// shell checks ultimately leave the resource unchanged.
type mutationTrackingRunner struct {
	ssh.Runner
	outcomes *operationOutcomes
}

func (r mutationTrackingRunner) RunSudo(command string) (string, error) {
	output, err := r.Runner.RunSudo(command)
	if err == nil {
		r.outcomes.changedOperation()
	}
	return output, err
}
