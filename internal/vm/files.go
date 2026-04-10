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

func DeployJar(client *ssh.Client, app *config.AppConfig, opts RunOptions, host string) error {
	return scp.Transfer(client, app.Jar.LocalPath, app.Jar.RemotePath, scp.TransferOptions{
		DryRun: opts.DryRun,
		Host:   runnerHost(client, host),
	})
}

func DeployConfigFiles(client *ssh.Client, app *config.AppConfig, opts RunOptions, host string) error {
	for _, cf := range app.ConfigFiles {
		cf.Normalize()
		if err := scp.Transfer(client, cf.LocalPath, cf.RemotePath, scp.TransferOptions{
			DryRun: opts.DryRun,
			Host:   runnerHost(client, host),
		}); err != nil {
			return fmt.Errorf("transferring config file %s: %w", cf.LocalPath, err)
		}
	}
	return nil
}

func DeployExtraFiles(client *ssh.Client, app *config.AppConfig, opts RunOptions, host string) error {
	return deployExtraFiles(client, app.ExtraFiles, opts, host)
}

func DeployServerExtraFiles(client *ssh.Client, extraFiles []config.ExtraFile, opts RunOptions, host string) error {
	return deployExtraFiles(client, extraFiles, opts, host)
}

func deployExtraFiles(client *ssh.Client, extraFiles []config.ExtraFile, opts RunOptions, host string) error {
	host = runnerHost(client, host)
	for _, ef := range extraFiles {
		if err := scp.Transfer(client, ef.LocalPath, ef.RemotePath, scp.TransferOptions{
			DryRun: opts.DryRun,
			Host:   host,
		}); err != nil {
			return fmt.Errorf("transferring extra file %s: %w", ef.LocalPath, err)
		}

		if ef.Chmod == "" {
			continue
		}

		if opts.DryRun {
			logger.Info(host, "DRY-RUN: would chmod %s %s", ef.Chmod, ef.RemotePath)
			continue
		}

		if err := applyRemoteFileMode(client, ef.RemotePath, ef.Chmod); err != nil {
			return fmt.Errorf("chmod extra file %s: %w", ef.RemotePath, err)
		}
	}
	return nil
}

func applyRemoteFileMode(runner ssh.Runner, remotePath string, mode string) error {
	cmd := fmt.Sprintf("chmod %s %s", ssh.ShellQuote(mode), ssh.ShellQuote(remotePath))
	_, err := runner.Run(cmd)
	return err
}

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

	if err := scp.Transfer(client, scriptSource, serverPath, scp.TransferOptions{}); err != nil {
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

func actionLabel(opts RunOptions, action string) string {
	if opts.DryRun {
		return "Planning " + action
	}
	return titleCase(action)
}

func resultLabel(opts RunOptions, actual string, planned string) string {
	if opts.DryRun {
		return titleCase(planned)
	}
	return titleCase(actual)
}

func titleCase(value string) string {
	if value == "" {
		return ""
	}
	return strings.ToUpper(value[:1]) + value[1:]
}
