package docker

import (
	"errors"
	"fmt"

	"go-deployer/internal/config"
	"go-deployer/pkg/logger"
)

type serverStatus string

const (
	serverDone    serverStatus = "done"
	serverSkipped serverStatus = "skipped"
	serverFailed  serverStatus = "failed"
)

type serverResult struct {
	Name   string
	Host   string
	Status serverStatus
	Err    error
}

func resultFor(server config.ServerConfig, changed bool, err error) serverResult {
	result := serverResult{Name: server.Name, Host: server.Host, Status: serverDone, Err: err}
	if err != nil {
		result.Status = serverFailed
	} else if !changed {
		result.Status = serverSkipped
	}
	return result
}

type deploymentSummary struct {
	results []serverResult
}

func (s deploymentSummary) err() error {
	var failures []error
	for _, result := range s.results {
		if result.Status == serverFailed && result.Err != nil {
			failures = append(failures, fmt.Errorf("%s (%s): %w", result.Name, result.Host, result.Err))
		}
	}
	if len(failures) == 0 {
		return nil
	}
	if len(failures) == 1 {
		return failures[0]
	}
	return fmt.Errorf("%d Docker server operations failed: %w", len(failures), errors.Join(failures...))
}

func (s deploymentSummary) log() {
	logger.GlobalInfo("--- Docker deployment result summary ---")
	done, skipped, failed := 0, 0, 0
	for _, result := range s.results {
		switch result.Status {
		case serverDone:
			done++
		case serverSkipped:
			skipped++
		case serverFailed:
			failed++
		}
		message := fmt.Sprintf("%s %s (%s)", result.Status, result.Name, result.Host)
		if result.Err != nil {
			message += ": " + result.Err.Error()
		}
		logger.GlobalInfo("%s", message)
	}
	logger.GlobalInfo("Totals: done=%d skipped=%d failed=%d", done, skipped, failed)
}
