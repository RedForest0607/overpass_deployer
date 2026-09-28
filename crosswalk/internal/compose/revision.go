package compose

import (
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"

	"go-deployer/internal/config"
	"go-deployer/internal/docker"
	"go-deployer/internal/ssh"
)

var revisionLink = regexp.MustCompile(`^\.revisions/[a-f0-9]{64}$`)

type revisionPaths struct {
	root, directory, file, active, target string
}

func pathsFor(project config.ComposeProjectConfig, revision string) revisionPaths {
	return revisionPaths{
		root: project.RemoteDir, directory: path.Join(project.RemoteDir, ".revisions", revision),
		file:   path.Join(project.RemoteDir, ".revisions", revision, "compose.yml"),
		active: path.Join(project.RemoteDir, "active"), target: ".revisions/" + revision,
	}
}

func acquireRevisionLock(runner docker.RemoteRunner, root string) (func() error, error) {
	if root == "/" {
		return nil, errors.New("Compose remote directory cannot be the filesystem root for apply")
	}
	if _, err := runner.Run("install -d -m 0700 " + ssh.ShellQuote(root)); err != nil {
		return nil, errors.New("cannot prepare Compose project directory")
	}
	if _, err := runner.Run("chmod 0700 " + ssh.ShellQuote(root)); err != nil {
		return nil, errors.New("cannot restrict Compose project directory")
	}
	lock := path.Join(root, ".crosswalk-lock")
	if _, err := runner.Run("mkdir -m 0700 " + ssh.ShellQuote(lock)); err != nil {
		return nil, errors.New("Compose project is locked; inspect and clear a stale .crosswalk-lock after confirming no deployment is running")
	}
	return func() error {
		if _, err := runner.Run("rmdir " + ssh.ShellQuote(lock)); err != nil {
			return errors.New("failed to release Compose deployment lock")
		}
		return nil
	}, nil
}

func stageRevision(runner docker.RemoteRunner, project config.ComposeProjectConfig, paths revisionPaths, revision string) (bool, error) {
	if _, err := runner.Run("install -d -m 0700 " + ssh.ShellQuote(paths.directory)); err != nil {
		return false, errors.New("failed to prepare Compose revision directory")
	}
	if _, err := runner.Run("chmod 0700 " + ssh.ShellQuote(path.Join(paths.root, ".revisions")) + " " + ssh.ShellQuote(paths.directory)); err != nil {
		return false, errors.New("failed to restrict Compose revision directories")
	}
	command := "if [ -f " + ssh.ShellQuote(paths.file) + " ]; then sha256sum " + ssh.ShellQuote(paths.file) + "; elif [ -e " + ssh.ShellQuote(paths.file) + " ] || [ -L " + ssh.ShellQuote(paths.file) + " ]; then echo INVALID; else echo MISSING; fi"
	output, err := runner.Run(command)
	if err != nil {
		return false, errors.New("failed to inspect existing Compose revision")
	}
	value := strings.Fields(output)
	if len(value) == 2 && value[0] == revision {
		if _, err := runner.Run("chmod 0600 " + ssh.ShellQuote(paths.file)); err != nil {
			return false, errors.New("failed to restrict existing Compose revision")
		}
		return false, nil
	}
	if strings.TrimSpace(output) != "MISSING" {
		return false, errors.New("existing Compose revision does not match the requested checksum")
	}
	if err := transferComposeFile(runner, project.LocalPath, paths.file, &transferOutcome{}); err != nil {
		return false, errors.New("failed to transfer Compose revision")
	}
	if _, err := runner.Run("chmod 0600 " + ssh.ShellQuote(paths.file)); err != nil {
		return false, errors.New("failed to restrict Compose revision permissions")
	}
	output, err = runner.Run("sha256sum " + ssh.ShellQuote(paths.file))
	if err != nil || len(strings.Fields(output)) != 2 || strings.Fields(output)[0] != revision {
		return false, errors.New("remote Compose revision checksum does not match")
	}
	return true, nil
}

func currentRevision(runner docker.RemoteRunner, paths revisionPaths) (string, error) {
	command := "if [ -L " + ssh.ShellQuote(paths.active) + " ]; then readlink " + ssh.ShellQuote(paths.active) + "; elif [ -e " + ssh.ShellQuote(paths.active) + " ]; then echo INVALID; else echo NONE; fi"
	output, err := runner.Run(command)
	if err != nil {
		return "", errors.New("cannot read active Compose revision")
	}
	value := strings.TrimSpace(output)
	if value == "NONE" {
		return "", nil
	}
	if !revisionLink.MatchString(value) {
		return "", errors.New("active Compose revision is not a Crosswalk-managed revision link")
	}
	return value, nil
}

func verifyRevisionChecksum(runner docker.RemoteRunner, file, revision string) error {
	output, err := runner.Run("sha256sum " + ssh.ShellQuote(file))
	fields := strings.Fields(output)
	if err != nil || len(fields) != 2 || fields[0] != revision {
		return errors.New("Compose revision checksum does not match its directory")
	}
	return nil
}

func switchRevision(runner docker.RemoteRunner, paths revisionPaths, target string) error {
	if !revisionLink.MatchString(target) {
		return errors.New("invalid Compose revision link target")
	}
	temporary := paths.active + ".next-" + strings.TrimPrefix(target, ".revisions/")
	command := "ln -s " + ssh.ShellQuote(target) + " " + ssh.ShellQuote(temporary) +
		" && mv -Tf " + ssh.ShellQuote(temporary) + " " + ssh.ShellQuote(paths.active)
	if _, err := runner.Run(command); err != nil {
		return fmt.Errorf("failed to switch active Compose revision to %s", target)
	}
	return nil
}
