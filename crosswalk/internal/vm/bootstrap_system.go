package vm

import (
	"fmt"
	"go-deployer/internal/config"
	"go-deployer/internal/ssh"
	"go-deployer/pkg/logger"
	"strings"
)

func applyTimezoneBootstrap(runner ssh.Runner, timezone config.TimezoneConfig, host string) error {
	if timezone.Name == "" {
		return nil
	}

	current, err := runner.Run("timedatectl show -p Timezone --value")
	if err != nil {
		return fmt.Errorf("checking timezone: %w", err)
	}
	if strings.TrimSpace(current) == timezone.Name {
		logger.Skip(host, "Timezone already set to %s", timezone.Name)
		return nil
	}

	logger.Info(host, "Setting timezone to %s...", timezone.Name)
	if _, err := runner.RunSudo("timedatectl set-timezone " + ssh.ShellQuote(timezone.Name)); err != nil {
		return fmt.Errorf("setting timezone: %w", err)
	}
	logger.Ok(host, "Set timezone to %s", timezone.Name)
	return nil
}

func applySwapBootstrap(runner ssh.Runner, swap config.SwapConfig, host string) error {
	if swap.Enabled == nil || !*swap.Enabled {
		return nil
	}

	active, err := runner.Run("swapon --noheadings --show=NAME")
	if err != nil {
		return fmt.Errorf("checking active swap: %w", err)
	}
	if containsLine(active, swap.Path) {
		logger.Skip(host, "Swap already active at %s", swap.Path)
		return ensureSwapInFstab(runner, swap.Path)
	}

	logger.Info(host, "Ensuring swap file %s (%s)...", swap.Path, swap.Size)
	if _, err := runner.RunSudo(buildSwapEnsureCommand(swap)); err != nil {
		activeAfter, checkErr := runner.Run("swapon --noheadings --show=NAME")
		if checkErr != nil || !containsLine(activeAfter, swap.Path) {
			return fmt.Errorf("enabling swap file: %w", err)
		}
		logger.Skip(host, "Swap already active at %s", swap.Path)
	}
	if err := ensureSwapInFstab(runner, swap.Path); err != nil {
		return err
	}

	logger.Ok(host, "Enabled swap file %s", swap.Path)
	return nil
}

func buildSwapEnsureCommand(swap config.SwapConfig) string {
	quotedPath := ssh.ShellQuote(swap.Path)
	quotedSize := ssh.ShellQuote(swap.Size)
	innerCommand := strings.Join([]string{
		"set -eu",
		fmt.Sprintf("swap_path=%s", quotedPath),
		fmt.Sprintf("swap_size=%s", quotedSize),
		`if [ ! -f "${swap_path}" ]; then fallocate -l "${swap_size}" "${swap_path}"; fi`,
		`chmod 600 "${swap_path}"`,
		`if ! file -s "${swap_path}" | grep -qi 'swap file'; then mkswap "${swap_path}"; fi`,
		`swapon "${swap_path}" || swapon --noheadings --show=NAME | grep -Fxq "${swap_path}"`,
	}, "; ")

	return "sh -lc " + ssh.ShellQuote(innerCommand)
}

func ensureSwapInFstab(runner ssh.Runner, path string) error {
	line := path + " none swap sw 0 0"
	command := "sh -lc " + ssh.ShellQuote("grep -Fxq "+ssh.ShellQuote(line)+" /etc/fstab || printf '%s\n' "+ssh.ShellQuote(line)+" >> /etc/fstab")
	if _, err := runner.RunSudo(command); err != nil {
		return fmt.Errorf("ensuring swap fstab entry: %w", err)
	}
	return nil
}
