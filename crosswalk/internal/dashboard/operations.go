package dashboard

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"go-deployer/internal/config"
	"go-deployer/internal/monitor"
	"go-deployer/internal/vm"
	"go-deployer/pkg/logger"
)

type operationKind string

const (
	operationStatus operationKind = "status"
	operationDryRun operationKind = "dry-run"
	operationDeploy operationKind = "deploy"
	maxResults                    = 50
)

type operationResult struct {
	Target      monitor.Target
	Server, App string
	Kind        operationKind
	Failed      bool
	Message     string
	StartedAt   time.Time
	Duration    time.Duration
}

// Status refreshes and history trimming cannot erase a retryable action.
func latestAction(actions map[monitor.Target]operationResult, target monitor.Target) (operationResult, bool) {
	result, ok := actions[target]
	return result, ok && result.Failed
}

func (a *application) recordResult(result operationResult) {
	if result.Kind != operationStatus {
		if a.lastActions == nil {
			a.lastActions = make(map[monitor.Target]operationResult)
		}
		a.lastActions[result.Target] = result
	}
	a.history = append(a.history, result)
	if len(a.history) > maxResults {
		a.history = a.history[len(a.history)-maxResults:]
	}
	if a.results == nil {
		return
	}
	a.results.Clear()
	for column, title := range []string{"Server", "Application", "Operation", "Result", "Duration", "Details"} {
		a.results.SetCell(0, column, tview.NewTableCell(title).SetTextColor(tcell.ColorYellow))
	}
	for i := len(a.history) - 1; i >= 0; i-- {
		item := a.history[i]
		state, color := "done", tcell.ColorGreen
		if item.Failed {
			state, color = "failed", tcell.ColorRed
		}
		cells := []string{item.Server, item.App, string(item.Kind), state, item.Duration.Round(time.Millisecond).String(), strings.Join(strings.Fields(item.Message), " ")}
		for column, text := range cells {
			cell := tview.NewTableCell(tview.Escape(text))
			if column == 3 {
				cell.SetTextColor(color)
			}
			a.results.SetCell(len(a.history)-i, column, cell)
		}
	}
}

func (a *application) operationBlocked() bool {
	if !a.deploying && !a.refreshing {
		return false
	}
	a.appendActivityNow("[yellow]An operation is already running. Retry after it finishes.[-]\n")
	return true
}

func (a *application) confirmControl(action monitor.Action) {
	if a.operationBlocked() {
		return
	}
	status, ok := a.currentStatus()
	if !ok {
		a.appendActivityNow("[red]Select an application before controlling it.[-]\n")
		return
	}
	a.confirm(fmt.Sprintf("Run %s for %s on %s?", strings.ToUpper(string(action)), status.AppName, status.ServerName), func() { a.runControl(status.Target, action) })
}

func (a *application) beginOperation(target monitor.Target, kind operationKind) (operationResult, bool) {
	if a.operationBlocked() {
		return operationResult{}, false
	}
	selected, err := selectedDeploymentConfig(a.cfg, target)
	if err != nil {
		a.appendActivityNow("[red]Unable to prepare operation: " + tview.Escape(err.Error()) + "[-]\n")
		return operationResult{}, false
	}
	server := selected.Servers[0]
	result := operationResult{Target: target, Server: server.Name, App: server.Apps[0].Name, Kind: kind, StartedAt: time.Now()}
	a.deploying = true
	a.stopLog()
	a.appendActivityNow(fmt.Sprintf("[yellow]%s started: %s / %s.[-]\n", kind, tview.Escape(result.Server), tview.Escape(result.App)))
	return result, true
}

func (a *application) finishOperation(result operationResult, output string, err error) {
	result.Duration = time.Since(result.StartedAt)
	result.Message = "completed"
	if err != nil {
		result.Failed = true
		result.Message = err.Error()
	}
	a.ui.QueueUpdateDraw(func() {
		a.deploying = false
		a.recordResult(result)
		if output != "" {
			a.appendActivityNow(tview.Escape(output) + "\n")
		}
		if err != nil {
			a.appendActivityNow("[red]" + string(result.Kind) + " failed: " + tview.Escape(err.Error()) + "[-]\nSelect this app and press y to review a retry. Remote work may have partially completed.\n")
		} else {
			a.appendActivityNow("[green]" + string(result.Kind) + " completed.[-]\n")
		}
		a.requestRefresh()
	})
}

func (a *application) runControl(target monitor.Target, action monitor.Action) {
	result, ok := a.beginOperation(target, operationKind(action))
	if !ok {
		return
	}
	go func() {
		output, err := a.monitor.Control(a.cfg, target, action)
		a.finishOperation(result, output, err)
	}()
}

func (a *application) confirmDeployment(dryRun bool) {
	if a.operationBlocked() {
		return
	}
	status, ok := a.currentStatus()
	if !ok {
		a.appendActivityNow("[red]Select an application before deploying it.[-]\n")
		return
	}
	kind := operationDeploy
	if dryRun {
		kind = operationDryRun
	}
	a.confirm(fmt.Sprintf("Run %s for %s on %s?", strings.ToUpper(string(kind)), status.AppName, status.ServerName), func() { a.runDeployment(status.Target, dryRun) })
}

func (a *application) runDeployment(target monitor.Target, dryRun bool) {
	kind := operationDeploy
	if dryRun {
		kind = operationDryRun
	}
	result, ok := a.beginOperation(target, kind)
	if !ok {
		return
	}
	selected, _ := selectedDeploymentConfig(a.cfg, target) // Validated by beginOperation.
	go func() {
		if err := config.ValidateAndApplyDefaults(selected); err != nil {
			a.finishOperation(result, "", err)
			return
		}
		restore := logger.SetOutput(dashboardLogWriter{dashboard: a})
		err := vm.RunWithOptions(selected, vm.RunOptions{DryRun: dryRun})
		restore()
		a.finishOperation(result, "", err)
	}()
}

func (a *application) confirmRetry() {
	if a.operationBlocked() {
		return
	}
	status, ok := a.currentStatus()
	if !ok {
		a.appendActivityNow("[yellow]Select the failed application's row first.[-]\n")
		return
	}
	result, ok := latestAction(a.lastActions, status.Target)
	if !ok {
		a.appendActivityNow("[yellow]No failed action remains for this app. Use t to retry its status query.[-]\n")
		return
	}
	a.confirm(fmt.Sprintf("Retry %s for %s on %s?\nPrevious error: %s\nRemote work may have partially completed. Only this app is retried.", result.Kind, result.App, result.Server, result.Message), func() {
		switch result.Kind {
		case operationDeploy, operationDryRun:
			a.runDeployment(result.Target, result.Kind == operationDryRun)
		case operationKind(monitor.ActionStart), operationKind(monitor.ActionStop), operationKind(monitor.ActionRestart):
			a.runControl(result.Target, monitor.Action(result.Kind))
		}
	})
}
