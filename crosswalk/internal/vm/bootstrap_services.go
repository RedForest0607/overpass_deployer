package vm

import (
	"fmt"
	"go-deployer/internal/config"
	"go-deployer/internal/ssh"
	"go-deployer/pkg/logger"
	"strings"
)

func applyServiceBootstrap(runner ssh.Runner, services []config.ServiceConfig, host string) error {
	for _, service := range services {
		if err := applySingleServiceBootstrap(runner, service, host); err != nil {
			return err
		}
	}
	return nil
}

func applySingleServiceBootstrap(runner ssh.Runner, service config.ServiceConfig, host string) error {
	if service.Name == "" {
		return nil
	}

	if serviceEnabled(service) {
		if out, err := runner.Run("systemctl is-enabled " + ssh.ShellQuote(service.Name)); err != nil || strings.TrimSpace(out) != "enabled" {
			logger.Info(host, "Enabling service %s...", service.Name)
			if _, err := runner.RunSudo("systemctl enable " + ssh.ShellQuote(service.Name)); err != nil {
				return fmt.Errorf("enabling service %s: %w", service.Name, err)
			}
		} else {
			logger.Skip(host, "Service %s already enabled", service.Name)
		}
	} else {
		if out, err := runner.Run("systemctl is-enabled " + ssh.ShellQuote(service.Name)); err == nil && strings.TrimSpace(out) == "enabled" {
			logger.Info(host, "Disabling service %s...", service.Name)
			if _, err := runner.RunSudo("systemctl disable " + ssh.ShellQuote(service.Name)); err != nil {
				return fmt.Errorf("disabling service %s: %w", service.Name, err)
			}
		} else {
			logger.Skip(host, "Service %s already disabled", service.Name)
		}
	}

	switch serviceState(service) {
	case config.ServiceStateStarted:
		if out, err := runner.Run("systemctl is-active " + ssh.ShellQuote(service.Name)); err == nil && strings.TrimSpace(out) == "active" {
			logger.Skip(host, "Service %s already active", service.Name)
			return nil
		}
		logger.Info(host, "Starting service %s...", service.Name)
		if _, err := runner.RunSudo("systemctl start " + ssh.ShellQuote(service.Name)); err != nil {
			return fmt.Errorf("starting service %s: %w", service.Name, err)
		}
		logger.Ok(host, "Started service %s", service.Name)
	case config.ServiceStateStopped:
		if out, err := runner.Run("systemctl is-active " + ssh.ShellQuote(service.Name)); err == nil && strings.TrimSpace(out) == "inactive" {
			logger.Skip(host, "Service %s already inactive", service.Name)
			return nil
		}
		logger.Info(host, "Stopping service %s...", service.Name)
		if _, err := runner.RunSudo("systemctl stop " + ssh.ShellQuote(service.Name)); err != nil {
			return fmt.Errorf("stopping service %s: %w", service.Name, err)
		}
		logger.Ok(host, "Stopped service %s", service.Name)
	case config.ServiceStateRestarted:
		logger.Info(host, "Restarting service %s...", service.Name)
		if _, err := runner.RunSudo("systemctl restart " + ssh.ShellQuote(service.Name)); err != nil {
			return fmt.Errorf("restarting service %s: %w", service.Name, err)
		}
		logger.Ok(host, "Restarted service %s", service.Name)
	}

	return nil
}

func serviceEnabled(service config.ServiceConfig) bool {
	return service.Enabled == nil || *service.Enabled
}

func serviceState(service config.ServiceConfig) string {
	if service.State == "" {
		if !serviceEnabled(service) {
			return config.ServiceStateStopped
		}
		return config.ServiceStateStarted
	}
	return service.State
}
