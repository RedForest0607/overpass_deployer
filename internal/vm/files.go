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
	scriptData := app.ToScriptData()

	startPath := filepath.ToSlash(filepath.Join(app.Script.RemoteDir, "start.sh"))
	stopPath := filepath.ToSlash(filepath.Join(app.Script.RemoteDir, "stop.sh"))

	if opts.DryRun {
		startTemplate := app.Script.Template
		if startTemplate == "" {
			startTemplate = "embedded:start.sh.tmpl"
		}
		logger.Info(host, "DRY-RUN: would render %s to %s", startTemplate, startPath)
		logger.Info(host, "DRY-RUN: would render embedded:stop.sh.tmpl to %s", stopPath)
		logger.Info(host, "DRY-RUN: would chmod +x %s %s", startPath, stopPath)
		return nil
	}

	startLocal, err := template.Render(app.Script.Template, "start.sh", scriptData)
	if err != nil {
		return fmt.Errorf("rendering start.sh: %w", err)
	}
	defer os.Remove(startLocal)

	if err := scp.Transfer(client, startLocal, startPath, scp.TransferOptions{}); err != nil {
		return fmt.Errorf("transferring start.sh: %w", err)
	}

	// Always use embedded template for stop.sh in M1
	stopLocal, err := template.Render("", "stop.sh", scriptData)
	if err != nil {
		return fmt.Errorf("rendering stop.sh: %w", err)
	}
	defer os.Remove(stopLocal)

	if err := scp.Transfer(client, stopLocal, stopPath, scp.TransferOptions{}); err != nil {
		return fmt.Errorf("transferring stop.sh: %w", err)
	}

	logger.Info(host, "Making scripts executable...")
	cmd := fmt.Sprintf("chmod +x %s %s", ssh.ShellQuote(startPath), ssh.ShellQuote(stopPath))
	if _, err := client.Run(cmd); err != nil {
		return fmt.Errorf("chmod scripts: %w", err)
	}
	logger.Ok(host, "Scripts deployed")

	return nil
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
