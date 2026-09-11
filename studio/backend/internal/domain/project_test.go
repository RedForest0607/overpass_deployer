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

func TestValidateTopologyStructureAcceptsCrosswalkDeploymentModel(t *testing.T) {
	blueprint := json.RawMessage(`{"apiVersion":"studio.deployer/v1","deployment":{"ssh":{"user":"deploy","keyPath":"${KEY}","knownHostsPath":"${HOSTS}","hostKeyChecking":"strict"}},"nodes":[{"id":"vm","kind":"vm","name":"vm","spec":{"provider":"on-prem","host":"10.0.0.1","deployment":{"bootstrap":{"packages":["curl"]},"files":[{"artifactRef":"file-1","remotePath":"/etc/app.conf","chmod":"0644"}],"utilities":[{"name":"redis","files":[{"artifactRef":"archive-1","remotePath":"/tmp/redis.tgz","extract":{"enabled":true,"remoteDir":"/app/redis"}}]}]}}},{"id":"app","kind":"java","name":"app","spec":{"artifactRef":"jar-1","javaVersion":"21","maxHeap":"1g","ports":[{"name":"main","servicePort":8080,"protocol":"TCP"}],"deployment":{"javaOptions":["-Duser.timezone=Asia/Seoul"],"configFiles":[{"artifactRef":"config-1","remotePath":"/app/app.yml"}],"script":{"mode":"template","template":"embedded:server.sh.tmpl","valuesArtifactRef":"values-1","remotePath":"/app/server.sh"}}}}],"placements":[{"workloadRef":"app","targetRef":"vm","mode":"process"}],"connections":[]}`)
	layout := json.RawMessage(`{"nodes":[],"viewport":{"x":0,"y":0,"zoom":1}}`)
	if err := ValidateTopologyStructure(blueprint, layout, 10, 10); err != nil {
		t.Fatal(err)
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
