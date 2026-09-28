package dashboard

import (
	"errors"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"go-deployer/internal/config"
	"go-deployer/internal/monitor"
)

type fixtureMonitor struct {
	statuses []monitor.AppStatus
	queries  chan monitor.Target
	actions  chan monitor.Target
}

func (m fixtureMonitor) Collect(*config.Config) []monitor.AppStatus {
	return append([]monitor.AppStatus(nil), m.statuses...)
}
func (m fixtureMonitor) CollectTarget(_ *config.Config, target monitor.Target) (monitor.AppStatus, error) {
	m.queries <- target
	status := m.statuses[target.AppIndex]
	status.State = monitor.StateRunning
	return status, nil
}
func (m fixtureMonitor) Control(_ *config.Config, target monitor.Target, _ monitor.Action) (string, error) {
	m.actions <- target
	return "started", nil
}
func (m fixtureMonitor) OpenLog(*config.Config, monitor.Target) (*monitor.LogStream, error) {
	return nil, errors.New("not used by this fixture")
}

func workflowFixture(t *testing.T) (*application, fixtureMonitor, func()) {
	t.Helper()
	first, second := monitor.Target{AppIndex: 0}, monitor.Target{AppIndex: 1}
	m := fixtureMonitor{
		statuses: []monitor.AppStatus{{Target: first, AppName: "first", State: monitor.StateRunning}, {Target: second, AppName: "second", State: monitor.StateUnavailable}},
		queries:  make(chan monitor.Target, 2), actions: make(chan monitor.Target, 2),
	}
	a := &application{
		ui: tview.NewApplication(), monitor: m, activity: tview.NewTextView(), table: tview.NewTable(), results: tview.NewTable(),
		cfg: &config.Config{Servers: []config.ServerConfig{{Name: "server", Apps: []config.AppConfig{{Name: "first"}, {Name: "second"}}}}}, rows: m.statuses,
	}
	a.root = tview.NewFlex().AddItem(a.table, 0, 1, true)
	screen := tcell.NewSimulationScreen("UTF-8")
	a.ui.SetScreen(screen).SetRoot(a.root, true)
	done := make(chan error, 1)
	start := func() {
		go func() { done <- a.ui.Run() }()
		t.Cleanup(func() {
			a.ui.Stop()
			select {
			case err := <-done:
				if err != nil {
					t.Errorf("simulation UI: %v", err)
				}
			case <-time.After(2 * time.Second):
				t.Error("simulation UI did not stop")
			}
		})
	}
	return a, m, start
}

func receive[T any](t *testing.T, values <-chan T) T {
	t.Helper()
	select {
	case value := <-values:
		return value
	case <-time.After(2 * time.Second):
		t.Fatal("workflow did not complete")
	}
	var zero T
	return zero
}

func TestStatusRetryUpdatesOnlySelectedRow(t *testing.T) {
	a, m, start := workflowFixture(t)
	a.selected = monitor.Target{AppIndex: 1}
	completed := make(chan []monitor.AppStatus, 1)
	a.ui.SetAfterDrawFunc(func(tcell.Screen) {
		if len(a.history) > 0 && !a.refreshing {
			select {
			case completed <- append([]monitor.AppStatus(nil), a.rows...):
			default:
			}
		}
	})
	start()
	a.ui.QueueUpdateDraw(a.requestStatusRetry)
	if target := receive(t, m.queries); target.AppIndex != 1 {
		t.Fatal("wrong status retry target")
	}
	rows := receive(t, completed)
	if rows[0] != m.statuses[0] || rows[1].State != monitor.StateRunning {
		t.Fatal("retry must preserve unrelated row and update selected row")
	}
}

func TestConfirmedRetryKeepsOriginalTargetAfterSelectionChanges(t *testing.T) {
	a, m, start := workflowFixture(t)
	first := monitor.Target{AppIndex: 0}
	a.selected = first
	a.recordResult(operationResult{Target: first, App: "first", Server: "server", Kind: operationKind(monitor.ActionRestart), Failed: true, Message: "timeout"})
	completed := make(chan operationResult, 1)
	a.ui.SetAfterDrawFunc(func(tcell.Screen) {
		if result := a.lastActions[first]; !result.Failed && result.Kind != "" {
			select {
			case completed <- result:
			default:
			}
		}
	})
	start()
	a.ui.QueueUpdateDraw(func() {
		a.confirmRetry()
		// Confirmation must keep its captured target even if selection changes.
		a.selected = monitor.Target{AppIndex: 1}
	})
	select {
	case <-m.actions:
		t.Fatal("retry ran without confirmation")
	default:
	}
	a.ui.QueueEvent(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
	a.ui.QueueEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if target := receive(t, m.actions); target != first {
		t.Fatal("confirmation retried newly selected app")
	}
	if result := receive(t, completed); result.Target != first || result.Failed {
		t.Fatal("retry summary did not record success")
	}
}
