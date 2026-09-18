package domain

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

type Issue struct {
	Code          string   `json:"code"`
	Severity      string   `json:"severity"`
	Stage         string   `json:"stage"`
	NodeIDs       []string `json:"nodeIds"`
	ConnectionIDs []string `json:"connectionIds"`
	Path          string   `json:"path"`
	Message       string   `json:"message"`
}

type Validation struct {
	ClientRequestID string  `json:"clientRequestId"`
	Valid           bool    `json:"valid"`
	Issues          []Issue `json:"issues"`
}

type serviceSpec struct {
	Host        string  `json:"host"`
	Endpoint    string  `json:"endpoint"`
	Username    string  `json:"username"`
	TokenEnv    string  `json:"tokenEnv"`
	SecretRef   string  `json:"secretRef"`
	Location    string  `json:"location"`
	Namespace   string  `json:"namespace"`
	JavaVersion string  `json:"javaVersion"`
	MaxHeap     string  `json:"maxHeap"`
	Image       string  `json:"image"`
	Digest      string  `json:"digest"`
	ArtifactRef *string `json:"artifactRef"`
	Ports       []struct {
		Name          string `json:"name"`
		ServicePort   int    `json:"servicePort"`
		ContainerPort int    `json:"containerPort"`
		Protocol      string `json:"protocol"`
	} `json:"ports"`
}

var resourceName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
var heapSize = regexp.MustCompile(`^[1-9][0-9]*[mMgG]$`)
var containerImage = regexp.MustCompile(`^[a-z0-9]+(?:[._-][a-z0-9]+)*(?:/[a-z0-9]+(?:[._-][a-z0-9]+)*)+$`)
var containerDigest = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
var environmentName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// ValidateDesign runs only after structural validation. It never accesses a host.
func ValidateDesign(raw json.RawMessage, requestID string) Validation {
	var doc blueprintDocument
	_ = json.Unmarshal(raw, &doc)
	result := Validation{ClientRequestID: requestID, Valid: true, Issues: []Issue{}}
	add := func(code, stage, path, message, nodeID, edgeID, severity string) {
		issue := Issue{Code: code, Stage: stage, Path: path, Message: message, Severity: severity, NodeIDs: []string{}, ConnectionIDs: []string{}}
		if nodeID != "" {
			issue.NodeIDs = append(issue.NodeIDs, nodeID)
		}
		if edgeID != "" {
			issue.ConnectionIDs = append(issue.ConnectionIDs, edgeID)
		}
		result.Issues = append(result.Issues, issue)
		if severity == "error" {
			result.Valid = false
		}
	}
	if len(doc.Nodes) == 0 {
		add("EMPTY_TOPOLOGY", "model", "/nodes", "배포할 서비스를 추가하세요.", "", "", "error")
	}
	nodes := map[string]blueprintNode{}
	specs := map[string]serviceSpec{}
	names := map[string]bool{}
	for i, n := range doc.Nodes {
		path := fmt.Sprintf("/nodes/%d", i)
		if _, exists := nodes[n.ID]; exists {
			add("DUPLICATE_ID", "model", path+"/id", "노드 ID가 중복됩니다.", n.ID, "", "error")
		}
		nodes[n.ID] = n
		name := *n.Name
		if !resourceName.MatchString(name) || names[name] {
			add("INVALID_NAME", "model", path+"/name", "이름은 고유한 소문자·숫자·하이픈 조합이어야 합니다.", n.ID, "", "error")
		}
		names[name] = true
		var spec serviceSpec
		_ = json.Unmarshal(n.Spec, &spec)
		specs[n.ID] = spec
		required := ""
		switch n.Kind {
		case "vm", "ingress":
			if strings.TrimSpace(spec.Host) == "" {
				required = "host"
			}
		case "database", "registry":
			if strings.TrimSpace(spec.Endpoint) == "" {
				required = "endpoint"
			}
		case "redis", "elasticsearch":
			if spec.Location == "external" && strings.TrimSpace(spec.Endpoint) == "" {
				required = "endpoint"
			}
		case "kubernetes":
			if strings.TrimSpace(spec.Namespace) == "" {
				required = "namespace"
			}
		}
		if required != "" {
			add("REQUIRED_FIELD", "model", path+"/spec/"+required, "필수 주소 또는 namespace를 입력하세요.", n.ID, "", "error")
		}
		portNames := map[string]bool{}
		for j, p := range spec.Ports {
			if p.Name == "" || portNames[p.Name] {
				add("INVALID_PORT_NAME", "model", fmt.Sprintf("%s/spec/ports/%d/name", path, j), "포트 이름은 비어 있지 않고 노드 내에서 고유해야 합니다.", n.ID, "", "error")
			}
			portNames[p.Name] = true
		}
		if n.Kind == "java" {
			if spec.JavaVersion != "17" && spec.JavaVersion != "21" && spec.JavaVersion != "25" {
				add("JAVA_VERSION", "deployment", path+"/spec/javaVersion", "지원 Java 버전은 17, 21, 25입니다.", n.ID, "", "error")
			}
			if !heapSize.MatchString(spec.MaxHeap) {
				add("JVM_HEAP", "deployment", path+"/spec/maxHeap", "Heap은 512m, 1g처럼 양수와 단위로 입력하세요.", n.ID, "", "error")
			}
			if spec.ArtifactRef == nil || *spec.ArtifactRef == "" {
				add("ARTIFACT_MISSING", "artifact", path+"/spec/artifactRef", "JAR 업로드 연동 전입니다. 파일명만으로 배포 자산이 등록되지 않습니다.", n.ID, "", "warning")
			}
		}
		if n.Kind == "container" {
			if !containerImage.MatchString(spec.Image) {
				add("CONTAINER_IMAGE", "deployment", path+"/spec/image", "이미지는 registry host와 tag를 제외한 소문자 repository 경로여야 합니다.", n.ID, "", "error")
			}
			if !containerDigest.MatchString(spec.Digest) {
				add("CONTAINER_DIGEST", "deployment", path+"/spec/digest", "이미지 digest는 sha256:<64 hex> 형식이어야 합니다.", n.ID, "", "error")
			}
		}
		if n.Kind == "registry" {
			hasDockerCredentials := strings.TrimSpace(spec.Username) != "" || strings.TrimSpace(spec.TokenEnv) != ""
			if hasDockerCredentials {
				if strings.TrimSpace(spec.Username) == "" {
					add("REGISTRY_USERNAME", "deployment", path+"/spec/username", "Registry 사용자 이름을 입력하세요.", n.ID, "", "error")
				}
				if !environmentName.MatchString(spec.TokenEnv) {
					add("REGISTRY_TOKEN_ENV", "deployment", path+"/spec/tokenEnv", "비밀값 대신 Registry token 환경변수 이름을 입력하세요.", n.ID, "", "error")
				}
			} else if strings.TrimSpace(spec.SecretRef) == "" {
				add("REGISTRY_CREDENTIAL_REF", "deployment", path+"/spec", "Docker 인증용 username/tokenEnv 또는 기존 Secret 참조를 입력하세요.", n.ID, "", "error")
			}
		}
	}
	placements := map[string]int{}
	occupied := map[string]string{}
	for i, p := range doc.Placements {
		path := fmt.Sprintf("/placements/%d", i)
		source, sourceOK := nodes[p.WorkloadRef]
		target, targetOK := nodes[p.TargetRef]
		placements[p.WorkloadRef]++
		workload := source.Kind == "java" || source.Kind == "container" || source.Kind == "redis" || source.Kind == "elasticsearch"
		compatible := (target.Kind == "vm" && ((source.Kind == "container" && p.Mode == "container") || (source.Kind != "container" && (p.Mode == "process" || p.Mode == "compose")))) || (target.Kind == "kubernetes" && source.Kind != "container" && p.Mode == "kubernetes")
		if !sourceOK || !targetOK || !workload || !compatible || p.WorkloadRef == p.TargetRef {
			add("INVALID_PLACEMENT", "deployment", path, "실행 대상과 서비스 또는 실행 방식이 올바르지 않습니다.", p.WorkloadRef, "", "error")
			continue
		}
		if p.Mode == "process" || p.Mode == "container" {
			for _, port := range specs[p.WorkloadRef].Ports {
				key := fmt.Sprintf("%s/%s/%d", p.TargetRef, port.Protocol, port.ServicePort)
				if _, exists := occupied[key]; exists {
					add("HOST_PORT_CONFLICT", "deployment", path, "같은 VM/EC2에서 직접 실행하는 서비스 포트가 충돌합니다.", p.WorkloadRef, "", "error")
				}
				occupied[key] = p.WorkloadRef
			}
		}
	}
	for i, n := range doc.Nodes {
		wanted := 0
		if n.Kind == "java" || n.Kind == "container" || ((n.Kind == "redis" || n.Kind == "elasticsearch") && specs[n.ID].Location == "hosted") {
			wanted = 1
		}
		if placements[n.ID] != wanted {
			add("PLACEMENT_COUNT", "model", fmt.Sprintf("/nodes/%d", i), "내부 서비스는 실행 대상 하나, 외부 서비스는 실행 대상 없이 구성하세요.", n.ID, "", "error")
		}
	}
	edgeIDs := map[string]bool{}
	for i, c := range doc.Connections {
		path := fmt.Sprintf("/connections/%d", i)
		source, ok1 := nodes[c.SourceRef]
		target, ok2 := nodes[c.TargetRef]
		sourceApplication := source.Kind == "java" || source.Kind == "container"
		targetApplication := target.Kind == "java" || target.Kind == "container"
		allowed := (c.Kind == "connects_to" && sourceApplication && (targetApplication || target.Kind == "database" || target.Kind == "redis" || target.Kind == "elasticsearch")) ||
			(c.Kind == "routes_to" && source.Kind == "ingress" && targetApplication) ||
			(c.Kind == "publishes_to" && sourceApplication && target.Kind == "registry")
		if !ok1 || !ok2 || !allowed || c.SourceRef == c.TargetRef || edgeIDs[c.ID] {
			add("INVALID_CONNECTION", "model", path, "중복 ID, 잘못된 참조 또는 허용되지 않는 연결입니다.", c.SourceRef, c.ID, "error")
		}
		edgeIDs[c.ID] = true
		found := false
		if c.TargetPortName != nil {
			for _, p := range specs[c.TargetRef].Ports {
				if p.Name == *c.TargetPortName {
					found = true
				}
			}
		}
		if (c.Kind != "publishes_to" && !found) || (c.Kind == "publishes_to" && c.TargetPortName != nil) {
			add("INVALID_PORT_REFERENCE", "model", path+"/targetPortName", "대상 포트 참조가 올바르지 않습니다.", c.TargetRef, c.ID, "error")
		}
	}
	return result
}
