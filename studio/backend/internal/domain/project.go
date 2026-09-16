package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

type Project struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Revision    int64     `json:"revision"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type Environment string

const (
	EnvironmentDev  Environment = "dev"
	EnvironmentQA   Environment = "qa"
	EnvironmentProd Environment = "prod"
)

func ParseEnvironment(value string) (Environment, error) {
	environment := Environment(strings.ToLower(value))
	switch environment {
	case EnvironmentDev, EnvironmentQA, EnvironmentProd:
		return environment, nil
	default:
		return "", fmt.Errorf("unsupported environment %q", value)
	}
}

type Topology struct {
	ProjectID   string          `json:"projectId"`
	Environment Environment     `json:"environment"`
	Revision    int64           `json:"revision"`
	UpdatedAt   time.Time       `json:"updatedAt"`
	Blueprint   json.RawMessage `json:"blueprint"`
	Layout      json.RawMessage `json:"layout"`
}

func ValidateProject(name, description string) error {
	if strings.TrimSpace(name) == "" || len(name) > 128 {
		return errors.New("project name must contain 1 to 128 characters")
	}
	if len(description) > 2000 {
		return errors.New("project description must not exceed 2000 characters")
	}
	return nil
}

type blueprintDocument struct {
	APIVersion  string                `json:"apiVersion"`
	Deployment  *DeploymentProfile    `json:"deployment,omitempty"`
	Nodes       []blueprintNode       `json:"nodes"`
	Placements  []blueprintPlacement  `json:"placements"`
	Connections []blueprintConnection `json:"connections"`
}

type blueprintNode struct {
	ID   string          `json:"id"`
	Kind string          `json:"kind"`
	Name *string         `json:"name"`
	Spec json.RawMessage `json:"spec"`
}

type blueprintPlacement struct {
	WorkloadRef string `json:"workloadRef"`
	TargetRef   string `json:"targetRef"`
	Mode        string `json:"mode"`
}

type blueprintConnection struct {
	ID             string  `json:"id"`
	Kind           string  `json:"kind"`
	SourceRef      string  `json:"sourceRef"`
	TargetRef      string  `json:"targetRef"`
	TargetPortName *string `json:"targetPortName,omitempty"`
}

type layoutDocument struct {
	Nodes       []layoutNode       `json:"nodes"`
	Connections []layoutConnection `json:"connections,omitempty"`
	Viewport    viewport           `json:"viewport"`
}

type layoutNode struct {
	NodeID   string   `json:"nodeId"`
	Position position `json:"position"`
}

type layoutConnection struct {
	ConnectionID  string    `json:"connectionId"`
	BendOffset    *position `json:"bendOffset,omitempty"`
	LabelPosition *float64  `json:"labelPosition,omitempty"`
	// LabelOffset is accepted only for documents saved by the first movable-label draft.
	LabelOffset *position `json:"labelOffset,omitempty"`
}

type position struct {
	X *float64 `json:"x"`
	Y *float64 `json:"y"`
}

type viewport struct {
	X    *float64 `json:"x"`
	Y    *float64 `json:"y"`
	Zoom *float64 `json:"zoom"`
}

func ValidateTopologyStructure(blueprint, layout json.RawMessage, maxNodes, maxConnections int) error {
	var document blueprintDocument
	if err := decodeStrict(blueprint, &document); err != nil {
		return fmt.Errorf("invalid blueprint: %w", err)
	}
	if document.APIVersion != "studio.deployer/v1" {
		return errors.New("invalid blueprint: unsupported apiVersion")
	}
	if document.Nodes == nil || document.Placements == nil || document.Connections == nil {
		return errors.New("invalid blueprint: nodes, placements and connections are required")
	}
	if len(document.Nodes) > maxNodes || len(document.Connections) > maxConnections {
		return errors.New("invalid blueprint: topology limit exceeded")
	}
	allowedKinds := map[string]bool{"vm": true, "java": true, "redis": true, "elasticsearch": true, "database": true, "kubernetes": true, "registry": true, "ingress": true}
	for _, node := range document.Nodes {
		if node.ID == "" || !allowedKinds[node.Kind] || node.Name == nil || len(*node.Name) > 128 || !rawObject(node.Spec) {
			return errors.New("invalid blueprint: node fields are malformed")
		}
		if err := validateNodeSpec(node.Kind, node.Spec); err != nil {
			return fmt.Errorf("invalid blueprint: %s node spec: %w", node.Kind, err)
		}
	}
	if err := validateDeploymentStructure(document); err != nil {
		return fmt.Errorf("invalid blueprint: %w", err)
	}
	for _, placement := range document.Placements {
		if placement.WorkloadRef == "" || placement.TargetRef == "" || (placement.Mode != "process" && placement.Mode != "compose" && placement.Mode != "kubernetes") {
			return errors.New("invalid blueprint: placement fields are malformed")
		}
	}
	for _, connection := range document.Connections {
		if connection.ID == "" || connection.SourceRef == "" || connection.TargetRef == "" || (connection.Kind != "connects_to" && connection.Kind != "routes_to" && connection.Kind != "publishes_to") {
			return errors.New("invalid blueprint: connection fields are malformed")
		}
	}

	var layoutValue layoutDocument
	if err := decodeStrict(layout, &layoutValue); err != nil {
		return fmt.Errorf("invalid layout: %w", err)
	}
	if layoutValue.Nodes == nil || layoutValue.Viewport.X == nil || layoutValue.Viewport.Y == nil || layoutValue.Viewport.Zoom == nil || *layoutValue.Viewport.Zoom <= 0 {
		return errors.New("invalid layout: nodes and a positive viewport are required")
	}
	for _, node := range layoutValue.Nodes {
		if node.NodeID == "" || node.Position.X == nil || node.Position.Y == nil {
			return errors.New("invalid layout: node position is malformed")
		}
	}
	connectionIDs := make(map[string]struct{}, len(document.Connections))
	for _, connection := range document.Connections {
		connectionIDs[connection.ID] = struct{}{}
	}
	seenLayoutConnections := make(map[string]struct{}, len(layoutValue.Connections))
	for _, connection := range layoutValue.Connections {
		if connection.ConnectionID == "" || (connection.BendOffset == nil && connection.LabelPosition == nil && connection.LabelOffset == nil) {
			return errors.New("invalid layout: connection layout is malformed")
		}
		if _, ok := connectionIDs[connection.ConnectionID]; !ok {
			return errors.New("invalid layout: connection layout references an unknown connection")
		}
		if _, exists := seenLayoutConnections[connection.ConnectionID]; exists {
			return errors.New("invalid layout: duplicate connection layout")
		}
		seenLayoutConnections[connection.ConnectionID] = struct{}{}
		if !validLayoutPosition(connection.BendOffset) || !validLayoutPosition(connection.LabelOffset) {
			return errors.New("invalid layout: connection offset is malformed")
		}
		if connection.LabelPosition != nil && (*connection.LabelPosition < 0 || *connection.LabelPosition > 1) {
			return errors.New("invalid layout: connection label position is malformed")
		}
	}
	return nil
}

func validLayoutPosition(value *position) bool {
	if value == nil {
		return true
	}
	return value.X != nil && value.Y != nil && *value.X >= -1_000_000 && *value.X <= 1_000_000 && *value.Y >= -1_000_000 && *value.Y <= 1_000_000
}

func decodeStrict(body []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("multiple JSON values are not allowed")
	}
	return nil
}

func rawObject(value json.RawMessage) bool {
	var object map[string]json.RawMessage
	return json.Unmarshal(value, &object) == nil && object != nil
}

type portValue struct {
	Name        *string `json:"name"`
	ServicePort *int    `json:"servicePort"`
	Protocol    *string `json:"protocol"`
}

func validateNodeSpec(kind string, raw json.RawMessage) error {
	switch kind {
	case "vm":
		var spec struct {
			Provider   *string       `json:"provider"`
			Host       *string       `json:"host"`
			Deployment *VMDeployment `json:"deployment,omitempty"`
		}
		if err := decodeSpec(raw, &spec, "provider", "host"); err != nil {
			return err
		}
		if spec.Provider == nil || (*spec.Provider != "on-prem" && *spec.Provider != "aws-ec2") || spec.Host == nil {
			return errors.New("provider or host has an invalid type or value")
		}
	case "java":
		var spec struct {
			ArtifactRef json.RawMessage `json:"artifactRef"`
			JavaVersion *string         `json:"javaVersion"`
			MaxHeap     *string         `json:"maxHeap"`
			Ports       []portValue     `json:"ports"`
			Deployment  *JavaDeployment `json:"deployment,omitempty"`
		}
		if err := decodeSpec(raw, &spec, "artifactRef", "javaVersion", "maxHeap", "ports"); err != nil {
			return err
		}
		if spec.JavaVersion == nil || spec.MaxHeap == nil || spec.Ports == nil || !nullableString(spec.ArtifactRef) {
			return errors.New("artifactRef, javaVersion, maxHeap or ports has an invalid type")
		}
		return validatePorts(spec.Ports)
	case "redis", "elasticsearch":
		var spec struct {
			Location   *string            `json:"location"`
			Endpoint   *string            `json:"endpoint,omitempty"`
			Ports      []portValue        `json:"ports"`
			SecretRef  *string            `json:"secretRef,omitempty"`
			Deployment *UtilityDeployment `json:"deployment,omitempty"`
		}
		if err := decodeSpec(raw, &spec, "location", "ports"); err != nil {
			return err
		}
		if spec.Location == nil || (*spec.Location != "hosted" && *spec.Location != "external") || spec.Ports == nil {
			return errors.New("location or ports has an invalid type or value")
		}
		return validatePorts(spec.Ports)
	case "database":
		var spec struct {
			Engine    *string     `json:"engine"`
			Endpoint  *string     `json:"endpoint"`
			Ports     []portValue `json:"ports"`
			SecretRef *string     `json:"secretRef,omitempty"`
		}
		if err := decodeSpec(raw, &spec, "engine", "endpoint", "ports"); err != nil {
			return err
		}
		if spec.Engine == nil || *spec.Engine != "postgresql" || spec.Endpoint == nil || spec.Ports == nil {
			return errors.New("engine, endpoint or ports has an invalid type or value")
		}
		return validatePorts(spec.Ports)
	case "kubernetes":
		var spec struct {
			Namespace *string `json:"namespace"`
		}
		if err := decodeSpec(raw, &spec, "namespace"); err != nil || spec.Namespace == nil {
			return errors.New("namespace is required and must be a string")
		}
	case "registry":
		var spec struct {
			Endpoint  *string `json:"endpoint"`
			SecretRef *string `json:"secretRef,omitempty"`
		}
		if err := decodeSpec(raw, &spec, "endpoint"); err != nil || spec.Endpoint == nil {
			return errors.New("endpoint is required and must be a string")
		}
	case "ingress":
		var spec struct {
			Host *string `json:"host"`
		}
		if err := decodeSpec(raw, &spec, "host"); err != nil || spec.Host == nil {
			return errors.New("host is required and must be a string")
		}
	}
	return nil
}

func decodeSpec(raw json.RawMessage, destination any, required ...string) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return errors.New("spec must be an object")
	}
	for _, field := range required {
		if _, exists := fields[field]; !exists {
			return fmt.Errorf("field %s is required", field)
		}
	}
	return decodeStrict(raw, destination)
}

func nullableString(raw json.RawMessage) bool {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return true
	}
	var value string
	return json.Unmarshal(raw, &value) == nil
}

func validatePorts(ports []portValue) error {
	for _, port := range ports {
		if port.Name == nil || port.ServicePort == nil || *port.ServicePort < 1 || *port.ServicePort > 65535 || port.Protocol == nil || (*port.Protocol != "TCP" && *port.Protocol != "UDP") {
			return errors.New("port fields have an invalid type or value")
		}
	}
	return nil
}
