package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"go-deployer/internal/buildinfo"
	"go-deployer/internal/config"
	"go-deployer/internal/update"
	"go-deployer/internal/vm"
)

// main은 실제 OS 입출력과 구현체 의존성을 연결한 뒤 CLI 실행 결과를 종료 코드로 반환한다.
func main() {
	os.Exit(run(os.Args[1:], dependencies{
		stdout:     os.Stdout,
		stderr:     os.Stderr,
		loadConfig: config.Load,
		runVM:      vm.RunWithOptions,
		runUpdate:  update.Execute,
		buildInfo:  buildinfo.Current,
	}))
}

type dependencies struct {
	stdout     io.Writer
	stderr     io.Writer
	loadConfig func(path string) (*config.Config, error)
	runVM      func(cfg *config.Config, opts vm.RunOptions) error
	runUpdate  func(ctx context.Context, cfg update.Config, opts update.Options) (*update.Result, error)
	buildInfo  func() buildinfo.Info
}

// run은 최상위 서브커맨드를 해석하고 각 실행 경로로 라우팅한다.
func run(args []string, deps dependencies) int {
	if len(args) < 1 {
		printUsage(deps.stderr)
		return 1
	}

	switch args[0] {
	case "vm":
		return runVM(args[1:], deps)
	case "version":
		return runVersion(deps)
	case "update":
		return runUpdate(args[1:], deps)
	case "docker":
		fmt.Fprintln(deps.stderr, "docker mode is not implemented yet (M3)")
		return 1
	default:
		fmt.Fprintf(deps.stderr, "Unknown subcommand: %s\n", args[0])
		printUsage(deps.stderr)
		return 1
	}
}

// runVM은 VM 배포 옵션을 파싱하고 설정 로드 후 배포 워크플로를 실행한다.
func runVM(args []string, deps dependencies) int {
	vmCmd := flag.NewFlagSet("vm", flag.ContinueOnError)
	vmCmd.SetOutput(deps.stderr)

	configPath := vmCmd.String("config", "deploy.yml", "Path to deploy.yml configuration file")
	dryRun := vmCmd.Bool("dry-run", false, "Print planned actions without making remote changes")
	serverTags := vmCmd.String("server-tag", "", "Deploy only servers matching any provided tags (comma-separated)")
	appTags := vmCmd.String("app-tag", "", "Deploy only apps matching any provided tags (comma-separated)")
	parallelServers := vmCmd.Int("parallel", 1, "Maximum number of servers to deploy concurrently")

	if err := vmCmd.Parse(args); err != nil {
		return 1
	}

	cfg, err := deps.loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(deps.stderr, "Failed to load configuration: %v\n", err)
		return 1
	}

	if err := deps.runVM(cfg, vm.RunOptions{
		DryRun:          *dryRun,
		ServerTags:      parseTagFilter(*serverTags),
		AppTags:         parseTagFilter(*appTags),
		ParallelServers: *parallelServers,
	}); err != nil {
		fmt.Fprintf(deps.stderr, "Deployment failed: %v\n", err)
		return 1
	}

	return 0
}

// runVersion은 빌드 시점에 주입된 버전과 플랫폼 메타데이터를 출력한다.
func runVersion(deps dependencies) int {
	info := deps.buildInfo()

	fmt.Fprintf(deps.stdout, "version: %s\n", info.Version)
	fmt.Fprintf(deps.stdout, "commit: %s\n", info.Commit)
	fmt.Fprintf(deps.stdout, "built at: %s\n", info.Date)
	fmt.Fprintf(deps.stdout, "built by: %s\n", info.BuiltBy)
	fmt.Fprintf(deps.stdout, "platform: %s/%s\n", info.GOOS, info.GOARCH)
	fmt.Fprintf(deps.stdout, "repository: %s\n", info.Repository())

	return 0
}

// runUpdate는 업데이트 확인 또는 설치 옵션을 파싱하고 업데이트 서비스를 호출한다.
func runUpdate(args []string, deps dependencies) int {
	updateCmd := flag.NewFlagSet("update", flag.ContinueOnError)
	updateCmd.SetOutput(deps.stderr)

	checkOnly := updateCmd.Bool("check", false, "Check for updates without replacing the current binary")
	targetVersion := updateCmd.String("version", "", "Install a specific release tag instead of the latest release")

	if err := updateCmd.Parse(args); err != nil {
		return 1
	}

	info := deps.buildInfo()

	result, err := deps.runUpdate(context.Background(), update.Config{
		CurrentVersion: info.Version,
		RepoOwner:      info.RepoOwner,
		RepoName:       info.RepoName,
		GitHubAPIBase:  info.GitHubAPIBase,
	}, update.Options{
		CheckOnly:     *checkOnly,
		TargetVersion: *targetVersion,
	})
	if err != nil {
		fmt.Fprintf(deps.stderr, "Update failed: %v\n", err)
		return 1
	}

	if result.UpToDate {
		fmt.Fprintf(deps.stdout, "Already up to date (%s)\n", result.CurrentVersion)
		return 0
	}

	if *checkOnly {
		fmt.Fprintf(deps.stdout, "Update available: %s -> %s\n", result.CurrentVersion, result.TargetVersion)
		fmt.Fprintf(deps.stdout, "Asset: %s\n", result.AssetName)
		fmt.Fprintf(deps.stdout, "Release: %s\n", result.ReleaseURL)
		return 0
	}

	fmt.Fprintf(deps.stdout, "Updated deploy from %s to %s\n", result.CurrentVersion, result.TargetVersion)
	fmt.Fprintf(deps.stdout, "Executable: %s\n", result.ExecutablePath)

	return 0
}

// printUsage는 지원하는 서브커맨드와 주요 플래그를 사용자에게 안내한다.
func printUsage(w io.Writer) {
	fmt.Fprintf(w, "Usage: deploy <subcommand> [flags]\n\n")
	fmt.Fprintf(w, "Subcommands:\n")
	fmt.Fprintf(w, "  vm      Run VM mode deployment\n")
	fmt.Fprintf(w, "  version Print build metadata\n")
	fmt.Fprintf(w, "  update  Check for or install a newer release\n")
	fmt.Fprintf(w, "  docker  Not implemented yet (M3)\n\n")
	fmt.Fprintf(w, "Flags for 'vm':\n")
	fmt.Fprintf(w, "  --config string   Path to configuration file (default: deploy.yml)\n")
	fmt.Fprintf(w, "  --dry-run         Print planned actions without remote changes\n")
	fmt.Fprintf(w, "  --server-tag      Deploy only servers matching any provided tags (comma-separated)\n")
	fmt.Fprintf(w, "  --app-tag         Deploy only apps matching any provided tags (comma-separated)\n")
	fmt.Fprintf(w, "  --parallel int    Maximum number of servers to deploy concurrently (default: 1)\n")
	fmt.Fprintf(w, "\nFlags for 'update':\n")
	fmt.Fprintf(w, "  --check           Check for updates without replacing the current binary\n")
	fmt.Fprintf(w, "  --version string  Install a specific release tag\n")
}

// parseTagFilter는 쉼표로 구분된 태그 입력을 소문자 중복 없는 목록으로 정규화한다.
func parseTagFilter(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	seen := make(map[string]struct{})
	tags := make([]string, 0)
	for _, part := range strings.Split(raw, ",") {
		tag := strings.ToLower(strings.TrimSpace(part))
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		tags = append(tags, tag)
	}

	if len(tags) == 0 {
		return nil
	}

	return tags
}
