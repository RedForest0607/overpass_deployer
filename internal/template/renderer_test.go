package template

import (
	"os"
	"strings"
	"testing"

	"go-deployer/internal/config"
)

func TestRenderUsesEmbeddedTemplateAndSubstitutesVariables(t *testing.T) {
	tmpFile, err := Render("", "start.sh", config.ScriptData{
		AppName:   "sample-app",
		BaseDir:   "/opt/sample",
		JarPath:   "/opt/sample/bin/app.jar",
		Port:      8080,
		JvmMin:    "256m",
		JvmMax:    "1g",
		JavaOpts:  []string{"-Dspring.profiles.active=prod"},
		ExtraOpts: []string{"--debug"},
	})
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}
	defer os.Remove(tmpFile)

	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("reading rendered file: %v", err)
	}

	rendered := string(content)
	for _, fragment := range []string{
		`APP_NAME="sample-app"`,
		`JAR_PATH="/opt/sample/bin/app.jar"`,
		`"-Dspring.profiles.active=prod"`,
		`"--debug"`,
	} {
		if !strings.Contains(rendered, fragment) {
			t.Fatalf("expected rendered template to contain %q, got:\n%s", fragment, rendered)
		}
	}
}

func TestRenderPrefersCustomTemplate(t *testing.T) {
	customTemplate, err := os.CreateTemp("", "custom-start-*.tmpl")
	if err != nil {
		t.Fatalf("creating custom template: %v", err)
	}
	defer os.Remove(customTemplate.Name())

	if _, err := customTemplate.WriteString("custom {{ .AppName }} {{ .Port }}\n"); err != nil {
		t.Fatalf("writing custom template: %v", err)
	}
	if err := customTemplate.Close(); err != nil {
		t.Fatalf("closing custom template: %v", err)
	}

	tmpFile, err := Render(customTemplate.Name(), "start.sh", config.ScriptData{
		AppName: "custom-app",
		Port:    9090,
	})
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}
	defer os.Remove(tmpFile)

	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("reading rendered file: %v", err)
	}

	rendered := strings.TrimSpace(string(content))
	if rendered != "custom custom-app 9090" {
		t.Fatalf("expected custom template to win, got %q", rendered)
	}
}
