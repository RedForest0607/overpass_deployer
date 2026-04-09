package vm

import (
	"fmt"
	"os"
	"path/filepath"
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
		if err := scp.Transfer(client, cf.Local, cf.Remote, scp.TransferOptions{
			DryRun: opts.DryRun,
			Host:   runnerHost(client, host),
		}); err != nil {
			return fmt.Errorf("transferring config file %s: %w", cf.Local, err)
		}
	}
	return nil
}

func DeployScripts(client *ssh.Client, app *config.AppConfig, opts RunOptions, host string) error {
	host = runnerHost(client, host)
	scriptData, err := resolveScriptTemplateData(app)
	if err != nil {
		return fmt.Errorf("resolving script template data: %w", err)
	}

	serverPath := filepath.ToSlash(filepath.Join(app.Script.RemoteDir, "server.sh"))

	if opts.DryRun {
		serverTemplate := app.Script.Template
		if serverTemplate == "" {
			serverTemplate = "embedded:server.sh.tmpl"
		}
		logger.Info(host, "DRY-RUN: would render %s to %s", serverTemplate, serverPath)
		logger.Info(host, "DRY-RUN: would chmod +x %s", serverPath)
		return nil
	}

	serverLocal, err := template.Render(app.Script.Template, "server.sh", scriptData)
	if err != nil {
		return fmt.Errorf("rendering server.sh: %w", err)
	}
	defer os.Remove(serverLocal)

	if err := scp.Transfer(client, serverLocal, serverPath, scp.TransferOptions{}); err != nil {
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
