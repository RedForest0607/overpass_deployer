package config

import (
	"strings"
	"testing"
)

func TestParseRequiresDataDirectory(t *testing.T) {
	_, err := Parse(nil, func(string) (string, bool) { return "", false })
	if err == nil || !strings.Contains(err.Error(), "data directory is required") {
		t.Fatalf("expected missing data directory error, got %v", err)
	}
}

func TestParseRejectsPublicBinding(t *testing.T) {
	_, err := Parse([]string{"--data-dir", t.TempDir(), "--address", "0.0.0.0:8080"}, nil)
	if err == nil || !strings.Contains(err.Error(), "not loopback") {
		t.Fatalf("expected public binding error, got %v", err)
	}
}

func TestParseReadsEnvironmentAndFlagsOverrideIt(t *testing.T) {
	values := map[string]string{
		"STUDIO_DATA_DIR":        "/env/data",
		"STUDIO_ADDRESS":         "localhost:9000",
		"STUDIO_ALLOWED_ORIGINS": "http://localhost:3000",
	}
	lookup := func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}

	cfg, err := Parse([]string{"--data-dir", "/flag/data"}, lookup)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DataDir != "/flag/data" || cfg.Address != "localhost:9000" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if len(cfg.AllowedOrigins) != 1 || cfg.AllowedOrigins[0] != "http://localhost:3000" {
		t.Fatalf("unexpected origins: %v", cfg.AllowedOrigins)
	}
}

func TestParseConfiguresTrustedHeaderAuthentication(t *testing.T) {
	values := map[string]string{
		"STUDIO_DATA_DIR":          t.TempDir(),
		"STUDIO_AUTH_MODE":         "trusted-header",
		"STUDIO_AUTH_HEADER":       "X-Verified-User",
		"STUDIO_AUTH_PROXY_SECRET": strings.Repeat("s", 32),
	}
	cfg, err := Parse(nil, func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AuthMode != "trusted-header" || cfg.AuthHeader != "X-Verified-User" || cfg.ProxySecret == "" {
		t.Fatalf("unexpected authentication config: %+v", cfg)
	}
}

func TestParseRejectsUnsafeAuthenticationConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		values map[string]string
	}{
		{name: "unknown mode", values: map[string]string{"STUDIO_AUTH_MODE": "public"}},
		{name: "missing proxy secret", values: map[string]string{"STUDIO_AUTH_MODE": "trusted-header"}},
		{name: "short proxy secret", values: map[string]string{"STUDIO_AUTH_MODE": "trusted-header", "STUDIO_AUTH_PROXY_SECRET": "short"}},
		{name: "invalid identity header", values: map[string]string{"STUDIO_AUTH_HEADER": "X User"}},
		{name: "same headers", values: map[string]string{"STUDIO_AUTH_HEADER": "X-Studio-Proxy-Secret"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.values["STUDIO_DATA_DIR"] = t.TempDir()
			_, err := Parse(nil, func(key string) (string, bool) {
				value, ok := test.values[key]
				return value, ok
			})
			if err == nil {
				t.Fatal("expected invalid authentication configuration")
			}
		})
	}
}
