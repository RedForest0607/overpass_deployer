package compose

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-deployer/internal/config"
	"go-deployer/internal/docker"
	"go-deployer/internal/scp"
)

const testImage = "harbor.example.com/demo/api@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func fixtureCompose(t *testing.T) (config.ComposeProjectConfig, desiredCompose) {
	t.Helper()
	local := filepath.Join(t.TempDir(), "compose.yml")
	if err := os.WriteFile(local, []byte("services:\n  api:\n    image: "+testImage+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	project := config.ComposeProjectConfig{Name: "commerce", LocalPath: local, RemoteDir: "/app/compose/commerce"}
	desired, err := readDesiredCompose(local, "harbor.example.com")
	if err != nil {
		t.Fatal(err)
	}
	return project, desired
}

func TestReadDesiredComposeRejectsMutableOrForeignImages(t *testing.T) {
	for _, image := range []string{"harbor.example.com/demo/api:latest", "evil.example.com/demo/api@sha256:" + strings.Repeat("a", 64)} {
		local := filepath.Join(t.TempDir(), "compose.yml")
		if err := os.WriteFile(local, []byte("services:\n  api:\n    image: "+image+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := readDesiredCompose(local, "harbor.example.com"); err == nil {
			t.Fatalf("accepted %q", image)
		}
	}
}

func TestReadDesiredComposeAcceptsRegistryPort(t *testing.T) {
	image := "registry.example.com:5443/demo/api@sha256:" + strings.Repeat("a", 64)
	local := filepath.Join(t.TempDir(), "compose.yml")
	if err := os.WriteFile(local, []byte("services:\n  api:\n    image: "+image+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	desired, err := readDesiredCompose(local, "registry.example.com:5443")
	if err != nil {
		t.Fatalf("registry with port was rejected: %v", err)
	}
	if len(desired.images) != 1 || desired.images[0] != image {
		t.Fatalf("images=%q", desired.images)
	}
}

func TestReadDesiredComposeRejectsExternalInputsAndInterpolation(t *testing.T) {
	for _, extra := range []string{
		"    env_file: .env\n",
		"    build: .\n",
		"    environment:\n      TOKEN: ${RUNTIME_TOKEN}\n",
	} {
		local := filepath.Join(t.TempDir(), "compose.yml")
		content := "services:\n  api:\n    image: " + testImage + "\n" + extra
		if err := os.WriteFile(local, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := readDesiredCompose(local, "harbor.example.com"); err == nil {
			t.Fatalf("accepted external input: %s", extra)
		}
	}
}

func TestApplyProjectStagesPullsVerifiesAndSwitchesBeforeUp(t *testing.T) {
	project, desired := fixtureCompose(t)
	pulled := false
	uploaded := false
	runner := &fakeRemoteRunner{}
	runner.runHook = func(command string) (string, error) {
		switch {
		case strings.Contains(command, "readlink "):
			return "NONE\n", nil
		case strings.Contains(command, "sha256sum "):
			if uploaded {
				return desired.revision + "  compose.yml\n", nil
			}
			return "MISSING\n", nil
		case strings.Contains(command, " config --services"):
			return "api\n", nil
		case strings.Contains(command, " config --images"):
			return testImage + "\n", nil
		case strings.Contains(command, "docker image ls"):
			if pulled {
				return testImage + "\n", nil
			}
			return "", nil
		case strings.Contains(command, "docker image inspect"):
			return `["` + testImage + `"]`, nil
		case strings.Contains(command, "docker pull "):
			pulled = true
		case strings.Contains(command, " ps --all --format json"):
			return `{"Service":"api","State":"running","Health":"healthy"}`, nil
		}
		return "", nil
	}
	original := transferComposeFile
	transferComposeFile = func(_ docker.RemoteRunner, _, _ string, _ scp.TransferObserver) error { uploaded = true; return nil }
	t.Cleanup(func() { transferComposeFile = original })

	changed, err := applyProject(runner, project, desired)
	if err != nil || !changed || !uploaded || !pulled {
		t.Fatalf("changed=%t uploaded=%t pulled=%t err=%v", changed, uploaded, pulled, err)
	}
	commands := strings.Join(runner.commands, "\n")
	for _, fragment := range []string{"config --quiet", "config --services", "config --images", "docker pull", "mv -Tf", " up -d --remove-orphans --pull never --no-build", " ps --all --format json", "rmdir "} {
		if !strings.Contains(commands, fragment) {
			t.Fatalf("missing %q in %s", fragment, commands)
		}
	}
	if strings.Index(commands, "docker pull") > strings.Index(commands, "mv -Tf") || strings.Index(commands, "mv -Tf") > strings.Index(commands, " up -d ") {
		t.Fatalf("unsafe apply ordering: %s", commands)
	}
}

func TestApplyProjectRestoresPreviousRevisionOnFailedHealth(t *testing.T) {
	project, desired := fixtureCompose(t)
	previous := ".revisions/" + strings.Repeat("b", 64)
	runner := &fakeRemoteRunner{}
	runner.runHook = func(command string) (string, error) {
		switch {
		case strings.Contains(command, "readlink "):
			return previous + "\n", nil
		case strings.Contains(command, "sha256sum "):
			if strings.Contains(command, previous) {
				return strings.TrimPrefix(previous, ".revisions/") + "  compose.yml\n", nil
			}
			return desired.revision + "  compose.yml\n", nil
		case strings.Contains(command, " config --services"):
			return "api\n", nil
		case strings.Contains(command, " config --images"):
			return testImage + "\n", nil
		case strings.Contains(command, "docker image ls"):
			return testImage + "\n", nil
		case strings.Contains(command, "docker image inspect"):
			return `["` + testImage + `"]`, nil
		case strings.Contains(command, " ps --all --format json"):
			if strings.Contains(command, previous) {
				return `{"Service":"api","State":"running"}`, nil
			}
			return `{"Service":"api","State":"running","Health":"unhealthy"}`, nil
		}
		return "", nil
	}
	_, err := applyProject(runner, project, desired)
	if err == nil || !strings.Contains(err.Error(), "previous revision restored") {
		t.Fatalf("err=%v", err)
	}
	commands := strings.Join(runner.commands, "\n")
	if strings.Count(commands, "mv -Tf") != 2 || !strings.Contains(commands, "ln -s '"+previous+"'") || !strings.Contains(commands, previous+"/compose.yml' up -d") {
		t.Fatalf("rollback not performed: %s", commands)
	}
}

func TestApplyProjectStopsFirstRevisionAndClearsActiveLinkOnFailedHealth(t *testing.T) {
	project, desired := fixtureCompose(t)
	runner := &fakeRemoteRunner{runHook: func(command string) (string, error) {
		switch {
		case strings.Contains(command, "readlink "):
			return "NONE\n", nil
		case strings.Contains(command, "sha256sum "):
			return desired.revision + "  compose.yml\n", nil
		case strings.Contains(command, " config --services"):
			return "api\n", nil
		case strings.Contains(command, " config --images"):
			return testImage + "\n", nil
		case strings.Contains(command, "docker image ls"):
			return testImage + "\n", nil
		case strings.Contains(command, "docker image inspect"):
			return `["` + testImage + `"]`, nil
		case strings.Contains(command, " ps --all --format json"):
			return `{"Service":"api","State":"running","Health":"unhealthy"}`, nil
		}
		return "", nil
	}}
	_, err := applyProject(runner, project, desired)
	if err == nil || !strings.Contains(err.Error(), "first revision stopped and deactivated") {
		t.Fatalf("err=%v", err)
	}
	commands := strings.Join(runner.commands, "\n")
	if !strings.Contains(commands, " down --remove-orphans") || !strings.Contains(commands, "unlink '/app/compose/commerce/active'") {
		t.Fatalf("first revision was not stopped and deactivated: %s", commands)
	}
}

func TestApplyProjectRejectsUnmanagedContainersBeforeStaging(t *testing.T) {
	project, desired := fixtureCompose(t)
	runner := &fakeRemoteRunner{runHook: func(command string) (string, error) {
		if strings.Contains(command, "readlink ") {
			return "NONE", nil
		}
		if strings.Contains(command, "docker ps --all") {
			return "existing-id\n", nil
		}
		return "", nil
	}}
	_, err := applyProject(runner, project, desired)
	if err == nil || !strings.Contains(err.Error(), "unmanaged") {
		t.Fatalf("err=%v", err)
	}
	if strings.Contains(strings.Join(runner.commands, "\n"), " up -d ") {
		t.Fatal("unmanaged project was applied")
	}
}

func TestApplyProjectRejectsTamperedPreviousRevisionBeforeSwitch(t *testing.T) {
	project, desired := fixtureCompose(t)
	previous := ".revisions/" + strings.Repeat("b", 64)
	runner := &fakeRemoteRunner{runHook: func(command string) (string, error) {
		if strings.Contains(command, "readlink ") {
			return previous, nil
		}
		if strings.Contains(command, "sha256sum ") {
			return strings.Repeat("c", 64) + "  compose.yml", nil
		}
		return "", nil
	}}
	_, err := applyProject(runner, project, desired)
	if err == nil || !strings.Contains(err.Error(), "previous revision unavailable") {
		t.Fatalf("error=%v", err)
	}
	if strings.Contains(strings.Join(runner.commands, "\n"), "mv -Tf") {
		t.Fatal("active revision switched without a verified rollback candidate")
	}
}

func TestPullImagesStopsOnInspectErrorBeforeMutating(t *testing.T) {
	runner := &fakeRemoteRunner{runHook: func(command string) (string, error) {
		if strings.Contains(command, "docker image ls") {
			return "", errors.New("daemon unavailable")
		}
		return "", nil
	}}
	if _, err := pullImages(runner, []string{testImage}); err == nil {
		t.Fatal("image query error was ignored")
	}
	if strings.Contains(strings.Join(runner.commands, "\n"), "docker pull") {
		t.Fatal("pull must not run after image query error")
	}
}
