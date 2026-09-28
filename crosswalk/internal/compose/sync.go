package compose

import (
	"fmt"
	"path"

	"go-deployer/internal/config"
	"go-deployer/internal/docker"
	"go-deployer/internal/scp"
	"go-deployer/internal/ssh"
)

type transferOutcome struct {
	changed bool
}

func (o *transferOutcome) TransferChanged(string) { o.changed = true }
func (o *transferOutcome) TransferSkipped(string) {}

var transferComposeFile = func(runner docker.RemoteRunner, localPath, remotePath string, observer scp.TransferObserver) error {
	client, ok := runner.(*ssh.Client)
	if !ok {
		return fmt.Errorf("Compose file transfer requires an SSH client")
	}
	return scp.Transfer(client, localPath, remotePath, scp.TransferOptions{Observer: observer})
}

var syncComposeFile = syncComposeProjectFile

func syncComposeProjectFile(runner docker.RemoteRunner, project config.ComposeProjectConfig) (string, bool, error) {
	remoteFile := path.Join(project.RemoteDir, "compose.yml")
	if _, err := runner.Run("install -d -m 0700 " + ssh.ShellQuote(project.RemoteDir)); err != nil {
		return "", false, fmt.Errorf("prepare remote Compose directory: %w", err)
	}
	outcome := &transferOutcome{}
	if err := transferComposeFile(runner, project.LocalPath, remoteFile, outcome); err != nil {
		return "", false, fmt.Errorf("sync Compose file: %w", err)
	}
	if _, err := runner.Run("chmod 0600 " + ssh.ShellQuote(remoteFile)); err != nil {
		return "", false, fmt.Errorf("restrict remote Compose file permissions: %w", err)
	}
	return remoteFile, outcome.changed, nil
}
