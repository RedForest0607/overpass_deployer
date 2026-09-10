package vm

import (
	"errors"
	"fmt"

	"go-deployer/internal/config"
	"go-deployer/pkg/logger"
)

type serverResultStatus string

const (
	serverResultDone    serverResultStatus = "done"
	serverResultSkipped serverResultStatus = "skipped"
	serverResultFailed  serverResultStatus = "failed"
)

type serverResult struct {
	Name   string
	Host   string
	Status serverResultStatus
	Err    error
}

func newServerResult(server config.ServerConfig) serverResult {
	return serverResult{Name: server.Name, Host: server.Host, Status: serverResultDone}
}

type deploymentSummary struct {
	results []serverResult
}

func (s *deploymentSummary) add(result serverResult) {
	s.results = append(s.results, result)
}

func (s deploymentSummary) hasFailures() bool {
	for _, result := range s.results {
		if result.Status == serverResultFailed {
			return true
		}
	}
	return false
}

func (s deploymentSummary) err() error {
	var failures []error
	for _, result := range s.results {
		if result.Status != serverResultFailed || result.Err == nil {
			continue
		}
		failures = append(failures, fmt.Errorf("%s (%s): %w", result.Name, result.Host, result.Err))
	}
	if len(failures) == 0 {
		return nil
	}
	if len(failures) == 1 {
		return errors.Unwrap(failures[0])
	}
	return fmt.Errorf("%d server deployments failed: %w", len(failures), errors.Join(failures...))
}

// log는 배포 종료 시 서버 단위 결과와 집계를 한곳에 출력한다.
func (s deploymentSummary) log() {
	logger.GlobalInfo("--- Deployment result summary ---")
	done, skipped, failed := 0, 0, 0
	for _, result := range s.results {
		switch result.Status {
		case serverResultDone:
			done++
		case serverResultSkipped:
			skipped++
		case serverResultFailed:
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
