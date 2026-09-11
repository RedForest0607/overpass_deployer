package crosswalkvm

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type memoryResolver map[string][]byte

func (r memoryResolver) Resolve(_ context.Context, id string) (Artifact, error) {
	body, ok := r[id]
	if !ok {
		return Artifact{}, fmt.Errorf("missing")
	}
	sum := sha256.Sum256(body)
	name := map[string]string{"jar": "commerce.jar", "config": "application.yml", "app-file": "agent.zip", "server-file": "limits.conf", "utility": "redis.tar.gz", "script": "custom.sh.tmpl", "values": "script-values.yml"}[id]
	return Artifact{ID: id, OriginalName: name, Size: int64(len(body)), SHA256: hex.EncodeToString(sum[:]), Open: func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(body)), nil }}, nil
}

func TestGenerateCoversCrosswalkDeliveryFeatures(t *testing.T) {
	resolver := memoryResolver{"jar": []byte("jar"), "config": []byte("server.port: 8080"), "app-file": []byte("archive"), "server-file": []byte("limits"), "utility": []byte("redis"), "script": []byte("{{.AppName}}"), "values": []byte("ActiveProfile: qa")}
	raw := json.RawMessage(`{
      "apiVersion":"studio.deployer/v1",
      "deployment":{
        "ssh":{"user":"deploy","keyPath":"${DEPLOY_SSH_KEY_PATH}","knownHostsPath":"${DEPLOY_KNOWN_HOSTS}","hostKeyChecking":"strict","port":22},
        "bastion":{"host":"bastion.example","aliasUser":"ops"},
        "directoryDefaults":[{"prefix":"/app/overpass","owner":"deploy","group":"deploy"}]
      },
      "nodes":[
        {"id":"vm","kind":"vm","name":"qa-was-01","spec":{"provider":"on-prem","host":"10.0.0.10","deployment":{
          "tags":["qa","was"],"sshPort":2222,"bastionHost":"10.0.0.10","bastionSSHPort":22,
          "bootstrap":{"packages":["curl","unzip"],"jdk":{"enabled":true,"vendor":"corretto","major":21},"osUpdate":{"enabled":true},"timezone":"Asia/Seoul","swap":{"enabled":true,"size":"2G"},"services":[{"name":"nginx","enabled":true,"state":"started"}]},
          "directories":["/app/shared"],
          "files":[{"artifactRef":"server-file","remotePath":"/etc/security/limits.d/app.conf","chmod":"0644"}],
          "utilities":[{"name":"redis","directories":["/app/redis"],"files":[{"artifactRef":"utility","remotePath":"/tmp/redis.tar.gz","extract":{"enabled":true,"remoteDir":"/app/redis","stripComponents":1,"sudo":true,"owner":"redis","group":"redis","chownRecursive":true}}],"services":[{"name":"redis","state":"restarted"}]}]
        }}},
        {"id":"app","kind":"java","name":"commerce","spec":{"artifactRef":"jar","javaVersion":"21","maxHeap":"2g","ports":[{"name":"main","servicePort":8080,"protocol":"TCP"}],"deployment":{
          "baseDir":"/app/overpass/commerce","jarRemotePath":"/app/overpass/commerce/lib/commerce.jar","minHeap":"512m",
          "javaOptions":["-Duser.timezone=Asia/Seoul"],"extraOptions":["--spring.profiles.active=qa"],
          "configFiles":[{"artifactRef":"config","remotePath":"/app/overpass/commerce/config/application.yml"}],
          "extraFiles":[{"artifactRef":"app-file","packagePath":"files/app/commerce/agent.zip","remotePath":"/tmp/agent.zip","chmod":"0755","extract":{"enabled":true,"remoteDir":"/opt/agent"}}],
          "script":{"mode":"template","artifactRef":"script","valuesArtifactRef":"values","remotePath":"/app/overpass/commerce/bin/server.sh"}
        }}}
      ],
      "placements":[{"workloadRef":"app","targetRef":"vm","mode":"process"}],"connections":[]
    }`)
	bundle, err := Generate(context.Background(), raw, "qa", resolver)
	if err != nil {
		t.Fatal(err)
	}
	var rendered map[string]any
	if err := yaml.Unmarshal(bundle.DeployYAML, &rendered); err != nil {
		t.Fatal(err)
	}
	yamlText := string(bundle.DeployYAML)
	for _, expected := range []string{"bootstrap:", "utils:", "config_files:", "extra_files:", "chmod: \"0755\"", "extract:", "java_opts:", "extra_opts:", "values_file:", "bastion:", "./assets/commerce/commerce.jar"} {
		if !strings.Contains(yamlText, expected) {
			t.Errorf("deploy.yml does not contain %q:\n%s", expected, yamlText)
		}
	}

	var zipped bytes.Buffer
	if err := WriteZIP(&zipped, bundle); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(zipped.Bytes()), int64(zipped.Len()))
	if err != nil {
		t.Fatal(err)
	}
	entries := map[string]bool{}
	for _, file := range zr.File {
		entries[file.Name] = true
	}
	for _, expected := range []string{"deploy.yml", "assets/commerce/commerce.jar", "config/commerce/application.yml", "files/app/commerce/agent.zip", "files/server/qa-was-01/limits.conf", "files/utility/qa-was-01/redis/redis.tar.gz", "scripts/commerce/custom.sh.tmpl", "values/commerce/script-values.yml"} {
		if !entries[expected] {
			t.Errorf("ZIP entry %q is missing", expected)
		}
	}
}

func TestGenerateRejectsUnsafeOrCollidingPaths(t *testing.T) {
	resolver := memoryResolver{"jar": []byte("jar"), "config": []byte("a"), "app-file": []byte("b")}
	base := `{"apiVersion":"studio.deployer/v1","deployment":{"ssh":{"user":"deploy","keyPath":"key","knownHostsPath":"hosts","hostKeyChecking":"strict"}},"nodes":[{"id":"vm","kind":"vm","name":"vm","spec":{"provider":"on-prem","host":"host"}},{"id":"app","kind":"java","name":"app","spec":{"artifactRef":"jar","javaVersion":"21","maxHeap":"1g","ports":[{"name":"main","servicePort":8080,"protocol":"TCP"}],"deployment":{"extraFiles":[%s]}}}],"placements":[{"workloadRef":"app","targetRef":"vm","mode":"process"}]}`
	for _, tc := range []struct{ name, files, want string }{
		{"traversal", `{"artifactRef":"config","packagePath":"../secret","remotePath":"/tmp/a"}`, "unsafe package path"},
		{"collision", `{"artifactRef":"config","packagePath":"files/same","remotePath":"/tmp/a"},{"artifactRef":"app-file","packagePath":"files/same","remotePath":"/tmp/b"}`, "multiple artifacts"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Generate(context.Background(), json.RawMessage(fmt.Sprintf(base, tc.files)), "dev", resolver)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestWriteZIPRechecksPathsAndArtifactIntegrity(t *testing.T) {
	artifact, err := (memoryResolver{"jar": []byte("jar")}).Resolve(context.Background(), "jar")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		bundle Bundle
		want   string
	}{
		{"unsafe path", Bundle{Files: []PackageFile{{Path: "../jar", Artifact: artifact}}}, "unsafe package path"},
		{"changed body", Bundle{Files: []PackageFile{{Path: "assets/app.jar", Artifact: Artifact{ID: artifact.ID, Size: artifact.Size + 1, SHA256: artifact.SHA256, Open: artifact.Open}}}}, "changed while packaging"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			err := WriteZIP(&output, tc.bundle)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}
