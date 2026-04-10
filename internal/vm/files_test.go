package vm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-deployer/internal/config"
)

func TestPrepareScriptSourceUsesLocalFileMode(t *testing.T) {
	scriptPath := tempFile(t, "server.sh")
	app := &config.AppConfig{
		Name: "sample",
		Script: config.ScriptConfig{
			Mode:      config.ScriptModeLocalFile,
			LocalPath: scriptPath,
		},
	}

	gotPath, gotDescription, cleanup, err := prepareScriptSource(app)
	if err != nil {
		t.Fatalf("prepareScriptSource failed: %v", err)
	}
	if cleanup != nil {
		t.Fatal("expected no cleanup function for local-file mode")
	}
	if gotPath != scriptPath {
		t.Fatalf("expected local script path %q, got %q", scriptPath, gotPath)
	}
	if gotDescription != scriptPath {
		t.Fatalf("expected description %q, got %q", scriptPath, gotDescription)
	}
}

func TestPrepareScriptSourceRendersTemplateMode(t *testing.T) {
	app := &config.AppConfig{
		Name:    "sample",
		BaseDir: "/opt/sample",
		Port:    8080,
		Jar: config.JarConfig{
			RemotePath: "/opt/sample/lib/app.jar",
		},
		Jvm: config.JvmConfig{
			MinHeap: "256m",
			MaxHeap: "1g",
		},
		Script: config.ScriptConfig{
			Mode: config.ScriptModeTemplate,
		},
	}

	gotPath, gotDescription, cleanup, err := prepareScriptSource(app)
	if err != nil {
		t.Fatalf("prepareScriptSource failed: %v", err)
	}
	if gotDescription != "embedded:server.sh.tmpl" {
		t.Fatalf("expected embedded template description, got %q", gotDescription)
	}
	if cleanup == nil {
		t.Fatal("expected cleanup function for rendered template")
	}
	if _, err := os.Stat(gotPath); err != nil {
		t.Fatalf("expected rendered script file to exist: %v", err)
	}
	cleanup()
	if _, err := os.Stat(gotPath); !os.IsNotExist(err) {
		t.Fatalf("expected rendered script file to be removed, got err=%v", err)
	}
}

func TestPrepareScriptSourceUsesMatchingTemplateValues(t *testing.T) {
	valuesPath := filepath.Join(t.TempDir(), "server.values.yml")
	valuesContent := "" +
		"HAMONICA_JAVA_AGENT: otel-javaagent-hamonica-2.0.0-SNAPSHOT.jar\n" +
		"HAMONICA_CONFIG_FILE: hamonica2-otel.fo-pcweb.config.properties\n"
	if err := os.WriteFile(valuesPath, []byte(valuesContent), 0o600); err != nil {
		t.Fatalf("write values file: %v", err)
	}

	app := &config.AppConfig{
		Name:    "sample",
		BaseDir: "/opt/sample",
		Port:    8080,
		Jar: config.JarConfig{
			RemotePath: "/opt/sample/lib/app.jar",
		},
		Jvm: config.JvmConfig{
			MinHeap: "256m",
			MaxHeap: "1g",
		},
		Script: config.ScriptConfig{
			Mode:       config.ScriptModeTemplate,
			ValuesFile: valuesPath,
		},
	}

	gotPath, _, cleanup, err := prepareScriptSource(app)
	if err != nil {
		t.Fatalf("prepareScriptSource failed: %v", err)
	}
	defer cleanup()

	content, err := os.ReadFile(gotPath)
	if err != nil {
		t.Fatalf("read rendered script: %v", err)
	}

	rendered := string(content)
	for _, fragment := range []string{
		`HAMONICA_HOME="${HAMONICA_HOME:-/app/software/hamonica2-agent}"`,
		`HAMONICA_JAVA_AGENT="${HAMONICA_JAVA_AGENT:-otel-javaagent-hamonica-2.0.0-SNAPSHOT.jar}"`,
		`HAMONICA_CONFIG_FILE="${HAMONICA_CONFIG_FILE:-hamonica2-otel.fo-pcweb.config.properties}"`,
	} {
		if !strings.Contains(rendered, fragment) {
			t.Fatalf("expected rendered script to contain %q, got:\n%s", fragment, rendered)
		}
	}
}
