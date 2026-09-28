package dashboard

import (
	"strings"
	"testing"

	"github.com/rivo/tview"
	"go-deployer/internal/config"
	"go-deployer/internal/monitor"
)

func TestRetryKeepsFailedTargetAndDoesNotReplaySuccessfulAction(t *testing.T) {
	first := monitor.Target{ServerIndex: 0, AppIndex: 0}
	second := monitor.Target{ServerIndex: 1, AppIndex: 0}
	a := &application{}
	for _, result := range []operationResult{
		{Target: first, Kind: operationDeploy, Failed: true},
		{Target: second, Kind: operationKind(monitor.ActionRestart), Failed: true},
		{Target: first, Kind: operationStatus},
	} {
		a.recordResult(result)
	}
	result, ok := latestAction(a.lastActions, first)
	if !ok || result.Target != first || result.Kind != operationDeploy {
		t.Fatalf("retry must retain selected failed action, got %#v", result)
	}
	for i := 0; i < maxResults+1; i++ {
		a.recordResult(operationResult{Target: first, Kind: operationStatus})
	}
	if _, ok := latestAction(a.lastActions, first); !ok {
		t.Fatal("status/history trimming must retain a failed action")
	}
	a.recordResult(operationResult{Target: first, Kind: operationDeploy})
	if _, ok := latestAction(a.lastActions, first); ok {
		t.Fatal("a successful retry must supersede the old failure")
	}
	if _, ok := latestAction(a.lastActions, second); !ok {
		t.Fatal("success on one target must not clear another target's failure")
	}
}

func TestResultsAreBoundedAndRemoteTextCannotInjectFormatting(t *testing.T) {
	a := &application{results: tview.NewTable()}
	for i := 0; i < maxResults+3; i++ {
		a.recordResult(operationResult{App: "app", Kind: operationDeploy, Failed: true, Message: "[green]failed\nconnection"})
	}
	if len(a.history) != maxResults || a.results.GetRowCount() != maxResults+1 {
		t.Fatal("result table must retain only bounded recent results")
	}
	if text := a.results.GetCell(1, 5).Text; !strings.Contains(text, "[green[]") || strings.Contains(text, "\n") {
		t.Fatalf("remote result text must be escaped and compact, got %q", text)
	}
}

func TestBusyOperationCannotStartAnotherMutationOrRefresh(t *testing.T) {
	a := &application{activity: tview.NewTextView(), cfg: &config.Config{Servers: []config.ServerConfig{{Name: "server", Apps: []config.AppConfig{{Name: "app"}}}}}}
	if _, ok := a.beginOperation(monitor.Target{}, operationDeploy); !ok {
		t.Fatal("first operation should begin")
	}
	if _, ok := a.beginOperation(monitor.Target{}, operationKind(monitor.ActionRestart)); ok {
		t.Fatal("concurrent remote mutation must be rejected")
	}
	a.requestRefresh()
	if a.refreshing {
		t.Fatal("refresh should not race a mutation")
	}
	if !strings.Contains(a.activity.GetText(true), "already running") {
		t.Fatal("busy refusal needs actionable feedback")
	}
}

func TestDashboardRejectsContainerOnlyConfigWithoutStartingUI(t *testing.T) {
	if err := Run(&config.Config{Servers: []config.ServerConfig{{Containers: []config.ContainerConfig{{Name: "api"}}}}}); err == nil {
		t.Fatal("container-only dashboard must explain its supported scope")
	}
}
