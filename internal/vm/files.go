package vm

import (
	"fmt"
	"os"
	"strings"

	"go-deployer/internal/config"
	"go-deployer/internal/scp"
	"go-deployer/internal/ssh"
	"go-deployer/internal/template"
	"go-deployer/pkg/logger"
)

// DeployJar는 앱 JAR 파일을 원격 실행 경로로 전송한다.
func DeployJar(client *ssh.Client, app *config.AppConfig, opts RunOptions, host string) error {
	return scp.Transfer(client, app.Jar.LocalPath, app.Jar.RemotePath, scp.TransferOptions{
		DryRun:  opts.DryRun,
		Host:    runnerHost(client, host),
		Session: opts.TransferSession,
	})
}

// DeployConfigFiles는 앱에 연결된 설정 파일들을 정규화된 원격 경로로 전송한다.
func DeployConfigFiles(client *ssh.Client, app *config.AppConfig, opts RunOptions, host string) error {
	for _, cf := range app.ConfigFiles {
		cf.Normalize()
		if err := scp.Transfer(client, cf.LocalPath, cf.RemotePath, scp.TransferOptions{
			DryRun:  opts.DryRun,
			Host:    runnerHost(client, host),
			Session: opts.TransferSession,
		}); err != nil {
			return fmt.Errorf("transferring config file %s: %w", cf.LocalPath, err)
		}
	}
	return nil
}

// DeployExtraFiles는 앱 단위 추가 파일을 전송하고 필요한 경우 chmod를 적용한다.
func DeployExtraFiles(client *ssh.Client, app *config.AppConfig, opts RunOptions, host string) error {
	return deployExtraFiles(client, app.ExtraFiles, opts, host)
}

// DeployServerExtraFiles는 특정 앱에 묶이지 않은 서버 레벨 추가 파일을 배포한다.
func DeployServerExtraFiles(client *ssh.Client, extraFiles []config.ExtraFile, opts RunOptions, host string) error {
	return deployExtraFiles(client, extraFiles, opts, host)
}

// deployExtraFiles는 공통 추가 파일 전송 로직과 전송 후 권한 변경을 처리한다.
func deployExtraFiles(client *ssh.Client, extraFiles []config.ExtraFile, opts RunOptions, host string) error {
	host = runnerHost(client, host)
	for _, ef := range extraFiles {
		if err := scp.Transfer(client, ef.LocalPath, ef.RemotePath, scp.TransferOptions{
			DryRun:  opts.DryRun,
			Host:    host,
			Session: opts.TransferSession,
		}); err != nil {
			return fmt.Errorf("transferring extra file %s: %w", ef.LocalPath, err)
		}

		if ef.Chmod != "" {
			if opts.DryRun {
				logger.Info(host, "DRY-RUN: would chmod %s %s", ef.Chmod, ef.RemotePath)
			} else if err := applyRemoteFileMode(client, ef.RemotePath, ef.Chmod); err != nil {
				return fmt.Errorf("chmod extra file %s: %w", ef.RemotePath, err)
			}
		}

		if !ef.Extract.Enabled {
			continue
		}

		if opts.DryRun {
			logger.Info(host, "DRY-RUN: would extract %s to %s", ef.RemotePath, ef.Extract.RemoteDir)
			continue
		}

		if err := applyRemoteArchiveExtraction(client, ef); err != nil {
			return fmt.Errorf("extract extra file %s: %w", ef.RemotePath, err)
		}
	}
	return nil
}

// applyRemoteFileMode는 원격 파일에 검증된 chmod 모드를 적용한다.
func applyRemoteFileMode(runner ssh.Runner, remotePath string, mode string) error {
	cmd := fmt.Sprintf("chmod %s %s", ssh.ShellQuote(mode), ssh.ShellQuote(remotePath))
	_, err := runner.Run(cmd)
	return err
}

// applyRemoteArchiveExtraction은 전송된 tar 파일을 지정된 원격 디렉터리에 압축 해제한다.
func applyRemoteArchiveExtraction(runner ssh.Runner, extraFile config.ExtraFile) error {
	setupCmd := buildExtractDirectorySetupCommand(extraFile.Extract.RemoteDir)
	if _, err := runner.RunSudo(setupCmd); err != nil {
		return fmt.Errorf("prepare extract directory %s: %w", extraFile.Extract.RemoteDir, err)
	}

	cmd, err := buildRemoteArchiveExtractionCommand(extraFile)
	if err != nil {
		return err
	}
	_, err = runner.Run(cmd)
	return err
}

// buildExtractDirectorySetupCommand는 압축 해제 대상 디렉터리만 준비한다.
// 하위 전체를 chown하지 않아 기존 파일 권한을 넓게 바꾸지 않는다.
func buildExtractDirectorySetupCommand(remoteDir string) string {
	quotedRemoteDir := ssh.ShellQuote(remoteDir)
	innerCommand := strings.Join([]string{
		"set -eu",
		fmt.Sprintf("base_dir=%s", quotedRemoteDir),
		`owner_name="${SUDO_USER:-$(id -un)}"`,
		`owner_group="$(id -gn "${owner_name}")"`,
		`mkdir -p "${base_dir}"`,
		`chown "${owner_name}:${owner_group}" "${base_dir}"`,
	}, "; ")

	return "sh -lc " + ssh.ShellQuote(innerCommand)
}

// buildRemoteArchiveExtractionCommand는 지원하는 tar 계열 아카이브만 안전하게 압축 해제하는 명령을 만든다.
func buildRemoteArchiveExtractionCommand(extraFile config.ExtraFile) (string, error) {
	flags, err := tarExtractionFlags(extraFile.RemotePath)
	if err != nil {
		return "", err
	}

	parts := []string{
		fmt.Sprintf("tar %s %s -C %s",
			flags,
			ssh.ShellQuote(extraFile.RemotePath),
			ssh.ShellQuote(extraFile.Extract.RemoteDir),
		),
	}

	if extraFile.Extract.StripComponents > 0 {
		parts[0] += fmt.Sprintf(" --strip-components=%d", extraFile.Extract.StripComponents)
	}

	return strings.Join(parts, " && "), nil
}

// tarExtractionFlags는 파일 확장자에 맞는 tar 압축 해제 플래그를 선택한다.
func tarExtractionFlags(remotePath string) (string, error) {
	lower := strings.ToLower(strings.TrimSpace(remotePath))
	switch {
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return "-xzf", nil
	case strings.HasSuffix(lower, ".tar"):
		return "-xf", nil
	default:
		return "", fmt.Errorf("unsupported archive format %q", remotePath)
	}
}

// DeployScripts는 템플릿 렌더링 또는 로컬 파일 기준으로 서버 실행 스크립트를 배포한다.
func DeployScripts(client *ssh.Client, app *config.AppConfig, opts RunOptions, host string) error {
	host = runnerHost(client, host)
	app.Script.Normalize(app.BaseDir)
	serverPath := app.Script.RemotePath
	scriptSource, description, cleanup, err := prepareScriptSource(app)
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer cleanup()
	}

	if opts.DryRun {
		logger.Info(host, "DRY-RUN: would deploy %s to %s", description, serverPath)
		logger.Info(host, "DRY-RUN: would chmod +x %s", serverPath)
		return nil
	}

	if err := scp.Transfer(client, scriptSource, serverPath, scp.TransferOptions{
		Session: opts.TransferSession,
	}); err != nil {
		return fmt.Errorf("transferring server.sh: %w", err)
	}

	logger.Info(host, "Making scripts executable...")
	cmd := fmt.Sprintf("chmod +x %s", ssh.ShellQuote(serverPath))
	if _, err := client.Run(cmd); err != nil {
		return fmt.Errorf("chmod scripts: %w", err)
	}
	logger.Ok(host, "Scripts deployed")

	return nil
}

// prepareScriptSource는 스크립트 모드에 따라 전송할 로컬 파일 경로와 임시 파일 정리 함수를 준비한다.
func prepareScriptSource(app *config.AppConfig) (localPath string, description string, cleanup func(), err error) {
	switch app.Script.Mode {
	case "", config.ScriptModeTemplate:
		scriptData, renderErr := resolveScriptTemplateData(app)
		if renderErr != nil {
			return "", "", nil, fmt.Errorf("resolving script template data: %w", renderErr)
		}

		rendered, renderErr := template.Render(app.Script.Template, "server.sh", scriptData)
		if renderErr != nil {
			return "", "", nil, fmt.Errorf("rendering server.sh: %w", renderErr)
		}

		description = app.Script.Template
		if description == "" {
			description = "embedded:server.sh.tmpl"
		}
		return rendered, description, func() {
			_ = os.Remove(rendered)
		}, nil

	case config.ScriptModeLocalFile:
		return app.Script.LocalPath, app.Script.LocalPath, nil, nil

	default:
		return "", "", nil, fmt.Errorf("unsupported script mode %q", app.Script.Mode)
	}
}

// resolveScriptTemplateData는 앱 기본 템플릿 데이터와 values 파일 override를 병합한다.
func resolveScriptTemplateData(app *config.AppConfig) (map[string]any, error) {
	baseData := app.ToTemplateData()
	if app.Script.ValuesFile == "" {
		return baseData, nil
	}

	overrideData, err := template.LoadTemplateData(app.Script.ValuesFile)
	if err != nil {
		return nil, err
	}

	return template.MergeTemplateData(baseData, overrideData), nil
}

// runnerHost는 dry-run이나 테스트처럼 runner가 없을 때도 로그에 표시할 호스트명을 안전하게 결정한다.
func runnerHost(runner ssh.Runner, fallback string) string {
	if client, ok := runner.(*ssh.Client); ok {
		if client != nil {
			return client.Host()
		}
		return fallback
	}
	if runner != nil {
		return runner.Host()
	}
	return fallback
}

// actionLabel은 dry-run과 실제 실행에 맞는 진행 로그 동사를 만든다.
func actionLabel(opts RunOptions, action string) string {
	if opts.DryRun {
		return "Planning " + action
	}
	return titleCase(action)
}

// resultLabel은 dry-run과 실제 실행에 맞는 완료 로그 표현을 만든다.
func resultLabel(opts RunOptions, actual string, planned string) string {
	if opts.DryRun {
		return titleCase(planned)
	}
	return titleCase(actual)
}

// titleCase는 로그 메시지 첫 글자를 대문자로 보정한다.
func titleCase(value string) string {
	if value == "" {
		return ""
	}
	return strings.ToUpper(value[:1]) + value[1:]
}
