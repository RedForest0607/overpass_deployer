package vm

import (
	"os"
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
