package service

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestSafeGenerationFailureDoesNotExposeInternalDetails(t *testing.T) {
	secret := "/private/data/customer-token-secret"
	failure := safeGenerationFailure(errors.New(secret))
	if failure.Code != "GENERATION_FAILED" || strings.Contains(failure.Message, secret) {
		t.Fatalf("unsafe failure: %+v", failure)
	}

	timedOut := safeGenerationFailure(context.DeadlineExceeded)
	if timedOut.Code != "GENERATION_TIMEOUT" {
		t.Fatalf("timeout failure: %+v", timedOut)
	}

	invalid := safeGenerationFailure(&InvalidArtifact{Err: errors.New(secret)})
	if invalid.Code != "INVALID_ARTIFACT" || strings.Contains(invalid.Message, secret) {
		t.Fatalf("unsafe artifact failure: %+v", invalid)
	}
}
