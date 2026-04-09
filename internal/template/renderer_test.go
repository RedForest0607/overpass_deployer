package template

import (
	"os"
	"strings"
	"testing"
)

func TestRenderUsesEmbeddedTemplateAndSubstitutesVariables(t *testing.T) {
	tmpFile, err := Render("embedded:server.sh.tmpl", "server.sh", map[string]any{
		"AppName":       "sample-app",
		"BaseDir":       "/opt/sample",
		"JarPath":       "/opt/sample/bin/app.jar",
		"Port":          8080,
		"JvmMin":        "256m",
		"JvmMax":        "1g",
		"JavaOpts":      []string{"-Dspring.profiles.active=prod"},
		"ExtraOpts":     []string{"--debug"},
		"ActiveProfile": "prod",
		"ContextPath":   "health",
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
		`# 1. 애플리케이션 설정 (이 부분의 값만 수정하세요)`,
		`JAR_FILE="/opt/sample/bin/app.jar"`,
		`CONTEXT_PATH="${CONTEXT_PATH:-health}"`,
		`ACTIVE_PROFILE="${ACTIVE_PROFILE:-prod}"`,
		`#hamonica`,
		`JAVA_OPTS+="-Dspring.profiles.active=prod "`,
		`SPRING_OPTS+=" --debug"`,
		`status() {`,
		`tail -f "$LOG_FILE"`,
		`restart)`,
		`사용법: $0 {start|stop|restart|status|log}`,
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

	tmpFile, err := Render(customTemplate.Name(), "server.sh", map[string]any{
		"AppName": "custom-app",
		"Port":    9090,
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

func TestRenderFailsWhenTemplateValueIsMissing(t *testing.T) {
	customTemplate, err := os.CreateTemp("", "custom-missing-*.tmpl")
	if err != nil {
		t.Fatalf("creating custom template: %v", err)
	}
	defer os.Remove(customTemplate.Name())

	if _, err := customTemplate.WriteString("custom {{ .MissingValue }}\n"); err != nil {
		t.Fatalf("writing custom template: %v", err)
	}
	if err := customTemplate.Close(); err != nil {
		t.Fatalf("closing custom template: %v", err)
	}

	_, err = Render(customTemplate.Name(), "server.sh", map[string]any{
		"AppName": "custom-app",
	})
	if err == nil || !strings.Contains(err.Error(), "MissingValue") {
		t.Fatalf("expected missing value error, got %v", err)
	}
}
