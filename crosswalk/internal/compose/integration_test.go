package compose

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"go-deployer/internal/config"
)

func TestComposeApplySSHFixtureLifecycle(t *testing.T) {
	fixture := startComposeSSHFixture(t)
	t.Setenv("M7_FIXTURE_TOKEN", "fixture-token")
	composePath := filepath.Join(fixture.root, "compose.yml")
	remoteDir := filepath.Join(fixture.root, "remote", "commerce")
	cfg := &config.Config{
		SSH: config.SSHConfig{
			User: "fixture", KeyPath: fixture.keyPath,
			KnownHosts:      filepath.Join(fixture.root, "known_hosts"),
			HostKeyChecking: config.HostKeyAcceptNew,
			Port:            fixture.port(), TimeoutSec: 3,
		},
		Docker: config.DockerDeploymentConfig{Registry: config.DockerRegistryConfig{
			Host: "fixture.registry", Username: "fixture-user", TokenEnv: "M7_FIXTURE_TOKEN",
		}},
		Servers: []config.ServerConfig{{
			Name: "compose-fixture", Host: "127.0.0.1", Tags: []string{"fixture"},
			ComposeProjects: []config.ComposeProjectConfig{{Name: "commerce", LocalPath: composePath, RemoteDir: remoteDir}},
		}},
	}

	v1 := fixtureComposeDocument("v1", false)
	writeComposeFixtureDocument(t, composePath, v1)
	if err := Run(cfg, Options{Apply: true}); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	assertActiveComposeRevision(t, remoteDir, fixtureRevision(v1))
	assertFixtureCount(t, fixture.stateDir, "up-count", 1)

	if err := Run(cfg, Options{Apply: true}); err != nil {
		t.Fatalf("idempotent reapply: %v", err)
	}
	assertFixtureCount(t, fixture.stateDir, "up-count", 1)

	v2 := fixtureComposeDocument("v2", false)
	writeComposeFixtureDocument(t, composePath, v2)
	if err := Run(cfg, Options{Apply: true}); err != nil {
		t.Fatalf("upgrade apply: %v", err)
	}
	v2Revision := fixtureRevision(v2)
	assertActiveComposeRevision(t, remoteDir, v2Revision)
	assertFixtureCount(t, fixture.stateDir, "up-count", 2)

	broken := fixtureComposeDocument("broken", true)
	writeComposeFixtureDocument(t, composePath, broken)
	err := Run(cfg, Options{Apply: true})
	if err == nil || !strings.Contains(err.Error(), "previous revision restored") {
		t.Fatalf("health failure should restore the previous revision, error=%v", err)
	}
	assertActiveComposeRevision(t, remoteDir, v2Revision)
	assertFixtureCount(t, fixture.stateDir, "up-count", 4)
	lastRelease, err := os.ReadFile(filepath.Join(fixture.stateDir, "last-release"))
	if err != nil || strings.TrimSpace(string(lastRelease)) != "v2" {
		t.Fatalf("rollback release=%q error=%v", lastRelease, err)
	}
}

func fixtureComposeDocument(release string, failHealth bool) []byte {
	health := "false"
	if failHealth {
		health = "true"
	}
	return []byte("name: commerce\nservices:\n  api:\n    image: fixture.registry/demo/api@sha256:" + strings.Repeat("a", 64) + "\n    environment:\n      RELEASE: \"" + release + "\"\n      FAIL_HEALTH: \"" + health + "\"\n    restart: unless-stopped\n")
}

func writeComposeFixtureDocument(t *testing.T, target string, content []byte) {
	t.Helper()
	if err := os.WriteFile(target, content, 0o600); err != nil {
		t.Fatalf("write Compose fixture: %v", err)
	}
}

func fixtureRevision(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func assertActiveComposeRevision(t *testing.T, remoteDir, revision string) {
	t.Helper()
	target, err := os.Readlink(filepath.Join(remoteDir, "active"))
	if err != nil {
		t.Fatalf("read active revision: %v", err)
	}
	want := ".revisions/" + revision
	if target != want {
		t.Fatalf("active revision=%q, want %q", target, want)
	}
}

func assertFixtureCount(t *testing.T, stateDir, name string, want int) {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(stateDir, name))
	if err != nil {
		t.Fatalf("read fixture count %s: %v", name, err)
	}
	got, err := strconv.Atoi(strings.TrimSpace(string(content)))
	if err != nil || got != want {
		t.Fatalf("fixture count %s=%q, want %d (error=%v)", name, content, want, err)
	}
}
