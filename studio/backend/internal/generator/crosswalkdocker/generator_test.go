package crosswalkdocker

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const validDigest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func TestGenerateDockerChart(t *testing.T) {
	raw := json.RawMessage(`{
		"apiVersion":"studio.deployer/v1",
		"deployment":{"ssh":{"user":"deploy","keyPath":"${DEPLOY_KEY}","knownHostsPath":"${KNOWN_HOSTS}","hostKeyChecking":"strict"}},
		"nodes":[
			{"id":"registry","kind":"registry","name":"harbor","spec":{"endpoint":"harbor.example.com:8443","username":"robot$studio","tokenEnv":"HARBOR_TOKEN"}},
			{"id":"vm","kind":"vm","name":"app-prod-a","spec":{"provider":"on-prem","host":"10.0.1.10","deployment":{"tags":["prod"],"sshPort":2222}}},
			{"id":"api","kind":"container","name":"sample-api","spec":{"image":"overpass/sample-api","digest":"` + validDigest + `","ports":[{"name":"http","servicePort":8080,"containerPort":8081,"protocol":"TCP"}],"deployment":{"tags":["api"],"environment":{"SPRING_PROFILES_ACTIVE":"prod","DB_PASSWORD":"${DB_PASSWORD}"},"restartPolicy":"unless-stopped","healthTimeoutSeconds":90}}}
		],
		"placements":[{"workloadRef":"api","targetRef":"vm","mode":"container"}],
		"connections":[]
	}`)

	bundle, err := Generate(raw, "prod")
	if err != nil {
		t.Fatalf("Generate() returned error: %v", err)
	}
	var output struct {
		Format string `yaml:"format"`
		Docker struct {
			Registry chartRegistry `yaml:"registry"`
		} `yaml:"docker"`
		Servers []chartServer `yaml:"servers"`
	}
	if err := yaml.Unmarshal(bundle.DeployYAML, &output); err != nil {
		t.Fatalf("decode generated YAML: %v", err)
	}
	if output.Format != ChartFormat || output.Docker.Registry.TokenEnv != "HARBOR_TOKEN" || len(output.Servers) != 1 || len(output.Servers[0].Containers) != 1 {
		t.Fatalf("unexpected generated chart: %#v", output)
	}
	container := output.Servers[0].Containers[0]
	if container.Image != "overpass/sample-api" || container.Ports[0].Host != 8080 || container.Ports[0].Container != 8081 || container.Environment["DB_PASSWORD"] != "${DB_PASSWORD}" {
		t.Fatalf("unexpected generated container: %#v", container)
	}

	var archive bytes.Buffer
	if err := WriteZIP(&archive, bundle); err != nil {
		t.Fatalf("WriteZIP() returned error: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(archive.Bytes()), int64(archive.Len()))
	if err != nil || len(zr.File) != 1 || zr.File[0].Name != "deploy.yml" {
		t.Fatalf("unexpected Docker package ZIP: files=%v err=%v", zr.File, err)
	}
	reader, err := zr.File[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	zipped, _ := io.ReadAll(reader)
	_ = reader.Close()
	if !bytes.Equal(zipped, bundle.DeployYAML) {
		t.Fatal("ZIP deploy.yml differs from generated YAML")
	}
}

func TestGenerateRejectsMissingRegistryAndInvalidImage(t *testing.T) {
	base := `{"apiVersion":"studio.deployer/v1","deployment":{"ssh":{"user":"deploy","keyPath":"key"}},"nodes":[%s],"placements":%s}`
	vm := `{"id":"vm","kind":"vm","name":"vm","spec":{"host":"host"}}`
	container := `{"id":"api","kind":"container","name":"api","spec":{"image":"repo/api:latest","digest":"` + validDigest + `","ports":[]}}`
	placement := `[{"workloadRef":"api","targetRef":"vm","mode":"container"}]`

	if _, err := Generate(json.RawMessage(fmt.Sprintf(base, vm+","+container, placement)), "dev"); err == nil || !strings.Contains(err.Error(), "registry") {
		t.Fatalf("expected missing registry error, got %v", err)
	}

	registry := `{"id":"registry","kind":"registry","name":"registry","spec":{"endpoint":"harbor.example.com","username":"robot","tokenEnv":"HARBOR_TOKEN"}}`
	raw := `{"apiVersion":"studio.deployer/v1","deployment":{"ssh":{"user":"deploy","keyPath":"key"}},"nodes":[` + vm + `,` + container + `,` + registry + `],"placements":` + placement + `}`
	if _, err := Generate(json.RawMessage(raw), "dev"); err == nil || !strings.Contains(err.Error(), "image repository or digest") {
		t.Fatalf("expected invalid image error, got %v", err)
	}
}

func TestGenerateRejectsRegistryValuesCrosswalkCannotLoad(t *testing.T) {
	base := `{"apiVersion":"studio.deployer/v1","deployment":{"ssh":{"user":"deploy","keyPath":"key"}},"nodes":[{"id":"vm","kind":"vm","name":"vm","spec":{"host":"host"}},{"id":"api","kind":"container","name":"api","spec":{"image":"repo/api","digest":"` + validDigest + `","ports":[]}},{"id":"registry","kind":"registry","name":"registry","spec":{"endpoint":%q,"username":%q,"tokenEnv":"HARBOR_TOKEN"}}],"placements":[{"workloadRef":"api","targetRef":"vm","mode":"container"}],"connections":[]}`
	for _, test := range []struct {
		name, endpoint, username string
	}{
		{name: "URL scheme", endpoint: "https://harbor.example.com", username: "robot"},
		{name: "invalid port", endpoint: "harbor.example.com:70000", username: "robot"},
		{name: "unsupported username", endpoint: "harbor.example.com", username: "robot secret"},
	} {
		t.Run(test.name, func(t *testing.T) {
			raw := json.RawMessage(fmt.Sprintf(base, test.endpoint, test.username))
			if _, err := Generate(raw, "dev"); err == nil || !strings.Contains(err.Error(), "registry") {
				t.Fatalf("expected registry validation error, got %v", err)
			}
		})
	}
}
