package crosswalkcompose

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"studio-backend/internal/generator/crosswalkdocker"
)

const ID = "crosswalk-compose"
const Version = "1.0.0"
const ChartFormat = "crosswalk/v1"

var composeNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

type File struct {
	Path string
	Body []byte
}

type Bundle struct {
	DeployYAML []byte
	Files      []File
}

// Generate renders Compose directly from the common validated container model.
func Generate(raw json.RawMessage, environment string) (Bundle, error) {
	source, err := crosswalkdocker.BuildTargets(raw, environment)
	if err != nil {
		return Bundle{}, err
	}
	if len(source.Servers) == 0 {
		return Bundle{}, errors.New("crosswalk-compose requires at least one container server")
	}

	result := composeChart{Format: ChartFormat, SSH: source.SSH, Bastion: source.Bastion, Docker: source.Docker}
	files := make([]File, 0, len(source.Servers))
	for _, server := range source.Servers {
		projectName := strings.ToLower(strings.TrimSpace(server.Name))
		if !composeNamePattern.MatchString(projectName) {
			return Bundle{}, fmt.Errorf("server %q cannot be used as a Compose project name", server.Name)
		}
		services := make(map[string]composeService, len(server.Containers))
		for _, container := range server.Containers {
			if !composeNamePattern.MatchString(container.Name) {
				return Bundle{}, fmt.Errorf("container %q cannot be used as a Compose service name", container.Name)
			}
			if _, exists := services[container.Name]; exists {
				return Bundle{}, fmt.Errorf("Compose service name %q is duplicated on server %s", container.Name, server.Name)
			}
			restart := container.RestartPolicy
			if restart == "" {
				restart = "unless-stopped"
			}
			service := composeService{
				Image:       source.Docker.Registry.Host + "/" + container.Image + "@" + container.Digest,
				Environment: cloneMap(container.Environment),
				Restart:     restart,
			}
			for _, port := range container.Ports {
				protocol := strings.ToLower(strings.TrimSpace(port.Protocol))
				if protocol == "" {
					protocol = "tcp"
				}
				service.Ports = append(service.Ports, fmt.Sprintf("%d:%d/%s", port.Host, port.Container, protocol))
			}
			services[container.Name] = service
		}
		composeBody, err := yaml.Marshal(composeDocument{Name: projectName, Services: services})
		if err != nil {
			return Bundle{}, fmt.Errorf("render Compose project %s: %w", projectName, err)
		}
		filePath := "compose/" + projectName + "/compose.yml"
		files = append(files, File{Path: filePath, Body: composeBody})
		result.Servers = append(result.Servers, composeServer{
			Host: server.Host, Name: server.Name, Tags: append([]string(nil), server.Tags...),
			SSHPort: server.SSHPort, BastionHost: server.BastionHost, BastionSSHPort: server.BastionSSHPort,
			ComposeProjects: []composeProject{{Name: projectName, LocalPath: "./" + filePath, RemoteDir: "/app/overpass/compose/" + projectName}},
		})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	deployYAML, err := yaml.Marshal(result)
	if err != nil {
		return Bundle{}, fmt.Errorf("render deploy.yml: %w", err)
	}
	return Bundle{DeployYAML: deployYAML, Files: files}, nil
}

func cloneMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]string, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}

func WriteZIP(w io.Writer, bundle Bundle) error {
	archive := zip.NewWriter(w)
	values := append([]File{{Path: "deploy.yml", Body: bundle.DeployYAML}}, bundle.Files...)
	for _, file := range values {
		if strings.HasPrefix(file.Path, "/") || strings.Contains(file.Path, "..") {
			return fmt.Errorf("unsafe ZIP path %q", file.Path)
		}
		entry, err := archive.Create(file.Path)
		if err != nil {
			return fmt.Errorf("create ZIP entry %s: %w", file.Path, err)
		}
		if _, err := io.Copy(entry, bytes.NewReader(file.Body)); err != nil {
			return fmt.Errorf("write ZIP entry %s: %w", file.Path, err)
		}
	}
	if err := archive.Close(); err != nil {
		return fmt.Errorf("close ZIP: %w", err)
	}
	return nil
}
