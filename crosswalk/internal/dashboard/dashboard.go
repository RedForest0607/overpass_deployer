package dashboard

import (
	"bufio"
	"fmt"
	"strings"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"go-deployer/internal/config"
	"go-deployer/internal/monitor"
	"go-deployer/internal/vm"
	"go-deployer/pkg/logger"
)

const maxActivityBytes = 256 * 1024

// Run은 deploy.yml을 기준으로 상태, 로그, 제어, 선택 배포를 제공하는 터미널 대시보드를 시작한다.
func Run(cfg *config.Config) error {
	dashboard := &application{
		ui:      tview.NewApplication().EnableMouse(true),
		cfg:     cfg,
		monitor: monitor.NewService(),
		table:   tview.NewTable().SetSelectable(true, false).SetFixed(1, 0),
		activity: tview.NewTextView().
			SetDynamicColors(true).
			SetScrollable(true).
			SetWrap(false),
	}
	dashboard.table.SetBorder(true).SetTitle(" Application Status ")
	dashboard.activity.SetBorder(true).SetTitle(" Activity and Logs ")
	dashboard.table.SetSelectedFunc(func(row, _ int) {
		if row > 0 && row-1 < len(dashboard.rows) {
			target := dashboard.rows[row-1].Target
			if target != dashboard.selected {
				dashboard.stopLog()
			}
			dashboard.selected = target
		}
	})

	body := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(dashboard.table, 0, 2, true).
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

type application struct {
	ui         *tview.Application
	root       tview.Primitive
	cfg        *config.Config
	monitor    monitor.Service
	table      *tview.Table
	activity   *tview.TextView
	rows       []monitor.AppStatus
	selected   monitor.Target
	logMu      sync.Mutex
	log        *monitor.LogStream
	confirming bool
	deploying  bool
}

func (a *application) controls() *tview.Flex {
	buttons := []struct {
		label  string
		action func()
	}{
		{"Refresh", a.requestRefresh},
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

// requestRefresh는 UI 이벤트 루프를 막지 않도록 상태 조회를 별도 goroutine으로 요청한다.
func (a *application) requestRefresh() {
	go a.refresh()
}

func (a *application) refresh() {
	a.appendActivity("[yellow]Refreshing application status...[-]\n")
	go func() {
		statuses := a.monitor.Collect(a.cfg)
		a.ui.QueueUpdateDraw(func() {
			a.rows = statuses
			a.table.Clear()
			for column, title := range []string{"Server", "Application", "State", "PID", "Port", "Details"} {
				a.table.SetCell(0, column, tview.NewTableCell(title).SetSelectable(false).SetTextColor(tcell.ColorYellow))
			}
			for row, status := range statuses {
				a.table.SetCell(row+1, 0, tview.NewTableCell(status.ServerName+"\n"+status.Host))
				a.table.SetCell(row+1, 1, tview.NewTableCell(status.AppName))
				a.table.SetCell(row+1, 2, tview.NewTableCell(strings.ToUpper(string(status.State))).SetTextColor(stateColor(status.State)))
				a.table.SetCell(row+1, 3, tview.NewTableCell(status.PID))
				a.table.SetCell(row+1, 4, tview.NewTableCell(fmt.Sprintf("%d", status.Port)))
				details := status.Message
				if details == "" {
					details = status.Process
				}
				a.table.SetCell(row+1, 5, tview.NewTableCell(details))
			}
			if len(statuses) > 0 {
				a.selected = statuses[0].Target
				a.table.Select(1, 0)
			}
			a.appendActivityNow(fmt.Sprintf("[green]Status refresh completed: %d application(s).[-]\n", len(statuses)))
		})
	}()
}

// requestLog는 SSH 연결과 로그 stream 생성을 UI 이벤트 루프 밖에서 처리한다.
func (a *application) requestLog() {
	target := a.selected
	go a.openLog(target)
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

func (a *application) openLog(target monitor.Target) {
	a.stopLog()
	stream, err := a.monitor.OpenLog(a.cfg, target)
	if err != nil {
		a.appendActivity(fmt.Sprintf("[red]Unable to open log stream: %v[-]\n", err))
		return
	}
	a.setLog(stream)
	a.appendActivity("[yellow]Log stream started. Selecting another app or closing the dashboard stops it.[-]\n")
	go func() {
		scanner := bufio.NewScanner(stream.Reader)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			a.appendActivity(scanner.Text() + "\n")
		}
		if err := scanner.Err(); err != nil {
			a.appendActivity(fmt.Sprintf("[red]Log stream ended: %v[-]\n", err))
		}
	}()
}

func (a *application) stopLog() {
	a.logMu.Lock()
	stream := a.log
	a.log = nil
	a.logMu.Unlock()
	if stream == nil {
		return
	}
	_ = stream.Close()
}

func (a *application) setLog(stream *monitor.LogStream) {
	a.logMu.Lock()
	previous := a.log
	a.log = stream
	a.logMu.Unlock()
	if previous != nil {
		_ = previous.Close()
	}
}

func (a *application) confirmControl(action monitor.Action) {
	status, ok := a.currentStatus()
	if !ok {
		a.appendActivityNow("[red]Select an application before controlling it.[-]\n")
		return
	}
	target := status.Target
	a.confirm(fmt.Sprintf("Run %s for %s on %s?", strings.ToUpper(string(action)), status.AppName, status.ServerName), func() { a.runControl(target, action) })
}

func (a *application) runControl(target monitor.Target, action monitor.Action) {
	go func() {
		output, err := a.monitor.Control(a.cfg, target, action)
		if output != "" {
			a.appendActivity(output + "\n")
		}
		if err != nil {
			a.appendActivity(fmt.Sprintf("[red]%s failed: %v[-]\n", action, err))
		} else {
			a.appendActivity(fmt.Sprintf("[green]%s completed.[-]\n", action))
		}
		a.requestRefresh()
	}()
}

func (a *application) confirmDeployment(dryRun bool) {
	if a.deploying {
		a.appendActivityNow("[yellow]A deployment is already running.[-]\n")
		return
	}
	status, ok := a.currentStatus()
	if !ok {
		a.appendActivityNow("[red]Select an application before deploying it.[-]\n")
		return
	}
	label := "DEPLOY"
	if dryRun {
		label = "DRY-RUN"
	}
	target := status.Target
	a.confirm(fmt.Sprintf("Run %s for %s on %s?", label, status.AppName, status.ServerName), func() { a.runDeployment(target, dryRun) })
}

func (a *application) runDeployment(target monitor.Target, dryRun bool) {
	selectedCfg, err := selectedDeploymentConfig(a.cfg, target)
	if err != nil {
		a.appendActivityNow(fmt.Sprintf("[red]Unable to prepare deployment: %v[-]\n", err))
		return
	}
	a.deploying = true
	go func() {
		if err := config.ValidateAndApplyDefaults(selectedCfg); err != nil {
			a.appendActivity(fmt.Sprintf("[red]Deployment configuration is invalid: %v[-]\n", err))
			a.ui.QueueUpdateDraw(func() { a.deploying = false })
			return
		}

		restoreOutput := logger.SetOutput(dashboardLogWriter{dashboard: a})
		defer restoreOutput()
		defer a.ui.QueueUpdateDraw(func() { a.deploying = false })
		a.appendActivity("[yellow]Deployment started.[-]\n")
		err := vm.RunWithOptions(selectedCfg, vm.RunOptions{DryRun: dryRun})
		if err != nil {
			a.appendActivity(fmt.Sprintf("[red]Deployment failed: %v[-]\n", err))
		} else {
			a.appendActivity("[green]Deployment completed.[-]\n")
		}
		a.requestRefresh()
	}()
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

// selectedDeploymentConfig은 선택 앱과 그 배포에 필요한 SSH/디렉터리 기본값만 새 설정으로 복사한다.
// 서버 bootstrap, 공통 파일, utils, bastion 동기화는 대시보드의 앱 단위 배포 범위에서 제외한다.
func selectedDeploymentConfig(cfg *config.Config, target monitor.Target) (*config.Config, error) {
	if target.ServerIndex < 0 || target.ServerIndex >= len(cfg.Servers) {
		return nil, fmt.Errorf("server selection is out of range")
	}
	server := cfg.Servers[target.ServerIndex]
	apps := server.EffectiveApps()
	if target.AppIndex < 0 || target.AppIndex >= len(apps) {
		return nil, fmt.Errorf("application selection is out of range")
	}

	selectedServer := config.ServerConfig{
		Host:    server.Host,
		Name:    server.Name,
		Tags:    append([]string(nil), server.Tags...),
		SSHPort: server.SSHPort,
		Apps:    []config.AppConfig{cloneAppConfig(apps[target.AppIndex])},
	}
	return &config.Config{
		SSH: cfg.SSH,
		DirectoryDefaults: config.DirectoryDefaultsConfig{
			Rules: append([]config.DirectoryOwnershipRule(nil), cfg.DirectoryDefaults.Rules...),
		},
		Servers: []config.ServerConfig{selectedServer},
	}, nil
}

func cloneAppConfig(app config.AppConfig) config.AppConfig {
	cloned := app
	cloned.Tags = append([]string(nil), app.Tags...)
	cloned.Jvm.JavaOpts = append([]string(nil), app.Jvm.JavaOpts...)
	cloned.ExtraOpts = append(config.StringList(nil), app.ExtraOpts...)
	cloned.ConfigFiles = append([]config.ConfigFile(nil), app.ConfigFiles...)
	cloned.ExtraFiles = append([]config.ExtraFile(nil), app.ExtraFiles...)
	return cloned
}
