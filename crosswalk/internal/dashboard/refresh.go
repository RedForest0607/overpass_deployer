package dashboard

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"go-deployer/internal/monitor"
)

// All refresh state is owned by the UI loop. Duplicate requests never overlap.
func (a *application) requestRefresh() { a.refreshStatus(nil) }

func (a *application) requestStatusRetry() {
	status, ok := a.currentStatus()
	if !ok {
		a.appendActivityNow("[yellow]Select an application before retrying its status.[-]\n")
		return
	}
	a.refreshStatus(&status.Target)
}

func (a *application) refreshStatus(target *monitor.Target) {
	if a.refreshing || a.deploying {
		a.appendActivityNow("[yellow]An operation is already running. Retry after it finishes.[-]\n")
		return
	}
	a.refreshing = true
	started := time.Now()
	a.appendActivityNow("[yellow]Refreshing application status...[-]\n")
	go func() {
		var statuses []monitor.AppStatus
		var err error
		if target == nil {
			statuses = a.monitor.Collect(a.cfg)
		} else {
			var status monitor.AppStatus
			status, err = a.monitor.CollectTarget(a.cfg, *target)
			if err == nil {
				statuses = []monitor.AppStatus{status}
			}
		}
		a.ui.QueueUpdateDraw(func() {
			a.refreshing = false
			if err != nil {
				a.appendActivityNow("[red]Status retry failed: " + tview.Escape(err.Error()) + "[-]\n")
				return
			}
			if target == nil {
				a.rows = statuses
			} else {
				a.rows = mergeStatus(a.rows, statuses[0])
			}
			failed := 0
			for _, status := range statuses {
				result := operationResult{Target: status.Target, Server: status.ServerName, App: status.AppName, Kind: operationStatus, StartedAt: started, Duration: time.Since(started), Message: string(status.State)}
				if status.State == monitor.StateUnavailable || status.State == monitor.StateUnknown {
					result.Failed = true
					result.Message = status.Message
					failed++
				}
				a.recordResult(result)
			}
			a.renderStatuses()
			a.appendActivityNow(fmt.Sprintf("Status refresh: %d checked, %d unavailable/unknown. Select a failed app and press t to retry only its query.\n", len(statuses), failed))
		})
	}()
}

func mergeStatus(statuses []monitor.AppStatus, updated monitor.AppStatus) []monitor.AppStatus {
	result := append([]monitor.AppStatus(nil), statuses...)
	for i := range result {
		if result[i].Target == updated.Target {
			result[i] = updated
			return result
		}
	}
	return append(result, updated)
}

func (a *application) renderStatuses() {
	selected := a.selected
	a.table.Clear()
	for column, title := range []string{"Server", "Application", "State", "PID", "Port", "Details"} {
		a.table.SetCell(0, column, tview.NewTableCell(title).SetSelectable(false).SetTextColor(tcell.ColorYellow))
	}
	for row, status := range a.rows {
		a.table.SetCell(row+1, 0, tview.NewTableCell(tview.Escape(status.ServerName+"\n"+status.Host)))
		a.table.SetCell(row+1, 1, tview.NewTableCell(tview.Escape(status.AppName)))
		a.table.SetCell(row+1, 2, tview.NewTableCell(strings.ToUpper(string(status.State))).SetTextColor(stateColor(status.State)))
		a.table.SetCell(row+1, 3, tview.NewTableCell(tview.Escape(status.PID)))
		a.table.SetCell(row+1, 4, tview.NewTableCell(fmt.Sprintf("%d", status.Port)))
		details := status.Message
		if details == "" {
			details = status.Process
		}
		if status.State == monitor.StateUnavailable {
			details += " (t: retry status)"
		}
		a.table.SetCell(row+1, 5, tview.NewTableCell(tview.Escape(details)))
	}
	if len(a.rows) > 0 {
		row := statusRow(a.rows, selected)
		if row < 0 {
			row = 0
		}
		a.selectRow(row + 1)
		a.table.Select(row+1, 0)
	}
}
