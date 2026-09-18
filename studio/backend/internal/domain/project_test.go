package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateTopologyStructureAllowsIncompleteDesign(t *testing.T) {
	blueprint := json.RawMessage(`{"apiVersion":"studio.deployer/v1","nodes":[{"id":"app","kind":"java","name":"","spec":{"artifactRef":null,"javaVersion":"","maxHeap":"","ports":[]}}],"placements":[],"connections":[]}`)
	layout := json.RawMessage(`{"nodes":[{"nodeId":"app","position":{"x":0,"y":0}}],"viewport":{"x":0,"y":0,"zoom":1}}`)
	if err := ValidateTopologyStructure(blueprint, layout, 10, 10); err != nil {
		t.Fatal(err)
	}
}

func TestValidateTopologyStructureAcceptsAndScopesConnectionLayout(t *testing.T) {
	blueprint := json.RawMessage(`{"apiVersion":"studio.deployer/v1","nodes":[],"placements":[],"connections":[{"id":"conn-1","kind":"connects_to","sourceRef":"app","targetRef":"db"}]}`)
	valid := json.RawMessage(`{"nodes":[],"connections":[{"connectionId":"conn-1","bendOffset":{"x":40,"y":-20},"labelPosition":0.72}],"viewport":{"x":0,"y":0,"zoom":1}}`)
	if err := ValidateTopologyStructure(blueprint, valid, 10, 10); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{
		`{"nodes":[],"connections":[{"connectionId":"missing","bendOffset":{"x":0,"y":0}}],"viewport":{"x":0,"y":0,"zoom":1}}`,
		`{"nodes":[],"connections":[{"connectionId":"conn-1"}],"viewport":{"x":0,"y":0,"zoom":1}}`,
		`{"nodes":[],"connections":[{"connectionId":"conn-1","bendOffset":{"x":1000001,"y":0}}],"viewport":{"x":0,"y":0,"zoom":1}}`,
		`{"nodes":[],"connections":[{"connectionId":"conn-1","labelPosition":1.1}],"viewport":{"x":0,"y":0,"zoom":1}}`,
	} {
		if err := ValidateTopologyStructure(blueprint, json.RawMessage(invalid), 10, 10); err == nil {
			t.Fatalf("expected invalid connection layout: %s", invalid)
		}
	}
}

func TestValidateTopologyStructureAcceptsCrosswalkDeploymentModel(t *testing.T) {
	blueprint := json.RawMessage(`{"apiVersion":"studio.deployer/v1","deployment":{"ssh":{"user":"deploy","keyPath":"${KEY}","knownHostsPath":"${HOSTS}","hostKeyChecking":"strict"},"bootstrap":{"packages":["unzip"]}},"nodes":[{"id":"vm","kind":"vm","name":"vm","spec":{"provider":"on-prem","host":"10.0.0.1","deployment":{"bootstrap":{"packages":["curl"]},"files":[{"artifactRef":"file-1","remotePath":"/etc/app.conf","chmod":"0644"}],"utilities":[{"name":"redis","files":[{"artifactRef":"archive-1","remotePath":"/tmp/redis.tgz","extract":{"enabled":true,"remoteDir":"/app/redis"}}]}]}}},{"id":"app","kind":"java","name":"app","spec":{"artifactRef":"jar-1","javaVersion":"21","maxHeap":"1g","ports":[{"name":"main","servicePort":8080,"protocol":"TCP"}],"deployment":{"tags":["api"],"javaOptions":["-Duser.timezone=Asia/Seoul"],"configFiles":[{"artifactRef":"config-1","remotePath":"/app/app.yml"}],"script":{"mode":"local-file","artifactRef":"script-1","remotePath":"/app/server.sh"}}}},{"id":"search","kind":"elasticsearch","name":"search","spec":{"location":"hosted","ports":[],"deployment":{"directories":["/app/elasticsearch"],"services":[{"name":"elasticsearch","enabled":true,"state":"restarted"}]}}}],"placements":[{"workloadRef":"app","targetRef":"vm","mode":"process"},{"workloadRef":"search","targetRef":"vm","mode":"process"}],"connections":[]}`)
	layout := json.RawMessage(`{"nodes":[],"viewport":{"x":0,"y":0,"zoom":1}}`)
	if err := ValidateTopologyStructure(blueprint, layout, 10, 10); err != nil {
		t.Fatal(err)
	}
}

func TestValidateTopologyStructureAcceptsDockerDeploymentModel(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	blueprint := json.RawMessage(`{"apiVersion":"studio.deployer/v1","deployment":{"ssh":{"user":"deploy","keyPath":"${KEY}","knownHostsPath":"${HOSTS}","hostKeyChecking":"strict"}},"nodes":[{"id":"registry","kind":"registry","name":"harbor","spec":{"endpoint":"harbor.example.com","username":"robot-studio","tokenEnv":"HARBOR_TOKEN"}},{"id":"vm","kind":"vm","name":"docker-host","spec":{"provider":"on-prem","host":"10.0.0.10"}},{"id":"api","kind":"container","name":"sample-api","spec":{"image":"overpass/sample-api","digest":"` + digest + `","ports":[{"name":"main","servicePort":8080,"containerPort":8080,"protocol":"TCP"}],"deployment":{"environment":{"SPRING_PROFILES_ACTIVE":"dev"},"restartPolicy":"unless-stopped","healthTimeoutSeconds":60}}}],"placements":[{"workloadRef":"api","targetRef":"vm","mode":"container"}],"connections":[]}`)
	layout := json.RawMessage(`{"nodes":[{"nodeId":"registry","position":{"x":0,"y":0}},{"nodeId":"vm","position":{"x":200,"y":0}},{"nodeId":"api","position":{"x":50,"y":100}}],"viewport":{"x":0,"y":0,"zoom":1}}`)
	if err := ValidateTopologyStructure(blueprint, layout, 10, 10); err != nil {
		t.Fatalf("Docker deployment model should be accepted: %v", err)
	}
	if result := ValidateDesign(blueprint, "docker"); !result.Valid {
		t.Fatalf("Docker design should be valid: %#v", result.Issues)
	}
}

func TestValidateTopologyStructureRejectsConfigOnlyDeliveryOptions(t *testing.T) {
	blueprint := json.RawMessage(`{"apiVersion":"studio.deployer/v1","nodes":[{"id":"app","kind":"java","name":"app","spec":{"artifactRef":"jar-1","javaVersion":"21","maxHeap":"1g","ports":[],"deployment":{"configFiles":[{"artifactRef":"config-1","remotePath":"/app/application.yml","chmod":"0600"}]}}}],"placements":[],"connections":[]}`)
	layout := json.RawMessage(`{"nodes":[],"viewport":{"x":0,"y":0,"zoom":1}}`)
	if err := ValidateTopologyStructure(blueprint, layout, 10, 10); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("error=%v", err)
	}
}

func TestValidateTopologyStructureRejectsUnsafePackagePath(t *testing.T) {
	blueprint := json.RawMessage(`{"apiVersion":"studio.deployer/v1","nodes":[{"id":"app","kind":"java","name":"app","spec":{"artifactRef":"jar-1","javaVersion":"21","maxHeap":"1g","ports":[],"deployment":{"extraFiles":[{"artifactRef":"file-1","packagePath":"../secret","remotePath":"/tmp/file"}]}}}],"placements":[],"connections":[]}`)
	layout := json.RawMessage(`{"nodes":[],"viewport":{"x":0,"y":0,"zoom":1}}`)
	if err := ValidateTopologyStructure(blueprint, layout, 10, 10); err == nil || !strings.Contains(err.Error(), "ZIP-relative") {
		t.Fatalf("error=%v", err)
	}
}

func TestValidateTopologyStructureRejectsMissingKindSpecificFields(t *testing.T) {
	blueprint := json.RawMessage(`{"apiVersion":"studio.deployer/v1","nodes":[{"id":"app","kind":"java","name":"app","spec":{}}],"placements":[],"connections":[]}`)
	layout := json.RawMessage(`{"nodes":[],"viewport":{"x":0,"y":0,"zoom":1}}`)
	if err := ValidateTopologyStructure(blueprint, layout, 10, 10); err == nil {
		t.Fatal("expected missing Java spec fields to be rejected")
	}
}

func TestValidateTopologyStructureRejectsUnknownFieldsAndMissingArrays(t *testing.T) {
	for _, blueprint := range []string{
		`{"apiVersion":"studio.deployer/v1","nodes":[],"placements":[],"connections":[],"selected":true}`,
		`{"apiVersion":"studio.deployer/v1","nodes":[]}`,
	} {
		err := ValidateTopologyStructure(json.RawMessage(blueprint), json.RawMessage(`{"nodes":[],"viewport":{"x":0,"y":0,"zoom":1}}`), 10, 10)
		if err == nil {
			t.Fatalf("expected invalid blueprint: %s", blueprint)
		}
	}
}
