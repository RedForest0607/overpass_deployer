package vm

import (
	"fmt"
	"os"
	"strings"
	"time"

	"go-deployer/internal/config"
)

const (
	estimatedTransferBytesPerSecond = 25 * 1024 * 1024
	estimatedExtractBytesPerSecond  = 80 * 1024 * 1024

	estimatedConnectDuration       = 5 * time.Second
	estimatedOSUpdateDuration      = 2 * time.Minute
	estimatedPackageBaseDuration   = 45 * time.Second
	estimatedPackageItemDuration   = 8 * time.Second
	estimatedTimezoneDuration      = 5 * time.Second
	estimatedSwapDuration          = 30 * time.Second
	estimatedDirectoryDuration     = 1 * time.Second
	estimatedFileOverheadDuration  = 2 * time.Second
	estimatedAppOverheadDuration   = 3 * time.Second
	estimatedScriptDuration        = 2 * time.Second
	estimatedBastionServerDuration = 3 * time.Second
)

type deploymentEstimate struct {
	Duration      time.Duration
	TransferBytes int64
	FileCount     int
	ServerCount   int
}

func estimateConfigDeployment(cfg *config.Config) (deploymentEstimate, error) {
	total := deploymentEstimate{ServerCount: len(cfg.Servers)}

	for _, server := range cfg.Servers {
		serverEstimate, err := estimateServerDeployment(server)
		if err != nil {
			return deploymentEstimate{}, fmt.Errorf("estimating %s: %w", server.Name, err)
		}
		serverEstimate.ServerCount = 0
		total = mergeEstimate(total, serverEstimate)
	}

	if cfg.Bastion.Enabled() && len(cfg.Servers) > 0 {
		total.Duration += time.Duration(len(cfg.Servers)) * estimatedBastionServerDuration
	}

	return total, nil
}

func estimateServerDeployment(server config.ServerConfig) (deploymentEstimate, error) {
	estimate := deploymentEstimate{
		Duration:    estimatedConnectDuration,
		ServerCount: 1,
	}

	bootstrapDuration, err := estimateBootstrapDuration(server.Bootstrap)
	if err != nil {
		return deploymentEstimate{}, err
	}
	estimate.Duration += bootstrapDuration
	estimate.Duration += time.Duration(len(server.Directories)) * estimatedDirectoryDuration

	filesEstimate, err := estimateExtraFiles(server.ExtraFiles)
	if err != nil {
		return deploymentEstimate{}, err
	}
	estimate = mergeEstimate(estimate, filesEstimate)

	for _, app := range server.EffectiveApps() {
		appEstimate, err := estimateAppDeployment(app)
		if err != nil {
			return deploymentEstimate{}, fmt.Errorf("app %s: %w", app.Name, err)
		}
		estimate = mergeEstimate(estimate, appEstimate)
	}

	return estimate, nil
}

func estimateBootstrapDuration(bootstrap config.BootstrapConfig) (time.Duration, error) {
	var duration time.Duration

	packages, err := packagesForBootstrap(bootstrap)
	if err != nil {
		return 0, err
	}
	if bootstrap.OSUpdate.Enabled != nil && *bootstrap.OSUpdate.Enabled {
		duration += estimatedOSUpdateDuration
	}
	if len(packages) > 0 {
		duration += estimatedPackageBaseDuration + time.Duration(len(packages))*estimatedPackageItemDuration
	}
	if bootstrap.Timezone.Name != "" {
		duration += estimatedTimezoneDuration
	}
	if bootstrap.Swap.Enabled != nil && *bootstrap.Swap.Enabled {
		duration += estimatedSwapDuration
	}

	return duration, nil
}

func estimateAppDeployment(app config.AppConfig) (deploymentEstimate, error) {
	estimate := deploymentEstimate{
		Duration: estimatedAppOverheadDuration + 3*estimatedDirectoryDuration + estimatedScriptDuration,
	}

	jarEstimate, err := estimateFile(app.Jar.LocalPath, false)
	if err != nil {
		return deploymentEstimate{}, fmt.Errorf("jar: %w", err)
	}
	estimate = mergeEstimate(estimate, jarEstimate)

	for _, cf := range app.ConfigFiles {
		cf.Normalize()
		fileEstimate, err := estimateFile(cf.LocalPath, false)
		if err != nil {
			return deploymentEstimate{}, fmt.Errorf("config file %s: %w", cf.LocalPath, err)
		}
		estimate = mergeEstimate(estimate, fileEstimate)
	}

	extraEstimate, err := estimateExtraFiles(app.ExtraFiles)
	if err != nil {
		return deploymentEstimate{}, err
	}
	estimate = mergeEstimate(estimate, extraEstimate)

	if app.Script.Mode == config.ScriptModeLocalFile && app.Script.LocalPath != "" {
		scriptEstimate, err := estimateFile(app.Script.LocalPath, false)
		if err != nil {
			return deploymentEstimate{}, fmt.Errorf("script %s: %w", app.Script.LocalPath, err)
		}
		estimate = mergeEstimate(estimate, scriptEstimate)
	}

	return estimate, nil
}

func estimateExtraFiles(extraFiles []config.ExtraFile) (deploymentEstimate, error) {
	var estimate deploymentEstimate
	for _, ef := range extraFiles {
		fileEstimate, err := estimateFile(ef.LocalPath, ef.Extract.Enabled)
		if err != nil {
			return deploymentEstimate{}, fmt.Errorf("extra file %s: %w", ef.LocalPath, err)
		}
		estimate = mergeEstimate(estimate, fileEstimate)
	}
	return estimate, nil
}

func estimateFile(localPath string, extracts bool) (deploymentEstimate, error) {
	info, err := os.Stat(localPath)
	if err != nil {
		return deploymentEstimate{}, err
	}
	if info.IsDir() {
		return deploymentEstimate{}, fmt.Errorf("%s is a directory", localPath)
	}

	size := info.Size()
	duration := estimatedFileOverheadDuration + durationForBytes(size, estimatedTransferBytesPerSecond)
	if extracts {
		duration += durationForBytes(size, estimatedExtractBytesPerSecond)
	}

	return deploymentEstimate{
		Duration:      duration,
		TransferBytes: size,
		FileCount:     1,
	}, nil
}

func durationForBytes(size int64, bytesPerSecond int64) time.Duration {
	if size <= 0 {
		return 0
	}
	seconds := size / bytesPerSecond
	if size%bytesPerSecond != 0 {
		seconds++
	}
	return time.Duration(seconds) * time.Second
}

func mergeEstimate(base deploymentEstimate, next deploymentEstimate) deploymentEstimate {
	base.Duration += next.Duration
	base.TransferBytes += next.TransferBytes
	base.FileCount += next.FileCount
	base.ServerCount += next.ServerCount
	return base
}

func formatEstimate(estimate deploymentEstimate) string {
	return fmt.Sprintf("about %s, %s across %d files",
		formatEstimateDuration(estimate.Duration),
		formatBytes(estimate.TransferBytes),
		estimate.FileCount,
	)
}

func formatEstimateDuration(duration time.Duration) string {
	if duration < time.Minute {
		if duration < time.Second {
			return "<1s"
		}
		return fmt.Sprintf("%ds", int(duration.Seconds()))
	}

	duration = duration.Round(time.Minute)
	hours := int(duration / time.Hour)
	minutes := int((duration % time.Hour) / time.Minute)
	parts := make([]string, 0, 2)
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if minutes > 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}
	return strings.Join(parts, " ")
}

func formatBytes(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	value := float64(size)
	units := []string{"KiB", "MiB", "GiB", "TiB"}
	for _, suffix := range units {
		value /= unit
		if value < unit {
			return fmt.Sprintf("%.1f %s", value, suffix)
		}
	}
	return fmt.Sprintf("%.1f PiB", value/unit)
}
