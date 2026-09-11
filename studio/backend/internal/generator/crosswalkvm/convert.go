package crosswalkvm

import (
	"sort"
	"strings"

	"studio-backend/internal/domain"
)

func toSSH(value domain.SSHProfile) chartSSH {
	return chartSSH{User: value.User, KeyPath: value.KeyPath, KnownHostsPath: value.KnownHostsPath, HostKeyChecking: value.HostKeyChecking, Port: value.Port, TimeoutSeconds: value.TimeoutSeconds}
}

func toBastion(value *domain.BastionProfile) *chartBastion {
	if value == nil {
		return nil
	}
	return &chartBastion{Host: value.Host, User: value.User, KeyPath: value.KeyPath, KnownHostsPath: value.KnownHostsPath, HostKeyChecking: value.HostKeyChecking, Port: value.Port, TimeoutSeconds: value.TimeoutSeconds, AliasUser: value.AliasUser, SSHConfigPath: value.SSHConfigPath, ShellAliasesPath: value.ShellAliasesPath, TargetKnownHostsPath: value.TargetKnownHostsPath, TargetKeyPath: value.TargetKeyPath}
}

func fallbackTags(tags []string, environment string) []string {
	if len(tags) != 0 {
		return append([]string(nil), tags...)
	}
	return []string{strings.ToLower(environment)}
}

func toBootstrap(value *domain.BootstrapProfile) *chartBootstrap {
	if value == nil {
		return nil
	}
	result := &chartBootstrap{Packages: append([]string(nil), value.Packages...), Services: toServices(value.Services)}
	for _, item := range value.Users {
		result.Users = append(result.Users, chartUser{Name: item.Name, Group: item.Group, UID: item.UID, GID: item.GID, Home: item.Home, Shell: item.Shell, System: item.System, CreateHome: item.CreateHome})
	}
	for _, item := range value.Sysctls {
		result.Sysctls = append(result.Sysctls, chartSysctl{Name: item.Name, Value: item.Value, PersistPath: item.PersistPath})
	}
	if value.JDK != nil {
		result.JDK = &chartJDK{Enabled: value.JDK.Enabled, Vendor: value.JDK.Vendor, Major: value.JDK.Major, Headless: value.JDK.Headless}
	}
	if value.OSUpdate != nil {
		result.OSUpdate = &chartToggle{Enabled: value.OSUpdate.Enabled}
	}
	if value.Timezone != "" {
		result.Timezone = &chartTimezone{Name: value.Timezone}
	}
	if value.Swap != nil {
		result.Swap = &chartSwap{Enabled: value.Swap.Enabled, Path: value.Swap.Path, Size: value.Swap.Size}
	}
	if value.DockerCompose != nil {
		result.DockerCompose = &chartDockerCompose{Enabled: value.DockerCompose.Enabled, Version: value.DockerCompose.Version, SourceURL: value.DockerCompose.SourceURL, InstallPath: value.DockerCompose.InstallPath}
	}
	return result
}

func toServices(values []domain.ServiceSetting) []chartService {
	result := make([]chartService, 0, len(values))
	for _, value := range values {
		result = append(result, chartService{Name: value.Name, Enabled: value.Enabled, State: value.State})
	}
	return result
}

func toExtract(value *domain.ExtractProfile) *chartExtract {
	if value == nil {
		return nil
	}
	return &chartExtract{Enabled: value.Enabled, RemoteDir: value.RemoteDir, StripComponents: value.StripComponents, StagingPath: value.StagingPath, Sudo: value.Sudo, Owner: value.Owner, Group: value.Group, ChownRecursive: value.ChownRecursive, KeepStagedArchive: value.KeepStagedArchive}
}

func firstPort(value javaSpec) int {
	if len(value.Ports) == 0 {
		return 0
	}
	return value.Ports[0].ServicePort
}

func sortPackageFiles(files []PackageFile) {
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
}
