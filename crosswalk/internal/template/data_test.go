package template

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadTemplateDataSubstitutesEnvVariables(t *testing.T) {
	t.Setenv("ACTIVE_PROFILE", "prod")

	valuesPath := filepath.Join(t.TempDir(), "values.yml")
	if err := os.WriteFile(valuesPath, []byte("ActiveProfile: ${ACTIVE_PROFILE}\nContextPath: api\n"), 0o600); err != nil {
		t.Fatalf("writing values file: %v", err)
	}

	values, err := LoadTemplateData(valuesPath)
	if err != nil {
		t.Fatalf("loading template data: %v", err)
	}

	if values["ActiveProfile"] != "prod" {
		t.Fatalf("expected ActiveProfile to be substituted, got %#v", values["ActiveProfile"])
	}
	if values["ContextPath"] != "api" {
		t.Fatalf("expected ContextPath to be loaded, got %#v", values["ContextPath"])
	}
}

func TestMergeTemplateDataOverridesBaseValues(t *testing.T) {
	base := map[string]any{
		"AppName":       "sample-app",
		"ActiveProfile": "",
	}
	overrides := map[string]any{
		"ActiveProfile": "prod",
		"ContextPath":   "health",
	}

	merged := MergeTemplateData(base, overrides)

	expected := map[string]any{
		"AppName":       "sample-app",
		"ActiveProfile": "prod",
		"ContextPath":   "health",
	}
	if !reflect.DeepEqual(merged, expected) {
		t.Fatalf("unexpected merged data: got %#v want %#v", merged, expected)
	}
}

func TestLoadTemplateDataRejectsUnresolvedEnvironmentVariables(t *testing.T) {
	valuesPath := filepath.Join(t.TempDir(), "values.yml")
	if err := os.WriteFile(valuesPath, []byte("ActiveProfile: ${ACTIVE_PROFILE}\n"), 0o600); err != nil {
		t.Fatalf("writing values file: %v", err)
	}

	_, err := LoadTemplateData(valuesPath)
	if err == nil || !strings.Contains(err.Error(), "contains unresolved environment variable") {
		t.Fatalf("expected unresolved env error, got %v", err)
	}
}
