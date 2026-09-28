package compose

import (
	"strings"
	"testing"

	"go-deployer/internal/docker"
	"go-deployer/internal/scp"
)

func TestSyncComposeProjectFileRestrictsPermissionsAndTracksChange(t *testing.T) {
	runner := &fakeRemoteRunner{}
	original := transferComposeFile
	transferComposeFile = func(_ docker.RemoteRunner, localPath, remotePath string, observer scp.TransferObserver) error {
		if localPath != "./compose/compose-dev-01/compose.yml" || remotePath != "/app/overpass/compose/commerce/compose.yml" {
			t.Fatalf("local=%q remote=%q", localPath, remotePath)
		}
		observer.TransferChanged(remotePath)
		return nil
	}
	t.Cleanup(func() { transferComposeFile = original })

	remoteFile, changed, err := syncComposeProjectFile(runner, composeConfig().Servers[0].ComposeProjects[0])
	if err != nil {
		t.Fatal(err)
	}
	if !changed || remoteFile != "/app/overpass/compose/commerce/compose.yml" {
		t.Fatalf("remote=%q changed=%t", remoteFile, changed)
	}
	commands := strings.Join(runner.commands, "\n")
	if !strings.Contains(commands, "install -d -m 0700 '/app/overpass/compose/commerce'") ||
		!strings.Contains(commands, "chmod 0600 '/app/overpass/compose/commerce/compose.yml'") {
		t.Fatalf("permission commands=%q", commands)
	}
}

func TestSyncComposeProjectFilePreservesChecksumSkip(t *testing.T) {
	runner := &fakeRemoteRunner{}
	original := transferComposeFile
	transferComposeFile = func(_ docker.RemoteRunner, _, remotePath string, observer scp.TransferObserver) error {
		observer.TransferSkipped(remotePath)
		return nil
	}
	t.Cleanup(func() { transferComposeFile = original })

	_, changed, err := syncComposeProjectFile(runner, composeConfig().Servers[0].ComposeProjects[0])
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("checksum-identical Compose file must remain skipped")
	}
}
