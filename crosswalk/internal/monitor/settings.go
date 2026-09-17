package monitor

import (
	"path"
	"strings"

	"go-deployer/internal/config"
)

// appSettings는 설정의 선택 필드와 기존 기본값을 하나의 런타임 계약으로 해석한다.
type appSettings struct {
	ProcessMatch string
	LogPath      string
}

func settingsFor(app config.AppConfig) appSettings {
	processMatch := strings.TrimSpace(app.Monitor.ProcessMatch)
	if processMatch == "" {
		processMatch = app.Name
	}

	logFilePath := strings.TrimSpace(app.Monitor.LogPath)
	if logFilePath == "" {
		logFilePath = path.Join(app.BaseDir, "logs", app.Name+".log")
	}

	return appSettings{ProcessMatch: processMatch, LogPath: logFilePath}
}
