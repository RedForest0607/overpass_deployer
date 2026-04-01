package vm

import (
	"fmt"
	"os"
	"path/filepath"

	"go-deployer/internal/config"
	"go-deployer/internal/scp"
	"go-deployer/internal/ssh"
	"go-deployer/internal/template"
	"go-deployer/pkg/logger"
)

func DeployJar(client *ssh.Client, app *config.AppConfig) error {
	return scp.Transfer(client, app.Jar.LocalPath, app.Jar.RemotePath)
}

func DeployConfigFiles(client *ssh.Client, app *config.AppConfig) error {
	for _, cf := range app.ConfigFiles {
		if err := scp.Transfer(client, cf.Local, cf.Remote); err != nil {
			return fmt.Errorf("transferring config file %s: %w", cf.Local, err)
		}
	}
	return nil
}

func DeployScripts(client *ssh.Client, app *config.AppConfig) error {
	scriptData := app.ToScriptData()

	startPath := filepath.ToSlash(filepath.Join(app.Script.RemoteDir, "start.sh"))
	stopPath := filepath.ToSlash(filepath.Join(app.Script.RemoteDir, "stop.sh"))

	startLocal, err := template.Render(app.Script.Template, "start.sh", scriptData)
	if err != nil {
		return fmt.Errorf("rendering start.sh: %w", err)
	}
	defer os.Remove(startLocal)

	if err := scp.Transfer(client, startLocal, startPath); err != nil {
		return fmt.Errorf("transferring start.sh: %w", err)
	}

	// Always use embedded template for stop.sh in M1
	stopLocal, err := template.Render("", "stop.sh", scriptData)
	if err != nil {
		return fmt.Errorf("rendering stop.sh: %w", err)
	}
	defer os.Remove(stopLocal)

	if err := scp.Transfer(client, stopLocal, stopPath); err != nil {
		return fmt.Errorf("transferring stop.sh: %w", err)
	}

	host := client.Host()
	logger.Info(host, "Making scripts executable...")
	cmd := fmt.Sprintf("chmod +x %s %s", ssh.ShellQuote(startPath), ssh.ShellQuote(stopPath))
	if _, err := client.Run(cmd); err != nil {
		return fmt.Errorf("chmod scripts: %w", err)
	}
	logger.Ok(host, "Scripts deployed")

	return nil
}
