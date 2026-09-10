package vm

import "testing"

func TestOperationOutcomesMarksServerSkippedWhenEveryObservedOperationIsSkipped(t *testing.T) {
	outcomes := newOperationOutcomes(false)
	outcomes.skippedOperation()
	outcomes.TransferSkipped("/app/sample/lib/sample.jar")

	if got := outcomes.status(); got != serverResultSkipped {
		t.Fatalf("expected skipped result, got %q", got)
	}
}

func TestOperationOutcomesMarksServerDoneWhenAnyOperationChanges(t *testing.T) {
	outcomes := newOperationOutcomes(false)
	outcomes.TransferSkipped("/app/sample/lib/sample.jar")
	outcomes.TransferChanged("/app/sample/bin/server.sh")

	if got := outcomes.status(); got != serverResultDone {
		t.Fatalf("expected done result, got %q", got)
	}
}

func TestOperationOutcomesKeepsDryRunAsDone(t *testing.T) {
	outcomes := newOperationOutcomes(true)
	outcomes.TransferSkipped("/app/sample/lib/sample.jar")

	if got := outcomes.status(); got != serverResultDone {
		t.Fatalf("expected dry-run result to stay done, got %q", got)
	}
}
