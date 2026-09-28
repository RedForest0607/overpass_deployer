package compose

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
	name   string
	host   string
	status serverStatus
	err    error
}

func resultFor(server config.ServerConfig, changed bool, err error) serverResult {
	status := serverDone
	if err != nil {
		status = serverFailed
	} else if !changed {
		status = serverSkipped
	}
	return serverResult{name: server.Name, host: server.Host, status: status, err: err}
}

type deploymentSummary struct {
	results []serverResult
}

func (s deploymentSummary) err() error {
	var failures []error
	for _, result := range s.results {
		if result.status == serverFailed && result.err != nil {
			failures = append(failures, fmt.Errorf("%s (%s): %w", result.name, result.host, result.err))
		}
	}
	if len(failures) == 0 {
		return nil
	}
	if len(failures) == 1 {
		return failures[0]
	}
	return fmt.Errorf("%d Compose server operations failed: %w", len(failures), errors.Join(failures...))
}

func (s deploymentSummary) log() {
	logger.GlobalInfo("--- Compose deployment result summary ---")
	done, skipped, failed := 0, 0, 0
	for _, result := range s.results {
		switch result.status {
		case serverDone:
			done++
		case serverSkipped:
			skipped++
		case serverFailed:
			failed++
		}
		message := fmt.Sprintf("%s %s (%s)", result.status, result.name, result.host)
		if result.err != nil {
			message += ": " + result.err.Error()
		}
		logger.GlobalInfo("%s", message)
	}
	logger.GlobalInfo("Totals: done=%d skipped=%d failed=%d", done, skipped, failed)
}
