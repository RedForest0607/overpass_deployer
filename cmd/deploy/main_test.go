package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"go-deployer/internal/buildinfo"
	"go-deployer/internal/config"
	"go-deployer/internal/update"
	"go-deployer/internal/vm"
)

func TestRunVMRoutesDryRunOption(t *testing.T) {
	t.Helper()

	var gotPath string
	var gotOpts vm.RunOptions

	exitCode := run([]string{"vm", "--config", "sample.yml", "--dry-run"}, dependencies{
		stdout:    &bytes.Buffer{},
		stderr:    &bytes.Buffer{},
		buildInfo: func() buildinfo.Info { return buildinfo.Current() },
		loadConfig: func(path string) (*config.Config, error) {
			gotPath = path
			return &config.Config{}, nil
		},
		runVM: func(cfg *config.Config, opts vm.RunOptions) error {
			gotOpts = opts
			return nil
		},
		runUpdate: func(ctx context.Context, cfg update.Config, opts update.Options) (*update.Result, error) {
			return nil, nil
		},
	})

	if exitCode != 0 {
		t.Fatalf("expected success exit code, got %d", exitCode)
	}
	if gotPath != "sample.yml" {
		t.Fatalf("expected config path sample.yml, got %q", gotPath)
	}
	if !gotOpts.DryRun {
		t.Fatalf("expected dry-run option to be true")
	}
}

func TestRunVMRoutesTagFilters(t *testing.T) {
	t.Helper()

	var gotOpts vm.RunOptions

	exitCode := run([]string{"vm", "--config", "sample.yml", "--server-tag", "wave1, apm", "--app-tag", "fo, api"}, dependencies{
		stdout:    &bytes.Buffer{},
		stderr:    &bytes.Buffer{},
		buildInfo: func() buildinfo.Info { return buildinfo.Current() },
		loadConfig: func(path string) (*config.Config, error) {
			return &config.Config{}, nil
		},
		runVM: func(cfg *config.Config, opts vm.RunOptions) error {
			gotOpts = opts
			return nil
		},
		runUpdate: func(ctx context.Context, cfg update.Config, opts update.Options) (*update.Result, error) {
			return nil, nil
		},
	})

	if exitCode != 0 {
		t.Fatalf("expected success exit code, got %d", exitCode)
	}
	if gotOpts.ServerTags[0] != "wave1" || gotOpts.ServerTags[1] != "apm" {
		t.Fatalf("expected normalized server tags, got %v", gotOpts.ServerTags)
	}
	if gotOpts.AppTags[0] != "fo" || gotOpts.AppTags[1] != "api" {
		t.Fatalf("expected normalized app tags, got %v", gotOpts.AppTags)
	}
}

func TestRunRejectsUnknownSubcommand(t *testing.T) {
	t.Helper()

	stderr := &bytes.Buffer{}
	exitCode := run([]string{"unknown"}, dependencies{
		stdout:     &bytes.Buffer{},
		stderr:     stderr,
		buildInfo:  func() buildinfo.Info { return buildinfo.Current() },
		loadConfig: func(path string) (*config.Config, error) { return nil, nil },
		runVM:      func(cfg *config.Config, opts vm.RunOptions) error { return nil },
		runUpdate: func(ctx context.Context, cfg update.Config, opts update.Options) (*update.Result, error) {
			return nil, nil
		},
	})

	if exitCode != 1 {
		t.Fatalf("expected failure exit code, got %d", exitCode)
	}
	if stderr.Len() == 0 {
		t.Fatalf("expected usage output on stderr")
	}
}

func TestRunVersionPrintsBuildMetadata(t *testing.T) {
	t.Helper()

	stdout := &bytes.Buffer{}
	exitCode := run([]string{"version"}, dependencies{
		stdout: stdout,
		stderr: &bytes.Buffer{},
		buildInfo: func() buildinfo.Info {
			return buildinfo.Info{
				Version:   "v1.2.3",
				Commit:    "abc123",
				Date:      "2026-04-10T00:00:00Z",
				BuiltBy:   "goreleaser",
				RepoOwner: "acme",
				RepoName:  "overpassDeployer",
				GOOS:      "linux",
				GOARCH:    "amd64",
			}
		},
		loadConfig: func(path string) (*config.Config, error) { return nil, nil },
		runVM:      func(cfg *config.Config, opts vm.RunOptions) error { return nil },
		runUpdate: func(ctx context.Context, cfg update.Config, opts update.Options) (*update.Result, error) {
			return nil, nil
		},
	})

	if exitCode != 0 {
		t.Fatalf("expected success exit code, got %d", exitCode)
	}
	if !strings.Contains(stdout.String(), "version: v1.2.3") {
		t.Fatalf("expected version output, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "repository: acme/overpassDeployer") {
		t.Fatalf("expected repository output, got %q", stdout.String())
	}
}

func TestRunUpdateRoutesCheckOnlyOption(t *testing.T) {
	t.Helper()

	var gotCfg update.Config
	var gotOpts update.Options

	exitCode := run([]string{"update", "--check", "--version", "v1.2.3"}, dependencies{
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		buildInfo: func() buildinfo.Info {
			return buildinfo.Info{
				Version:       "v1.2.2",
				RepoOwner:     "acme",
				RepoName:      "overpassDeployer",
				GitHubAPIBase: "https://api.github.com",
			}
		},
		loadConfig: func(path string) (*config.Config, error) { return nil, nil },
		runVM:      func(cfg *config.Config, opts vm.RunOptions) error { return nil },
		runUpdate: func(ctx context.Context, cfg update.Config, opts update.Options) (*update.Result, error) {
			gotCfg = cfg
			gotOpts = opts
			return &update.Result{
				CurrentVersion: "v1.2.2",
				TargetVersion:  "v1.2.3",
				AssetName:      "deploy_1.2.3_linux_amd64.tar.gz",
				ReleaseURL:     "https://example.com/releases/v1.2.3",
			}, nil
		},
	})

	if exitCode != 0 {
		t.Fatalf("expected success exit code, got %d", exitCode)
	}
	if !gotOpts.CheckOnly {
		t.Fatalf("expected check-only option to be true")
	}
	if gotOpts.TargetVersion != "v1.2.3" {
		t.Fatalf("expected target version to be propagated, got %q", gotOpts.TargetVersion)
	}
	if gotCfg.RepoOwner != "acme" || gotCfg.RepoName != "overpassDeployer" {
		t.Fatalf("expected repository coordinates to be propagated, got %#v", gotCfg)
	}
}

func TestRunUpdateReturnsFailureOnUpdaterError(t *testing.T) {
	t.Helper()

	stderr := &bytes.Buffer{}
	exitCode := run([]string{"update"}, dependencies{
		stdout: &bytes.Buffer{},
		stderr: stderr,
		buildInfo: func() buildinfo.Info {
			return buildinfo.Info{
				Version:       "v1.2.2",
				RepoOwner:     "acme",
				RepoName:      "overpassDeployer",
				GitHubAPIBase: "https://api.github.com",
			}
		},
		loadConfig: func(path string) (*config.Config, error) { return nil, nil },
		runVM:      func(cfg *config.Config, opts vm.RunOptions) error { return nil },
		runUpdate: func(ctx context.Context, cfg update.Config, opts update.Options) (*update.Result, error) {
			return nil, errors.New("boom")
		},
	})

	if exitCode != 1 {
		t.Fatalf("expected failure exit code, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "Update failed: boom") {
		t.Fatalf("expected update error output, got %q", stderr.String())
	}
}
