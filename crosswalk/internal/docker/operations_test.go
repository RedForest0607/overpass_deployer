package docker

import (
	"bytes"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"go-deployer/internal/config"
	"go-deployer/pkg/logger"
)

func TestFilterConfigSelectsServerAndContainerTags(t *testing.T) {
	cfg := &config.Config{Servers: []config.ServerConfig{
		{Name: "prod-a", Tags: []string{"prod", "wave1"}, Containers: []config.ContainerConfig{
			{Name: "api", Tags: []string{"api"}}, {Name: "worker", Tags: []string{"worker"}},
		}},
		{Name: "dev-a", Tags: []string{"dev"}, Containers: []config.ContainerConfig{{Name: "api-dev", Tags: []string{"api"}}}},
	}}

	filtered, err := filterConfig(cfg, Options{ServerTags: []string{"prod"}, ContainerTags: []string{"worker"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered.Servers) != 1 || filtered.Servers[0].Name != "prod-a" || len(filtered.Servers[0].Containers) != 1 || filtered.Servers[0].Containers[0].Name != "worker" {
		t.Fatalf("servers=%#v", filtered.Servers)
	}
	if len(cfg.Servers[0].Containers) != 2 {
		t.Fatal("filter must not mutate the source configuration")
	}
}

func TestFilterConfigRejectsNoMatches(t *testing.T) {
	cfg := &config.Config{Servers: []config.ServerConfig{{
		Name: "prod-a", Tags: []string{"prod"}, Containers: []config.ContainerConfig{{Name: "api", Tags: []string{"api"}}},
	}}}
	_, err := filterConfig(cfg, Options{ContainerTags: []string{"missing"}})
	if err == nil || !strings.Contains(err.Error(), "no Docker containers matched") {
		t.Fatalf("error=%v", err)
	}
}

func TestRunServersLimitsParallelismAndPreservesSummaryOrder(t *testing.T) {
	original := executeDockerServer
	defer func() { executeDockerServer = original }()
	var mutex sync.Mutex
	active, maximum := 0, 0
	executeDockerServer = func(_ *config.Config, server config.ServerConfig, _ string, _, _ bool) (bool, error) {
		mutex.Lock()
		active++
		if active > maximum {
			maximum = active
		}
		mutex.Unlock()
		time.Sleep(20 * time.Millisecond)
		mutex.Lock()
		active--
		mutex.Unlock()
		if server.Name == "server-2" {
			return false, errors.New("simulated failure")
		}
		return server.Name == "server-1", nil
	}
	cfg := &config.Config{Servers: []config.ServerConfig{
		{Name: "server-1", Host: "10.0.0.1"},
		{Name: "server-2", Host: "10.0.0.2"},
		{Name: "server-3", Host: "10.0.0.3"},
	}}

	summary := runServers(cfg, "token", Options{}, 2)
	if maximum != 2 {
		t.Fatalf("maximum parallelism=%d", maximum)
	}
	if len(summary.results) != 3 || summary.results[0].Status != serverDone || summary.results[1].Status != serverFailed || summary.results[2].Status != serverSkipped {
		t.Fatalf("results=%#v", summary.results)
	}
	if err := summary.err(); err == nil || !strings.Contains(err.Error(), "server-2") {
		t.Fatalf("error=%v", err)
	}
}

func TestDeploymentSummaryLogsStableTotals(t *testing.T) {
	summary := deploymentSummary{results: []serverResult{
		{Name: "done", Host: "1", Status: serverDone},
		{Name: "skip", Host: "2", Status: serverSkipped},
		{Name: "fail", Host: "3", Status: serverFailed, Err: errors.New("safe failure")},
	}}
	var output bytes.Buffer
	restore := logger.SetOutput(&output)
	defer restore()
	summary.log()
	if !strings.Contains(output.String(), "Totals: done=1 skipped=1 failed=1") {
		t.Fatalf("output=%q", output.String())
	}
}

func TestDeploymentSummaryAggregatesMultipleFailures(t *testing.T) {
	summary := deploymentSummary{results: []serverResult{
		{Name: "server-a", Host: "1", Status: serverFailed, Err: errors.New("first")},
		{Name: "server-b", Host: "2", Status: serverFailed, Err: errors.New("second")},
	}}
	err := summary.err()
	if err == nil || !strings.Contains(err.Error(), "2 Docker server operations failed") ||
		!strings.Contains(err.Error(), "server-a") || !strings.Contains(err.Error(), "server-b") {
		t.Fatalf("error=%v", err)
	}
}

func TestRunRejectsNegativeParallelismBeforeRemoteAccess(t *testing.T) {
	err := Run(dockerConfig(), Options{DryRun: true, ParallelServers: -1})
	if err == nil || !strings.Contains(err.Error(), "parallel server count") {
		t.Fatalf("error=%v", err)
	}
}
