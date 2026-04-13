package vm

import (
	"reflect"
	"strings"
	"testing"

	"go-deployer/internal/config"
)

func TestFilterConfigMatchesServerTags(t *testing.T) {
	t.Helper()

	cfg := &config.Config{
		Servers: []config.ServerConfig{
			{Name: "devapm1", Host: "10.0.0.10", Tags: []string{"wave1", "apm"}},
			{Name: "devwas", Host: "10.0.0.20", Tags: []string{"wave2", "was"}},
		},
	}

	filtered, err := filterConfig(cfg, RunOptions{ServerTags: []string{"apm"}})
	if err != nil {
		t.Fatalf("expected server tag filtering to succeed, got %v", err)
	}
	if len(filtered.Servers) != 1 || filtered.Servers[0].Name != "devapm1" {
		t.Fatalf("expected only devapm1 to remain, got %#v", filtered.Servers)
	}
}

func TestFilterConfigMatchesAppTagsAndPreservesServerLevelWork(t *testing.T) {
	t.Helper()

	cfg := &config.Config{
		Servers: []config.ServerConfig{
			{
				Name:        "devwas",
				Host:        "10.0.0.20",
				Directories: []string{"/app/shared"},
				Apps: []config.AppConfig{
					{Name: "fo-api", Tags: []string{"fo"}},
					{Name: "bo-pcweb", Tags: []string{"bo"}},
				},
			},
		},
	}

	filtered, err := filterConfig(cfg, RunOptions{AppTags: []string{"fo"}})
	if err != nil {
		t.Fatalf("expected app tag filtering to succeed, got %v", err)
	}
	if len(filtered.Servers) != 1 {
		t.Fatalf("expected one filtered server, got %d", len(filtered.Servers))
	}
	if !reflect.DeepEqual(filtered.Servers[0].Directories, []string{"/app/shared"}) {
		t.Fatalf("expected server-level directories to be preserved, got %v", filtered.Servers[0].Directories)
	}
	if len(filtered.Servers[0].Apps) != 1 || filtered.Servers[0].Apps[0].Name != "fo-api" {
		t.Fatalf("expected only fo-api to remain, got %#v", filtered.Servers[0].Apps)
	}
}

func TestFilterConfigSupportsLegacyAppWithAppTagFilter(t *testing.T) {
	t.Helper()

	cfg := &config.Config{
		Servers: []config.ServerConfig{
			{
				Name: "devapm1",
				Host: "10.0.0.10",
				App: config.AppConfig{
					Name: "hamonica2-api",
					Tags: []string{"api", "apm"},
				},
			},
		},
	}

	filtered, err := filterConfig(cfg, RunOptions{AppTags: []string{"api"}})
	if err != nil {
		t.Fatalf("expected legacy app filtering to succeed, got %v", err)
	}
	if len(filtered.Servers) != 1 {
		t.Fatalf("expected one filtered server, got %d", len(filtered.Servers))
	}
	if filtered.Servers[0].App.Name != "hamonica2-api" || len(filtered.Servers[0].Apps) != 0 {
		t.Fatalf("expected legacy app shape to be preserved, got %#v", filtered.Servers[0])
	}
}

func TestFilterConfigReturnsErrorWhenNoServerMatches(t *testing.T) {
	t.Helper()

	cfg := &config.Config{
		Servers: []config.ServerConfig{
			{Name: "devwas", Host: "10.0.0.20", Tags: []string{"wave2"}},
		},
	}

	_, err := filterConfig(cfg, RunOptions{ServerTags: []string{"wave1"}})
	if err == nil || !strings.Contains(err.Error(), "no servers matched the requested tag filters") {
		t.Fatalf("expected no-match error, got %v", err)
	}
}
