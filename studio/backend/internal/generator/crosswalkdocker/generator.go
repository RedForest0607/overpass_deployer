package crosswalkdocker

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"studio-backend/internal/domain"
)

const ID = "crosswalk-docker"
const Version = "1.0.0"
const ChartFormat = "crosswalk/v1"

var repositoryPattern = regexp.MustCompile(`^[a-z0-9]+(?:[._-][a-z0-9]+)*(?:/[a-z0-9]+(?:[._-][a-z0-9]+)*)+$`)
var digestPattern = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
var tokenEnvironmentPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
var registryHostPattern = regexp.MustCompile(`^[a-z0-9.-]+$`)
var registryUsernamePattern = regexp.MustCompile(`^[A-Za-z0-9_.@+$-]+$`)

type Bundle struct {
	DeployYAML []byte
}

type blueprint struct {
	APIVersion  string                    `json:"apiVersion"`
	Deployment  *domain.DeploymentProfile `json:"deployment,omitempty"`
	Nodes       []node                    `json:"nodes"`
	Placements  []placement               `json:"placements"`
	Connections []json.RawMessage         `json:"connections"`
}

type node struct {
	ID   string          `json:"id"`
	Kind string          `json:"kind"`
	Name string          `json:"name"`
	Spec json.RawMessage `json:"spec"`
}

type placement struct {
	WorkloadRef string `json:"workloadRef"`
	TargetRef   string `json:"targetRef"`
	Mode        string `json:"mode"`
}

type vmSpec struct {
	Host       string               `json:"host"`
	Deployment *domain.VMDeployment `json:"deployment,omitempty"`
}

type registrySpec struct {
	Endpoint string `json:"endpoint"`
	Username string `json:"username"`
	TokenEnv string `json:"tokenEnv"`
}

type containerSpec struct {
	Image  string `json:"image"`
	Digest string `json:"digest"`
	Ports  []struct {
		ServicePort   int    `json:"servicePort"`
		ContainerPort int    `json:"containerPort"`
		Protocol      string `json:"protocol"`
	} `json:"ports"`
	Deployment *domain.ContainerDeployment `json:"deployment,omitempty"`
}

func Generate(raw json.RawMessage, environment string) (Bundle, error) {
	var document blueprint
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return Bundle{}, fmt.Errorf("decode blueprint: %w", err)
	}
	if document.APIVersion != "studio.deployer/v1" {
		return Bundle{}, errors.New("unsupported blueprint version")
	}
	if document.Deployment == nil || strings.TrimSpace(document.Deployment.SSH.User) == "" || strings.TrimSpace(document.Deployment.SSH.KeyPath) == "" {
		return Bundle{}, errors.New("deployment.ssh.user and keyPath are required")
	}

	byID := make(map[string]node, len(document.Nodes))
	servers := make(map[string]*chartServer)
	serverOrder := make([]string, 0)
	var registry *chartRegistry
	for _, item := range document.Nodes {
		byID[item.ID] = item
		switch item.Kind {
		case "vm":
			var spec vmSpec
			if err := json.Unmarshal(item.Spec, &spec); err != nil {
				return Bundle{}, fmt.Errorf("decode VM %s: %w", item.Name, err)
			}
			server := chartServer{Host: spec.Host, Name: item.Name, Tags: []string{strings.ToLower(environment)}}
			if spec.Deployment != nil {
				server.Tags = fallbackTags(spec.Deployment.Tags, environment)
				server.SSHPort = spec.Deployment.SSHPort
				server.BastionHost = spec.Deployment.BastionHost
				server.BastionSSHPort = spec.Deployment.BastionSSHPort
			}
			servers[item.ID] = &server
			serverOrder = append(serverOrder, item.ID)
		case "registry":
			if registry != nil {
				return Bundle{}, errors.New("crosswalk-docker requires exactly one registry node")
			}
			var spec registrySpec
			if err := json.Unmarshal(item.Spec, &spec); err != nil {
				return Bundle{}, fmt.Errorf("decode registry %s: %w", item.Name, err)
			}
			endpoint := strings.ToLower(strings.TrimSpace(spec.Endpoint))
			if !validRegistryHost(endpoint) {
				return Bundle{}, errors.New("registry endpoint must be a host with an optional port, without a URL scheme")
			}
			username := strings.TrimSpace(spec.Username)
			if !registryUsernamePattern.MatchString(username) || !tokenEnvironmentPattern.MatchString(spec.TokenEnv) {
				return Bundle{}, errors.New("registry username and tokenEnv are required")
			}
			registry = &chartRegistry{Host: endpoint, Username: username, TokenEnv: spec.TokenEnv}
		}
	}
	if registry == nil {
		return Bundle{}, errors.New("crosswalk-docker requires exactly one registry node")
	}

	containerCount := 0
	for _, item := range document.Placements {
		if item.Mode != "container" {
			continue
		}
		server := servers[item.TargetRef]
		if server == nil {
			return Bundle{}, fmt.Errorf("container placement target %q is not a VM", item.TargetRef)
		}
		workload, exists := byID[item.WorkloadRef]
		if !exists || workload.Kind != "container" {
			return Bundle{}, fmt.Errorf("container placement workload %q is not a container", item.WorkloadRef)
		}
		container, err := buildContainer(workload)
		if err != nil {
			return Bundle{}, err
		}
		server.Containers = append(server.Containers, container)
		containerCount++
	}
	if containerCount == 0 {
		return Bundle{}, errors.New("at least one container placement is required")
	}

	profile := document.Deployment
	value := chart{
		Format:  ChartFormat,
		SSH:     chartSSH{User: profile.SSH.User, KeyPath: profile.SSH.KeyPath, KnownHostsPath: profile.SSH.KnownHostsPath, HostKeyChecking: profile.SSH.HostKeyChecking, Port: profile.SSH.Port, TimeoutSeconds: profile.SSH.TimeoutSeconds},
		Bastion: toBastion(profile.Bastion),
		Docker:  chartDocker{Registry: *registry},
	}
	for _, id := range serverOrder {
		if len(servers[id].Containers) > 0 {
			value.Servers = append(value.Servers, *servers[id])
		}
	}
	body, err := yaml.Marshal(value)
	if err != nil {
		return Bundle{}, fmt.Errorf("render deploy.yml: %w", err)
	}
	return Bundle{DeployYAML: body}, nil
}

func validRegistryHost(value string) bool {
	host := value
	if strings.Count(value, ":") > 1 {
		return false
	}
	if before, port, found := strings.Cut(value, ":"); found {
		host = before
		parsed, err := strconv.Atoi(port)
		if err != nil || parsed < 1 || parsed > 65535 {
			return false
		}
	}
	if !registryHostPattern.MatchString(host) || strings.Contains(host, "..") {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
	}
	return true
}

func buildContainer(item node) (chartContainer, error) {
	var spec containerSpec
	if err := json.Unmarshal(item.Spec, &spec); err != nil {
		return chartContainer{}, fmt.Errorf("decode container %s: %w", item.Name, err)
	}
	if !repositoryPattern.MatchString(spec.Image) || !digestPattern.MatchString(spec.Digest) {
		return chartContainer{}, fmt.Errorf("container %s image repository or digest is invalid", item.Name)
	}
	result := chartContainer{Name: item.Name, Image: spec.Image, Digest: spec.Digest}
	if spec.Deployment != nil {
		result.Tags = append([]string(nil), spec.Deployment.Tags...)
		result.Environment = cloneMap(spec.Deployment.Environment)
		result.RestartPolicy = spec.Deployment.RestartPolicy
		result.HealthTimeoutSeconds = spec.Deployment.HealthTimeoutSeconds
	}
	for _, port := range spec.Ports {
		result.Ports = append(result.Ports, chartPort{Host: port.ServicePort, Container: port.ContainerPort, Protocol: strings.ToLower(port.Protocol)})
	}
	return result, nil
}

func fallbackTags(tags []string, environment string) []string {
	if len(tags) > 0 {
		return append([]string(nil), tags...)
	}
	return []string{strings.ToLower(environment)}
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

func toBastion(value *domain.BastionProfile) *chartBastion {
	if value == nil {
		return nil
	}
	return &chartBastion{Host: value.Host, User: value.User, KeyPath: value.KeyPath, KnownHostsPath: value.KnownHostsPath, HostKeyChecking: value.HostKeyChecking, Port: value.Port, TimeoutSeconds: value.TimeoutSeconds, AliasUser: value.AliasUser, SSHConfigPath: value.SSHConfigPath, ShellAliasesPath: value.ShellAliasesPath, TargetKnownHostsPath: value.TargetKnownHostsPath, TargetKeyPath: value.TargetKeyPath}
}

func WriteZIP(w io.Writer, bundle Bundle) error {
	archive := zip.NewWriter(w)
	entry, err := archive.Create("deploy.yml")
	if err != nil {
		return fmt.Errorf("create deploy.yml ZIP entry: %w", err)
	}
	if _, err := entry.Write(bundle.DeployYAML); err != nil {
		return fmt.Errorf("write deploy.yml ZIP entry: %w", err)
	}
	if err := archive.Close(); err != nil {
		return fmt.Errorf("close ZIP: %w", err)
	}
	return nil
}
