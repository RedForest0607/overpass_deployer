package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunRequiresExplicitPaths(t *testing.T) {
	for _, args := range [][]string{{}, {"backup"}, {"verify"}, {"restore"}, {"inspect"}, {"unknown"}} {
		var output bytes.Buffer
		if err := run(args, &output); err == nil {
			t.Fatalf("args %v should fail", args)
		}
	}
}

func TestRunRejectsUnexpectedArguments(t *testing.T) {
	var output bytes.Buffer
	err := run([]string{"inspect", "--data-dir", "/tmp/studio", "extra"}, &output)
	if err == nil || !strings.Contains(err.Error(), "inspect requires") {
		t.Fatalf("unexpected error: %v", err)
	}
}
