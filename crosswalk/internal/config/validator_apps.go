package config

import (
	"fmt"
	templatepkg "go-deployer/internal/template"
	"path/filepath"
	"strings"
)

func validateMonitoringApp(app *AppConfig, prefix string, errs *[]string, checkUnresolvedEnv func(string, string)) {
	app.Name = strings.TrimSpace(app.Name)
	app.BaseDir = filepath.ToSlash(strings.TrimSpace(app.BaseDir))
	app.Script.RemotePath = filepath.ToSlash(strings.TrimSpace(app.Script.RemotePath))
	checkUnresolvedEnv(app.Name, prefix+".name")
	checkUnresolvedEnv(app.BaseDir, prefix+".base_dir")
	checkUnresolvedEnv(app.Script.RemotePath, prefix+".script.remote_path")
	if app.Name == "" {
		*errs = append(*errs, prefix+".name is required")
	}
	if app.BaseDir == "" || !strings.HasPrefix(app.BaseDir, "/") {
		*errs = append(*errs, prefix+".base_dir must be an absolute path")
	}
	if app.Script.RemotePath == "" || !strings.HasPrefix(app.Script.RemotePath, "/") {
		*errs = append(*errs, prefix+".script.remote_path must be an absolute path")
	}
	if app.Port != 0 {
		validatePort(app.Port, prefix+".port", errs)
	}
	validateMonitorConfig(app, prefix, errs, checkUnresolvedEnv)
}

// validateMonitorConfig는 원격 shell에 전달되는 관측 기준과 로그 경로를 정규화한다.
func validateMonitorConfig(app *AppConfig, prefix string, errs *[]string, checkUnresolvedEnv func(string, string)) {
	rawProcessMatch := app.Monitor.ProcessMatch
	rawLogPath := app.Monitor.LogPath
	app.Monitor.ProcessMatch = strings.TrimSpace(rawProcessMatch)
	app.Monitor.LogPath = filepath.ToSlash(strings.TrimSpace(rawLogPath))

	checkUnresolvedEnv(app.Monitor.ProcessMatch, prefix+".monitor.process_match")
	checkUnresolvedEnv(app.Monitor.LogPath, prefix+".monitor.log_path")
	if strings.ContainsAny(rawProcessMatch, "\r\n\x00") {
		*errs = append(*errs, prefix+".monitor.process_match must not contain newlines or NUL bytes")
	}
	if strings.ContainsAny(rawLogPath, "\r\n\x00") {
		*errs = append(*errs, prefix+".monitor.log_path must not contain newlines or NUL bytes")
	} else if app.Monitor.LogPath != "" && !strings.HasPrefix(app.Monitor.LogPath, "/") {
		*errs = append(*errs, prefix+".monitor.log_path must be an absolute path")
	}
}

// validateAppConfig는 앱 배포에 필요한 jar, JVM, 포트, 설정 파일, 스크립트 값을 검증하고 기본값을 적용한다.
func validateAppConfig(app *AppConfig, prefix string, errs *[]string, checkUnresolvedEnv func(string, string), prodLike bool) {
	checkUnresolvedEnv(app.Name, prefix+".name")
	if app.Name == "" {
		*errs = append(*errs, prefix+".name is required")
	}
	validateTags(&app.Tags, prefix+".tags", errs, checkUnresolvedEnv)

	checkUnresolvedEnv(app.BaseDir, prefix+".base_dir")
	if app.BaseDir == "" {
		*errs = append(*errs, prefix+".base_dir is required")
	}
	validateMonitorConfig(app, prefix, errs, checkUnresolvedEnv)

	checkUnresolvedEnv(app.Jar.LocalPath, prefix+".jar.local_path")
	if app.Jar.LocalPath == "" {
		*errs = append(*errs, prefix+".jar.local_path is required")
	} else {
		validateExistingFile(app.Jar.LocalPath, prefix+".jar.local_path", errs)
	}

	checkUnresolvedEnv(app.Jar.RemotePath, prefix+".jar.remote_path")
	if app.Jar.RemotePath == "" {
		*errs = append(*errs, prefix+".jar.remote_path is required")
	}

	if app.Jvm.MinHeap == "" {
		app.Jvm.MinHeap = DefaultJvmMin
	}
	if app.Jvm.MaxHeap == "" {
		app.Jvm.MaxHeap = DefaultJvmMax
	}
	for j, opt := range app.Jvm.JavaOpts {
		optField := fmt.Sprintf("%s.jvm.java_opts[%d]", prefix, j)
		checkUnresolvedEnv(opt, optField)
		if strings.TrimSpace(opt) == "" {
			*errs = append(*errs, optField+" must not be empty")
		}
	}

	if app.Port == 0 {
		*errs = append(*errs, prefix+".port is required")
	} else {
		validatePort(app.Port, prefix+".port", errs)
	}

	for j, opt := range app.ExtraOpts {
		optField := fmt.Sprintf("%s.extra_opts[%d]", prefix, j)
		checkUnresolvedEnv(opt, optField)
		if strings.TrimSpace(opt) == "" {
			*errs = append(*errs, optField+" must not be empty")
		}
	}

	for j := range app.ConfigFiles {
		cf := &app.ConfigFiles[j]
		cf.Normalize()
		cfPrefix := fmt.Sprintf("%s.config_files[%d]", prefix, j)
		checkUnresolvedEnv(cf.LocalPath, cfPrefix+".local_path")
		if cf.LocalPath == "" {
			*errs = append(*errs, cfPrefix+".local_path is required")
		} else {
			if strings.HasPrefix(cf.LocalPath, "~/") {
				cf.LocalPath = expandHome(cf.LocalPath)
			}
			rejectProdSampleFile(cf.LocalPath, cfPrefix+".local_path", prodLike, errs)
			validateExistingFile(cf.LocalPath, cfPrefix+".local_path", errs)
		}
		checkUnresolvedEnv(cf.RemotePath, cfPrefix+".remote_path")
		if cf.RemotePath == "" {
			*errs = append(*errs, cfPrefix+".remote_path is required")
		}
	}

	validateExtraFiles(&app.ExtraFiles, prefix+".extra_files", errs, checkUnresolvedEnv, prodLike)

	validateScriptConfig(app, prefix, errs, checkUnresolvedEnv)
}

// validateScriptConfig는 template/local-file 스크립트 모드별로 허용되는 필드와 파일 경로를 검증한다.
func validateScriptConfig(app *AppConfig, prefix string, errs *[]string, checkUnresolvedEnv func(string, string)) {
	mode := strings.ToLower(strings.TrimSpace(app.Script.Mode))
	if mode == "" {
		mode = ScriptModeTemplate
	}

	switch mode {
	case ScriptModeTemplate, ScriptModeLocalFile:
		app.Script.Mode = mode
	default:
		*errs = append(*errs, fmt.Sprintf("%s.script.mode must be %q or %q", prefix, ScriptModeTemplate, ScriptModeLocalFile))
		return
	}

	app.Script.Normalize(app.BaseDir)

	switch app.Script.Mode {
	case ScriptModeTemplate:
		if app.Script.LocalPath != "" {
			checkUnresolvedEnv(app.Script.LocalPath, prefix+".script.local_path")
			if strings.HasPrefix(app.Script.LocalPath, "~/") {
				app.Script.LocalPath = expandHome(app.Script.LocalPath)
			}
		}

		if app.Script.Template != "" {
			checkUnresolvedEnv(app.Script.Template, prefix+".script.template")
			if strings.HasPrefix(app.Script.Template, "~/") {
				app.Script.Template = expandHome(app.Script.Template)
			}
			if strings.HasPrefix(app.Script.Template, "embedded:") {
				if err := templatepkg.ValidateEmbeddedTemplateRef(app.Script.Template); err != nil {
					*errs = append(*errs, fmt.Sprintf("%s is invalid: %v", prefix+".script.template", err))
				}
			} else {
				validateExistingFile(app.Script.Template, prefix+".script.template", errs)
			}
		}

		if app.Script.ValuesFile != "" {
			checkUnresolvedEnv(app.Script.ValuesFile, prefix+".script.values_file")
			if strings.HasPrefix(app.Script.ValuesFile, "~/") {
				app.Script.ValuesFile = expandHome(app.Script.ValuesFile)
			}
			validateExistingFile(app.Script.ValuesFile, prefix+".script.values_file", errs)
		}

	case ScriptModeLocalFile:
		checkUnresolvedEnv(app.Script.LocalPath, prefix+".script.local_path")
		if app.Script.LocalPath == "" {
			*errs = append(*errs, prefix+".script.local_path is required when script.mode is local-file")
		} else {
			if strings.HasPrefix(app.Script.LocalPath, "~/") {
				app.Script.LocalPath = expandHome(app.Script.LocalPath)
			}
			validateExistingFile(app.Script.LocalPath, prefix+".script.local_path", errs)
		}

		if app.Script.Template != "" {
			*errs = append(*errs, prefix+".script.template cannot be used when script.mode is local-file")
		}
		if app.Script.ValuesFile != "" {
			*errs = append(*errs, prefix+".script.values_file cannot be used when script.mode is local-file")
		}
	}
}
