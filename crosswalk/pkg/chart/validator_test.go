package chart

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateFileUsesDeploymentValidationBoundary(t *testing.T) {
	fixture, err := filepath.Abs("../../../studio/backend/internal/generator/crosswalkvm/testdata/loader-compat")
	if err != nil {
		t.Fatal(err)
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(fixture); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })

	metadata, err := ValidateFile("deploy.yml")
	if err != nil {
		t.Fatal(err)
	}
	if metadata.Format != FormatV1 {
		t.Fatalf("format=%q", metadata.Format)
	}
}
