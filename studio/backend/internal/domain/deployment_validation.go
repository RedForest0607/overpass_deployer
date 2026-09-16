package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"
)

var chmodMode = regexp.MustCompile(`^0?[0-7]{3}$`)

func validateDeploymentStructure(document blueprintDocument) error {
	if document.Deployment != nil {
		profile := document.Deployment
		if profile.SSH.HostKeyChecking != "" && profile.SSH.HostKeyChecking != "strict" && profile.SSH.HostKeyChecking != "accept-new" && profile.SSH.HostKeyChecking != "insecure" {
			return errors.New("deployment.ssh.hostKeyChecking is invalid")
		}
		if err := optionalPort(profile.SSH.Port, "deployment.ssh.port"); err != nil {
			return err
		}
		if profile.Bastion != nil {
			if strings.TrimSpace(profile.Bastion.Host) == "" {
				return errors.New("deployment.bastion.host is required")
			}
			if err := optionalPort(profile.Bastion.Port, "deployment.bastion.port"); err != nil {
				return err
			}
		}
		for i, rule := range profile.DirectoryDefaults {
			if err := absolutePath(rule.Prefix, fmt.Sprintf("deployment.directoryDefaults[%d].prefix", i)); err != nil {
				return err
			}
			if strings.TrimSpace(rule.Owner) == "" {
				return fmt.Errorf("deployment.directoryDefaults[%d].owner is required", i)
			}
		}
		if profile.Bootstrap != nil {
			if err := validateServices(profile.Bootstrap.Services, "deployment.bootstrap.services"); err != nil {
				return err
			}
		}
	}

	for nodeIndex, node := range document.Nodes {
		switch node.Kind {
		case "vm":
			var spec struct {
				Deployment *VMDeployment `json:"deployment,omitempty"`
			}
			if err := json.Unmarshal(node.Spec, &spec); err != nil {
				continue
			}
			if spec.Deployment == nil {
				continue
			}
			prefix := fmt.Sprintf("nodes[%d].spec.deployment", nodeIndex)
			if err := optionalPort(spec.Deployment.SSHPort, prefix+".sshPort"); err != nil {
				return err
			}
			if err := optionalPort(spec.Deployment.BastionSSHPort, prefix+".bastionSSHPort"); err != nil {
				return err
			}
			for i, directory := range spec.Deployment.Directories {
				if err := absolutePath(directory, fmt.Sprintf("%s.directories[%d]", prefix, i)); err != nil {
					return err
				}
			}
			if err := validateDeliveries(spec.Deployment.Files, prefix+".files"); err != nil {
				return err
			}
			for i, utility := range spec.Deployment.Utilities {
				utilityPrefix := fmt.Sprintf("%s.utilities[%d]", prefix, i)
				if strings.TrimSpace(utility.Name) == "" {
					return fmt.Errorf("%s.name is required", utilityPrefix)
				}
				for j, directory := range utility.Directories {
					if err := absolutePath(directory, fmt.Sprintf("%s.directories[%d]", utilityPrefix, j)); err != nil {
						return err
					}
				}
				if err := validateDeliveries(utility.Files, utilityPrefix+".files"); err != nil {
					return err
				}
				if err := validateServices(utility.Services, utilityPrefix+".services"); err != nil {
					return err
				}
			}
			if spec.Deployment.Bootstrap != nil {
				if err := validateServices(spec.Deployment.Bootstrap.Services, prefix+".bootstrap.services"); err != nil {
					return err
				}
			}
		case "redis", "elasticsearch":
			var spec struct {
				Deployment *UtilityDeployment `json:"deployment,omitempty"`
			}
			if err := json.Unmarshal(node.Spec, &spec); err != nil || spec.Deployment == nil {
				continue
			}
			prefix := fmt.Sprintf("nodes[%d].spec.deployment", nodeIndex)
			for i, directory := range spec.Deployment.Directories {
				if err := absolutePath(directory, fmt.Sprintf("%s.directories[%d]", prefix, i)); err != nil {
					return err
				}
			}
			if err := validateDeliveries(spec.Deployment.Files, prefix+".files"); err != nil {
				return err
			}
			if err := validateServices(spec.Deployment.Services, prefix+".services"); err != nil {
				return err
			}
		case "java":
			var spec struct {
				Deployment *JavaDeployment `json:"deployment,omitempty"`
			}
			if err := json.Unmarshal(node.Spec, &spec); err != nil {
				continue
			}
			if spec.Deployment == nil {
				continue
			}
			prefix := fmt.Sprintf("nodes[%d].spec.deployment", nodeIndex)
			for field, value := range map[string]string{"baseDir": spec.Deployment.BaseDir, "jarRemotePath": spec.Deployment.JarRemotePath} {
				if value != "" {
					if err := absolutePath(value, prefix+"."+field); err != nil {
						return err
					}
				}
			}
			if spec.Deployment.JarPackagePath != "" {
				if err := packagePath(spec.Deployment.JarPackagePath, prefix+".jarPackagePath"); err != nil {
					return err
				}
			}
			if err := validateConfigDeliveries(spec.Deployment.ConfigFiles, prefix+".configFiles"); err != nil {
				return err
			}
			if err := validateDeliveries(spec.Deployment.ExtraFiles, prefix+".extraFiles"); err != nil {
				return err
			}
			if script := spec.Deployment.Script; script != nil {
				if script.Mode != "template" && script.Mode != "local-file" {
					return fmt.Errorf("%s.script.mode must be template or local-file", prefix)
				}
				if err := absolutePath(script.RemotePath, prefix+".script.remotePath"); err != nil {
					return err
				}
				if script.PackagePath != "" {
					if err := packagePath(script.PackagePath, prefix+".script.packagePath"); err != nil {
						return err
					}
				}
				if script.ValuesPackagePath != "" {
					if err := packagePath(script.ValuesPackagePath, prefix+".script.valuesPackagePath"); err != nil {
						return err
					}
				}
				if script.Mode == "local-file" && script.ArtifactRef == "" {
					return fmt.Errorf("%s.script.artifactRef is required for local-file mode", prefix)
				}
				if script.Mode == "local-file" && (script.Template != "" || script.ValuesArtifactRef != "") {
					return fmt.Errorf("%s.script template and valuesArtifactRef cannot be used for local-file mode", prefix)
				}
			}
		}
	}
	return nil
}

func validateConfigDeliveries(values []ConfigArtifactDelivery, prefix string) error {
	for i, value := range values {
		field := fmt.Sprintf("%s[%d]", prefix, i)
		if strings.TrimSpace(value.ArtifactRef) == "" {
			return fmt.Errorf("%s.artifactRef is required", field)
		}
		if err := absolutePath(value.RemotePath, field+".remotePath"); err != nil {
			return err
		}
		if value.PackagePath != "" {
			if err := packagePath(value.PackagePath, field+".packagePath"); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateDeliveries(values []ArtifactDelivery, prefix string) error {
	for i, value := range values {
		field := fmt.Sprintf("%s[%d]", prefix, i)
		if strings.TrimSpace(value.ArtifactRef) == "" {
			return fmt.Errorf("%s.artifactRef is required", field)
		}
		if err := absolutePath(value.RemotePath, field+".remotePath"); err != nil {
			return err
		}
		if value.PackagePath != "" {
			if err := packagePath(value.PackagePath, field+".packagePath"); err != nil {
				return err
			}
		}
		if value.Chmod != "" && !chmodMode.MatchString(value.Chmod) {
			return fmt.Errorf("%s.chmod must be an octal file mode", field)
		}
		if value.Extract != nil && value.Extract.Enabled {
			if err := absolutePath(value.Extract.RemoteDir, field+".extract.remoteDir"); err != nil {
				return err
			}
			if value.Extract.StripComponents < 0 {
				return fmt.Errorf("%s.extract.stripComponents must be zero or greater", field)
			}
		}
	}
	return nil
}

func validateServices(values []ServiceSetting, prefix string) error {
	for i, value := range values {
		if strings.TrimSpace(value.Name) == "" {
			return fmt.Errorf("%s[%d].name is required", prefix, i)
		}
		if value.State != "" && value.State != "started" && value.State != "stopped" && value.State != "restarted" {
			return fmt.Errorf("%s[%d].state is invalid", prefix, i)
		}
	}
	return nil
}

func optionalPort(value int, field string) error {
	if value < 0 || value > 65535 {
		return fmt.Errorf("%s must be between 1 and 65535 when set", field)
	}
	return nil
}
func absolutePath(value, field string) error {
	if value == "" || !strings.HasPrefix(value, "/") || path.Clean(value) != value {
		return fmt.Errorf("%s must be a clean absolute path", field)
	}
	return nil
}
func packagePath(value, field string) error {
	if strings.Contains(value, "\\") || strings.HasPrefix(value, "/") || path.Clean(value) != value || value == "." || value == ".." || strings.HasPrefix(value, "../") {
		return fmt.Errorf("%s must be a safe ZIP-relative path", field)
	}
	return nil
}
