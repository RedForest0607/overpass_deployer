package vm

import "go-deployer/internal/scp"

// operationOutcomes tracks observed remote work for exactly one server deployment.
// Dry-run plans intentionally remain done because no remote state is observed.
type operationOutcomes struct {
	dryRun  bool
	changed int
	skipped int
}

func newOperationOutcomes(dryRun bool) *operationOutcomes {
	return &operationOutcomes{dryRun: dryRun}
}

func (o *operationOutcomes) changedOperation() {
	if o != nil && !o.dryRun {
		o.changed++
	}
}

func (o *operationOutcomes) skippedOperation() {
	if o != nil && !o.dryRun {
		o.skipped++
	}
}

func (o *operationOutcomes) TransferChanged(string) {
	o.changedOperation()
}

func (o *operationOutcomes) TransferSkipped(string) {
	o.skippedOperation()
}

func (o *operationOutcomes) transferObserver() scp.TransferObserver {
	if o == nil || o.dryRun {
		return nil
	}
	return o
}

func (o *operationOutcomes) status() serverResultStatus {
	if o == nil || o.dryRun || o.changed > 0 || o.skipped == 0 {
		return serverResultDone
	}
	return serverResultSkipped
}
