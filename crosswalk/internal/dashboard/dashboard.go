package dashboard

import (
	"bufio"
	"fmt"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"go-deployer/internal/config"
	"go-deployer/internal/monitor"
)

const maxActivityBytes = 256 * 1024

// Run은 deploy.yml을 기준으로 상태, 로그, 제어, 선택 배포를 제공하는 터미널 대시보드를 시작한다.
func Run(cfg *config.Config) error {
	if !hasApplications(cfg) {
		return fmt.Errorf("dashboard requires VM applications; use docker or compose CLI commands for container workloads")
	}
	dashboard := &application{
		ui:      tview.NewApplication().EnableMouse(true),
		cfg:     cfg,
		monitor: monitor.NewService(),
		table:   tview.NewTable().SetSelectable(true, false).SetFixed(1, 0),
		results: tview.NewTable().SetFixed(1, 0),
		activity: tview.NewTextView().
			SetDynamicColors(true).
			SetScrollable(true).
			SetWrap(false),
	}
	dashboard.table.SetBorder(true).SetTitle(" Application Status ")
	dashboard.activity.SetBorder(true).SetTitle(" Activity and Logs ")
	dashboard.results.SetBorder(true).SetTitle(" Recent Results (selected app: t retry status / y retry action) ")
	dashboard.table.SetSelectionChangedFunc(func(row, _ int) { dashboard.selectRow(row) })

	body := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(dashboard.table, 0, 2, true).
		AddItem(dashboard.results, 0, 1, false).
		AddItem(dashboard.activity, 0, 3, false)
	dashboard.root = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(body, 0, 1, true).
		AddItem(dashboard.controls(), 1, 0, false)

	dashboard.ui.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if dashboard.confirming {
			return event
		}
		if event.Key() == tcell.KeyCtrlC || event.Rune() == 'q' {
			if dashboard.deploying {
				return nil
			}
			dashboard.stopLog()
			dashboard.ui.Stop()
			return nil
		}
		switch event.Rune() {
		case 'r':
			dashboard.requestRefresh()
			return nil
		case 't':
			dashboard.requestStatusRetry()
			return nil
		case 'y':
			dashboard.confirmRetry()
			return nil
		case 'l':
			dashboard.requestLog()
			return nil
		case 's':
			dashboard.confirmControl(monitor.ActionStart)
			return nil
		case 'x':
			dashboard.confirmControl(monitor.ActionStop)
			return nil
		case 'R':
			dashboard.confirmControl(monitor.ActionRestart)
			return nil
		case 'd':
			dashboard.confirmDeployment(true)
			return nil
		case 'D':
			dashboard.confirmDeployment(false)
			return nil
		}
		return event
	})

	initialized := false
	dashboard.ui.SetBeforeDrawFunc(func(tcell.Screen) bool {
		if !initialized {
			initialized = true
			dashboard.requestRefresh()
		}
		return false
	})
	return dashboard.ui.SetRoot(dashboard.root, true).Run()
}

type monitoringService interface {
	Collect(*config.Config) []monitor.AppStatus
	CollectTarget(*config.Config, monitor.Target) (monitor.AppStatus, error)
	Control(*config.Config, monitor.Target, monitor.Action) (string, error)
	OpenLog(*config.Config, monitor.Target) (*monitor.LogStream, error)
}

type application struct {
	ui          *tview.Application
	root        tview.Primitive
	cfg         *config.Config
	monitor     monitoringService
	table       *tview.Table
	results     *tview.Table
	history     []operationResult
	lastActions map[monitor.Target]operationResult
	refreshing  bool
	activity    *tview.TextView
	rows        []monitor.AppStatus
	selected    monitor.Target
	logMu       sync.Mutex
	log         *monitor.LogStream
	logVersion  uint64
	confirming  bool
	deploying   bool
}

func (a *application) controls() *tview.Flex {
	buttons := []struct {
		label  string
		action func()
	}{
		{"Refresh", a.requestRefresh},
		{"Retry status", a.requestStatusRetry},
		{"Retry action", a.confirmRetry},
		{"Log", a.requestLog},
		{"Start", func() { a.confirmControl(monitor.ActionStart) }},
		{"Stop", func() { a.confirmControl(monitor.ActionStop) }},
		{"Restart", func() { a.confirmControl(monitor.ActionRestart) }},
		{"Dry-run", func() { a.confirmDeployment(true) }},
		{"Deploy", func() { a.confirmDeployment(false) }},
	}
	bar := tview.NewFlex()
	for _, button := range buttons {
		bar.AddItem(tview.NewButton(button.label).SetSelectedFunc(button.action), 0, 1, false)
	}
	return bar
}

// requestLog는 SSH 연결과 로그 stream 생성을 UI 이벤트 루프 밖에서 처리한다.
func (a *application) requestLog() {
	target := a.selected
	appName := a.appName(target)
	version := a.stopLog()
	go a.openLog(target, appName, version)
}

func (a *application) selectRow(row int) {
	if row <= 0 || row-1 >= len(a.rows) {
		return
	}
	status := a.rows[row-1]
	if a.activity != nil {
		a.activity.SetTitle(fmt.Sprintf(" Activity and Logs: %s ", status.AppName))
	}
	target := status.Target
	if target == a.selected {
		return
	}
	a.stopLog()
	a.selected = target
}

func statusRow(statuses []monitor.AppStatus, target monitor.Target) int {
	for row, status := range statuses {
		if status.Target == target {
			return row
		}
	}
	return -1
}

func stateColor(state monitor.State) tcell.Color {
	switch state {
	case monitor.StateRunning:
		return tcell.ColorGreen
	case monitor.StateStopped:
		return tcell.ColorRed
	case monitor.StateUnavailable:
		return tcell.ColorOrange
	default:
		return tcell.ColorYellow
	}
}

func (a *application) openLog(target monitor.Target, appName string, version uint64) {
	stream, err := a.monitor.OpenLog(a.cfg, target)
	if err != nil {
		a.appendLogActivity(version, fmt.Sprintf("[red]Unable to open log stream: %v[-]\n", err), false)
		return
	}
	if !a.setLog(version, stream) {
		_ = stream.Close()
		return
	}
	a.appendLogActivity(version, fmt.Sprintf("[yellow]Log stream started: %s. Selecting another app or closing the dashboard stops it.[-]\n", appName), true)
	go func() {
		scanner := bufio.NewScanner(stream.Reader)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			a.appendLogActivity(version, scanner.Text()+"\n", false)
		}
		if err := scanner.Err(); err != nil {
			a.appendLogActivity(version, fmt.Sprintf("[red]Log stream ended: %v[-]\n", err), false)
		}
	}()
}

func (a *application) stopLog() uint64 {
	a.logMu.Lock()
	a.logVersion++
	version := a.logVersion
	stream := a.log
	a.log = nil
	a.logMu.Unlock()
	if stream != nil {
		_ = stream.Close()
	}
	return version
}

func (a *application) setLog(version uint64, stream *monitor.LogStream) bool {
	a.logMu.Lock()
	defer a.logMu.Unlock()
	if version != a.logVersion {
		return false
	}
	a.log = stream
	return true
}

func (a *application) appendLogActivity(version uint64, message string, clear bool) {
	a.ui.QueueUpdateDraw(func() {
		a.logMu.Lock()
		current := version == a.logVersion
		a.logMu.Unlock()
		if !current {
			return
		}
		if clear {
			a.activity.SetText("")
		}
		a.appendActivityNow(message)
	})
}

func (a *application) appName(target monitor.Target) string {
	for _, status := range a.rows {
		if status.Target == target {
			return status.AppName
		}
	}
	return "unknown"
}

func (a *application) currentStatus() (monitor.AppStatus, bool) {
	for _, status := range a.rows {
		if status.Target == a.selected {
			return status, true
		}
	}
	return monitor.AppStatus{}, false
}

func (a *application) confirm(message string, action func()) {
	if a.confirming {
		return
	}
	a.confirming = true
	modal := tview.NewModal().SetText(message).AddButtons([]string{"Cancel", "Confirm"}).SetDoneFunc(func(_ int, label string) {
		a.confirming = false
		a.ui.SetRoot(a.root, true)
		a.ui.SetFocus(a.table)
		if label == "Confirm" {
			action()
		}
	})
	a.ui.SetRoot(modal, true)
	a.ui.SetFocus(modal)
}

func (a *application) appendActivity(message string) {
	a.ui.QueueUpdateDraw(func() {
		a.appendActivityNow(message)
	})
}

func (a *application) appendActivityNow(message string) {
	existing := a.activity.GetText(true)
	if len(existing)+len(message) > maxActivityBytes {
		trimmed := len(existing) + len(message) - maxActivityBytes
		if trimmed < len(existing) {
			existing = existing[trimmed:]
		} else {
			existing = ""
		}
	}
	a.activity.SetText(existing + message)
	a.activity.ScrollToEnd()
}

type dashboardLogWriter struct{ dashboard *application }

func (w dashboardLogWriter) Write(p []byte) (int, error) {
	w.dashboard.appendActivity(string(p))
	return len(p), nil
}
