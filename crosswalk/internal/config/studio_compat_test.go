package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStudioGeneratedChartCompatibility(t *testing.T) {
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

	cfg, err := Load("deploy.yml")
	if err != nil {
		t.Fatalf("Studio chart must load with the actual Crosswalk loader: %v", err)
	}
	if len(cfg.Servers) != 1 || len(cfg.Servers[0].Utils) != 2 || len(cfg.Servers[0].Apps) != 1 {
		t.Fatalf("unexpected normalized chart: %#v", cfg.Servers)
	}
	if cfg.Servers[0].Apps[0].Script.Mode != ScriptModeLocalFile {
		t.Fatalf("script mode=%q", cfg.Servers[0].Apps[0].Script.Mode)
	}
	if len(cfg.Servers[0].Apps[0].Tags) != 2 {
		t.Fatalf("app tags=%v", cfg.Servers[0].Apps[0].Tags)
	}
	if cfg.Servers[0].Bootstrap.Timezone.Name != "Asia/Seoul" {
		t.Fatalf("global bootstrap was not merged: %#v", cfg.Servers[0].Bootstrap)
	}
}
