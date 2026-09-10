package vm

import (
	"fmt"
	"testing"

	"go-deployer/internal/config"
)

func TestExistingDirectoryOwnershipIsReconciled(t *testing.T) {
	for _, correct := range []bool{false, true} {
		t.Run(fmt.Sprint(correct), func(t *testing.T) {
			owner := "root:root"
			if correct {
				owner = "deploy:deploy"
			}
			r := &bootstrapRunner{runResults: map[string]commandResult{
				"test -d '/srv/app'":         {},
				"stat -c '%U:%G' '/srv/app'": {output: owner},
			}}
			o := newOperationOutcomes(false)
			err := CreateServerDirectories(r, []string{"/srv/app"}, RunOptions{outcomes: o, DirectoryRules: []config.DirectoryOwnershipRule{{Prefix: "/srv/app", Owner: "deploy", Group: "deploy"}}}, "test")
			if err != nil {
				t.Fatal(err)
			}
			if correct {
				if len(r.sudoCommands) != 0 || o.status() != serverResultSkipped {
					t.Fatal("unchanged ownership must skip")
				}
			} else if len(r.sudoCommands) != 1 || r.sudoCommands[0] != "chown 'deploy:deploy' '/srv/app'" || o.status() != serverResultDone {
				t.Fatalf("ownership correction missing: %v, %#v", r.sudoCommands, o)
			}
		})
	}
}

func TestBootstrapMutationOverridesSkippedFiles(t *testing.T) {
	r := &bootstrapRunner{runResults: map[string]commandResult{
		packageDetectCommandForTest(packageManagerDNF): {},
		"rpm -q 'git'": {err: fmt.Errorf("missing")},
	}}
	o := newOperationOutcomes(false)
	o.skippedOperation()
	if err := BootstrapHost(r, config.BootstrapConfig{Packages: []string{"git"}}, RunOptions{outcomes: o}, "test"); err != nil {
		t.Fatal(err)
	}
	if o.status() != serverResultDone {
		t.Fatal("package installation reported skipped")
	}
}

func TestServiceMutationOverridesSkippedFiles(t *testing.T) {
	r := &bootstrapRunner{runResults: map[string]commandResult{"systemctl is-enabled 'redis'": {output: "enabled"}}}
	o := newOperationOutcomes(false)
	o.skippedOperation()
	if err := applyServiceBootstrap(mutationTrackingRunner{Runner: r, outcomes: o}, []config.ServiceConfig{{Name: "redis", State: config.ServiceStateRestarted}}, "test"); err != nil {
		t.Fatal(err)
	}
	if o.status() != serverResultDone {
		t.Fatal("service restart reported skipped")
	}
}

func TestFilePermissionChangesAreTracked(t *testing.T) {
	for _, current := range []string{"644", "755"} {
		r := &bootstrapRunner{runResults: map[string]commandResult{
			"stat -c '%a' '/srv/start.sh'": {output: current},
			"chmod '755' '/srv/start.sh'":  {},
		}}
		o := newOperationOutcomes(false)
		if err := ensureRemoteFileMode(r, "/srv/start.sh", "+x", o); err != nil {
			t.Fatal(err)
		}
		if (o.status() == serverResultSkipped) != (current == "755") {
			t.Fatalf("incorrect outcome for mode %s", current)
		}
	}
}

func TestFailedMutationDoesNotCountAsCompletedWork(t *testing.T) {
	r := &bootstrapRunner{sudoResults: map[string]commandResult{"systemctl restart 'redis'": {err: fmt.Errorf("denied")}}}
	o := newOperationOutcomes(false)
	_, err := (mutationTrackingRunner{Runner: r, outcomes: o}).RunSudo("systemctl restart 'redis'")
	if err == nil || o.changed != 0 {
		t.Fatalf("failed operation counted as changed: %v, %#v", err, o)
	}
}
