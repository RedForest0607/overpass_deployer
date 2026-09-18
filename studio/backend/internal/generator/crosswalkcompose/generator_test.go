package crosswalkcompose

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestGenerateComposePackageFromContainerBlueprint(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	raw := json.RawMessage(`{"apiVersion":"studio.deployer/v1","deployment":{"ssh":{"user":"deploy","keyPath":"${DEPLOY_KEY}","knownHostsPath":"","hostKeyChecking":"insecure"}},"nodes":[{"id":"registry","kind":"registry","name":"harbor","spec":{"endpoint":"harbor.example.com","username":"robot-studio","tokenEnv":"HARBOR_TOKEN"}},{"id":"vm","kind":"vm","name":"compose-dev-01","spec":{"provider":"on-prem","host":"192.0.2.20"}},{"id":"api","kind":"container","name":"commerce-api","spec":{"image":"demo/commerce-api","digest":"` + digest + `","ports":[{"name":"main","servicePort":18080,"containerPort":8080,"protocol":"TCP"}],"deployment":{"environment":{"SPRING_PROFILES_ACTIVE":"dev"},"restartPolicy":"unless-stopped"}}}],"placements":[{"workloadRef":"api","targetRef":"vm","mode":"container"}],"connections":[]}`)

	bundle, err := Generate(raw, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if len(bundle.Files) != 1 || bundle.Files[0].Path != "compose/compose-dev-01/compose.yml" {
		t.Fatalf("files=%#v", bundle.Files)
	}
	var compose composeDocument
	if err := yaml.Unmarshal(bundle.Files[0].Body, &compose); err != nil {
		t.Fatal(err)
	}
	service := compose.Services["commerce-api"]
	if service.Image != "harbor.example.com/demo/commerce-api@"+digest || service.Ports[0] != "18080:8080/tcp" || service.Environment["SPRING_PROFILES_ACTIVE"] != "dev" {
		t.Fatalf("service=%#v", service)
	}
	if !bytes.Contains(bundle.DeployYAML, []byte("compose_projects:")) || !bytes.Contains(bundle.DeployYAML, []byte("token_env: HARBOR_TOKEN")) {
		t.Fatalf("deploy.yml=%s", bundle.DeployYAML)
	}

	var output bytes.Buffer
	if err := WriteZIP(&output, bundle); err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(bytes.NewReader(output.Bytes()), int64(output.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if len(archive.File) != 2 || archive.File[0].Name != "deploy.yml" || archive.File[1].Name != bundle.Files[0].Path {
		t.Fatalf("archive=%#v", archive.File)
	}
}

func TestGenerateComposeRejectsUnsafeProjectNames(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	raw := json.RawMessage(`{"apiVersion":"studio.deployer/v1","deployment":{"ssh":{"user":"deploy","keyPath":"key"}},"nodes":[{"id":"registry","kind":"registry","name":"harbor","spec":{"endpoint":"harbor.example.com","username":"robot","tokenEnv":"HARBOR_TOKEN"}},{"id":"vm","kind":"vm","name":"Bad.Project","spec":{"host":"host"}},{"id":"api","kind":"container","name":"api","spec":{"image":"demo/api","digest":"` + digest + `","ports":[]}}],"placements":[{"workloadRef":"api","targetRef":"vm","mode":"container"}],"connections":[]}`)
	if _, err := Generate(raw, "dev"); err == nil || !strings.Contains(err.Error(), "Compose project name") {
		t.Fatalf("expected project name error, got %v", err)
	}
}
