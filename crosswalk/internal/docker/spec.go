package docker

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"go-deployer/internal/config"
	"go-deployer/internal/ssh"
)

const (
	managedLabel         = "io.crosswalk.managed"
	environmentKeysLabel = "io.crosswalk.environment-keys"
)

type canonicalContainerSpec struct {
	Image       string          `json:"image"`
	Ports       []canonicalPort `json:"ports"`
	Environment []canonicalEnv  `json:"environment"`
	Restart     string          `json:"restart"`
}

type canonicalPort struct {
	HostIP    string `json:"host_ip"`
	Host      int    `json:"host"`
	Container int    `json:"container"`
	Protocol  string `json:"protocol"`
}

type canonicalEnv struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	Present bool   `json:"present"`
}

type dockerInspectContainer struct {
	Config struct {
		Env    []string          `json:"Env"`
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
	HostConfig struct {
		PortBindings map[string][]struct {
			HostIP   string `json:"HostIp"`
			HostPort string `json:"HostPort"`
		} `json:"PortBindings"`
		RestartPolicy struct {
			Name string `json:"Name"`
		} `json:"RestartPolicy"`
	} `json:"HostConfig"`
}

func desiredContainerSpec(registry config.DockerRegistryConfig, container config.ContainerConfig) canonicalContainerSpec {
	ports := make([]canonicalPort, 0, len(container.Ports))
	for _, port := range container.Ports {
		ports = append(ports, canonicalPort{
			HostIP: "*", Host: port.Host, Container: port.Container, Protocol: port.Protocol,
		})
	}
	environment := make([]canonicalEnv, 0, len(container.Environment))
	for name, value := range container.Environment {
		environment = append(environment, canonicalEnv{Name: name, Value: value, Present: true})
	}
	sortCanonicalSpec(ports, environment)
	return canonicalContainerSpec{
		Image:       registry.ImageReference(container),
		Ports:       ports,
		Environment: environment,
		Restart:     normalizeRestartPolicy(container.RestartPolicy),
	}
}

// inspectActualContainerSpec은 Docker inspect 결과에서 Crosswalk가 관리하는 필드만 정규화한다.
// environment 값은 비교 과정의 메모리에만 존재하며 label, log, error에 포함하지 않는다.
func inspectActualContainerSpec(runner remoteRunner, name, desiredImage string, image containerImageState) (canonicalContainerSpec, bool, error) {
	command := "docker container inspect " + ssh.ShellQuote(name)
	output, err := runner.Run(command)
	if err != nil {
		return canonicalContainerSpec{}, false, fmt.Errorf("failed to inspect container %s configuration", name)
	}
	var records []dockerInspectContainer
	if err := json.Unmarshal([]byte(output), &records); err != nil || len(records) != 1 {
		return canonicalContainerSpec{}, false, fmt.Errorf("container %s returned invalid configuration metadata", name)
	}
	record := records[0]
	managed := record.Config.Labels[managedLabel] == "true"

	actualImage := image.ImageID
	if containsDigest(image.RepoDigests, desiredImage) {
		actualImage = desiredImage
	}
	ports, err := canonicalPorts(record.HostConfig.PortBindings)
	if err != nil {
		return canonicalContainerSpec{}, false, fmt.Errorf("container %s returned invalid port bindings", name)
	}
	environment := canonicalEnvironment(record.Config.Env, record.Config.Labels[environmentKeysLabel])
	sortCanonicalSpec(ports, environment)

	return canonicalContainerSpec{
		Image:       actualImage,
		Ports:       ports,
		Environment: environment,
		Restart:     normalizeRestartPolicy(record.HostConfig.RestartPolicy.Name),
	}, managed, nil
}

func canonicalPorts(bindings map[string][]struct {
	HostIP   string `json:"HostIp"`
	HostPort string `json:"HostPort"`
}) ([]canonicalPort, error) {
	ports := make([]canonicalPort, 0, len(bindings))
	seen := make(map[canonicalPort]struct{})
	for containerBinding, hostBindings := range bindings {
		containerPortText, protocol, found := strings.Cut(containerBinding, "/")
		if !found {
			return nil, errors.New("missing container port protocol")
		}
		containerPort, err := strconv.Atoi(containerPortText)
		if err != nil {
			return nil, err
		}
		for _, binding := range hostBindings {
			hostPort, err := strconv.Atoi(binding.HostPort)
			if err != nil {
				return nil, err
			}
			port := canonicalPort{
				HostIP: normalizeHostIP(binding.HostIP), Host: hostPort, Container: containerPort, Protocol: protocol,
			}
			if _, exists := seen[port]; !exists {
				seen[port] = struct{}{}
				ports = append(ports, port)
			}
		}
	}
	return ports, nil
}

func canonicalEnvironment(values []string, managedKeys string) []canonicalEnv {
	actual := make(map[string]string, len(values))
	for _, value := range values {
		name, envValue, found := strings.Cut(value, "=")
		if found {
			actual[name] = envValue
		}
	}
	if managedKeys == "" {
		return []canonicalEnv{}
	}
	keys := strings.Split(managedKeys, ",")
	environment := make([]canonicalEnv, 0, len(keys))
	for _, name := range keys {
		value, exists := actual[name]
		environment = append(environment, canonicalEnv{Name: name, Value: value, Present: exists})
	}
	return environment
}

func canonicalSpecHash(spec canonicalContainerSpec) ([sha256.Size]byte, error) {
	data, err := json.Marshal(spec)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	return sha256.Sum256(data), nil
}

func sameCanonicalSpec(left, right canonicalContainerSpec) (bool, error) {
	leftHash, err := canonicalSpecHash(left)
	if err != nil {
		return false, err
	}
	rightHash, err := canonicalSpecHash(right)
	if err != nil {
		return false, err
	}
	return leftHash == rightHash, nil
}

func sortCanonicalSpec(ports []canonicalPort, environment []canonicalEnv) {
	sort.Slice(ports, func(i, j int) bool {
		left := fmt.Sprintf("%s:%05d:%05d:%s", ports[i].HostIP, ports[i].Host, ports[i].Container, ports[i].Protocol)
		right := fmt.Sprintf("%s:%05d:%05d:%s", ports[j].HostIP, ports[j].Host, ports[j].Container, ports[j].Protocol)
		return left < right
	})
	sort.Slice(environment, func(i, j int) bool { return environment[i].Name < environment[j].Name })
}

func normalizeHostIP(value string) string {
	switch value {
	case "", "0.0.0.0", "::":
		return "*"
	default:
		return value
	}
}

func normalizeRestartPolicy(value string) string {
	if value == "" {
		return "no"
	}
	return value
}
