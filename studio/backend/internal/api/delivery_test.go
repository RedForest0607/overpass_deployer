package api

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"studio-backend/internal/domain"
)

func TestArtifactToCrosswalkPackageLifecycle(t *testing.T) {
	handler := testHandler(t, nil)
	projectID := createTestProject(t, handler, "delivery")
	jarID := uploadTestArtifact(t, handler, projectID, "jar", "commerce.jar", jarFixture(t), http.StatusCreated)
	configID := uploadTestArtifact(t, handler, projectID, "config-template", "application.yml", []byte("server:\n  port: 8080\n"), http.StatusCreated)
	listed := httptest.NewRecorder()
	handler.ServeHTTP(listed, httptest.NewRequest("GET", "http://localhost/api/v1/projects/"+projectID+"/artifacts?limit=10", nil))
	if listed.Code != http.StatusOK || !strings.Contains(listed.Body.String(), jarID) || !strings.Contains(listed.Body.String(), configID) {
		t.Fatalf("list artifacts=%d %s", listed.Code, listed.Body.String())
	}
	blueprint := packageBlueprint(jarID, configID)
	save := httptest.NewRecorder()
	handler.ServeHTTP(save, jsonRequest("PUT", "/api/v1/projects/"+projectID+"/topologies/qa", `{"expectedRevision":0,"blueprint":`+blueprint+`,"layout":{"nodes":[],"viewport":{"x":0,"y":0,"zoom":1}}}`))
	if save.Code != 200 {
		t.Fatalf("save=%d %s", save.Code, save.Body.String())
	}
	validated := httptest.NewRecorder()
	handler.ServeHTTP(validated, jsonRequest("POST", "/api/v1/projects/"+projectID+"/validations", `{"environment":"qa","clientRequestId":"package-check","purpose":"package","generator":"crosswalk-vm","blueprint":`+blueprint+`}`))
	if validated.Code != 200 || !strings.Contains(validated.Body.String(), `"valid":true`) {
		t.Fatalf("validate package=%d %s", validated.Code, validated.Body.String())
	}

	body := `{"environment":"qa","revision":1,"generator":"crosswalk-vm","output":"package"}`
	request := jsonRequest("POST", "/api/v1/projects/"+projectID+"/generations", body)
	request.Header.Set("Idempotency-Key", "package-1")
	generated := httptest.NewRecorder()
	handler.ServeHTTP(generated, request)
	if generated.Code != 202 {
		t.Fatalf("generate=%d %s", generated.Code, generated.Body.String())
	}
	var generation domain.Generation
	if err := json.Unmarshal(generated.Body.Bytes(), &generation); err != nil || generation.Status != "queued" {
		t.Fatalf("generation=%+v err=%v", generation, err)
	}
	generation = awaitGeneration(t, handler, projectID, generation.ID)
	if generation.Status != "succeeded" || generation.PackageID == nil {
		t.Fatalf("completed generation=%+v", generation)
	}

	packageURL := "/api/v1/projects/" + projectID + "/packages/" + *generation.PackageID
	metadata := httptest.NewRecorder()
	handler.ServeHTTP(metadata, httptest.NewRequest("GET", "http://localhost"+packageURL, nil))
	if metadata.Code != 200 || !strings.Contains(metadata.Body.String(), `"path":"deploy.yml"`) {
		t.Fatalf("package=%d %s", metadata.Code, metadata.Body.String())
	}
	preview := httptest.NewRecorder()
	handler.ServeHTTP(preview, httptest.NewRequest("GET", "http://localhost"+packageURL+"/files/file_deploy", nil))
	if preview.Code != 200 || !strings.Contains(preview.Body.String(), "config_files:") {
		t.Fatalf("preview=%d %s", preview.Code, preview.Body.String())
	}
	download := httptest.NewRecorder()
	handler.ServeHTTP(download, httptest.NewRequest("GET", "http://localhost"+packageURL+"/download", nil))
	if download.Code != 200 || download.Header().Get("Content-Type") != "application/zip" {
		t.Fatalf("download=%d", download.Code)
	}
	zr, err := zip.NewReader(bytes.NewReader(download.Body.Bytes()), int64(download.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	entries := map[string]bool{}
	for _, file := range zr.File {
		entries[file.Name] = true
	}
	for _, name := range []string{"deploy.yml", "assets/commerce/commerce.jar", "config/commerce/application.yml"} {
		if !entries[name] {
			t.Errorf("missing ZIP entry %s", name)
		}
	}

	replay := httptest.NewRecorder()
	request = jsonRequest("POST", "/api/v1/projects/"+projectID+"/generations", body)
	request.Header.Set("Idempotency-Key", "package-1")
	handler.ServeHTTP(replay, request)
	if replay.Code != 202 || !strings.Contains(replay.Body.String(), generation.ID) {
		t.Fatalf("idempotent replay=%d %s", replay.Code, replay.Body.String())
	}
	conflict := httptest.NewRecorder()
	request = jsonRequest("POST", "/api/v1/projects/"+projectID+"/generations", `{"environment":"qa","revision":1,"generator":"crosswalk-vm","output":"draft"}`)
	request.Header.Set("Idempotency-Key", "package-1")
	handler.ServeHTTP(conflict, request)
	if conflict.Code != http.StatusConflict || !strings.Contains(conflict.Body.String(), "IDEMPOTENCY_CONFLICT") {
		t.Fatalf("idempotency conflict=%d %s", conflict.Code, conflict.Body.String())
	}
}

func TestArtifactValidationAndProjectScope(t *testing.T) {
	handler := testHandler(t, nil)
	first := createTestProject(t, handler, "first")
	second := createTestProject(t, handler, "second")
	uploadTestArtifact(t, handler, first, "jar", "bad.jar", []byte("not zip"), 422)
	uploadTestArtifact(t, handler, first, "archive", "bad.tar", []byte("not tar"), 422)
	uploadTestArtifact(t, handler, first, "archive", "unsafe.tar", unsafeTarFixture(t), 422)
	jarID := uploadTestArtifact(t, handler, first, "jar", "ok.jar", jarFixture(t), 201)
	blueprint := packageBlueprint(jarID, "")
	save := httptest.NewRecorder()
	handler.ServeHTTP(save, jsonRequest("PUT", "/api/v1/projects/"+second+"/topologies/dev", `{"expectedRevision":0,"blueprint":`+blueprint+`,"layout":{"nodes":[],"viewport":{"x":0,"y":0,"zoom":1}}}`))
	if save.Code != 200 {
		t.Fatal(save.Body.String())
	}
	request := jsonRequest("POST", "/api/v1/projects/"+second+"/generations", `{"environment":"dev","revision":1,"generator":"crosswalk-vm","output":"package"}`)
	request.Header.Set("Idempotency-Key", "foreign")
	result := httptest.NewRecorder()
	handler.ServeHTTP(result, request)
	if result.Code != 202 {
		t.Fatalf("foreign enqueue=%d %s", result.Code, result.Body.String())
	}
	var generation domain.Generation
	if err := json.Unmarshal(result.Body.Bytes(), &generation); err != nil {
		t.Fatal(err)
	}
	generation = awaitGeneration(t, handler, second, generation.ID)
	if generation.Status != "failed" || generation.Error == nil || !strings.Contains(generation.Error.Message, "artifact") {
		t.Fatalf("foreign generation=%+v", generation)
	}
}

func TestArtifactTextEditingCreatesImmutableVersion(t *testing.T) {
	handler := testHandler(t, nil)
	projectID := createTestProject(t, handler, "editable")
	otherProjectID := createTestProject(t, handler, "other")
	originalID := uploadTestArtifact(t, handler, projectID, "config-template", "application.yml", []byte("server:\n  port: 8080\n"), http.StatusCreated)
	updatedID := uploadTestArtifact(t, handler, projectID, "config-template", "application.yml", []byte("server:\n  port: 9090\n"), http.StatusCreated)
	if originalID == updatedID {
		t.Fatal("edited content must create a new artifact ID")
	}

	readContent := func(project, artifact string) string {
		t.Helper()
		result := httptest.NewRecorder()
		handler.ServeHTTP(result, httptest.NewRequest(http.MethodGet, "http://localhost/api/v1/projects/"+project+"/artifacts/"+artifact+"/content", nil))
		if result.Code != http.StatusOK {
			t.Fatalf("read artifact content=%d %s", result.Code, result.Body.String())
		}
		var response struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal(result.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		return response.Content
	}
	if got := readContent(projectID, originalID); got != "server:\n  port: 8080\n" {
		t.Fatalf("original artifact changed: %q", got)
	}
	if got := readContent(projectID, updatedID); got != "server:\n  port: 9090\n" {
		t.Fatalf("updated artifact=%q", got)
	}

	foreign := httptest.NewRecorder()
	handler.ServeHTTP(foreign, httptest.NewRequest(http.MethodGet, "http://localhost/api/v1/projects/"+otherProjectID+"/artifacts/"+originalID+"/content", nil))
	if foreign.Code != http.StatusNotFound {
		t.Fatalf("foreign project read=%d %s", foreign.Code, foreign.Body.String())
	}

	plainID := uploadTestArtifact(t, handler, projectID, "file", "notes.txt", []byte("plain text"), http.StatusCreated)
	plain := httptest.NewRecorder()
	handler.ServeHTTP(plain, httptest.NewRequest(http.MethodGet, "http://localhost/api/v1/projects/"+projectID+"/artifacts/"+plainID+"/content", nil))
	if plain.Code != http.StatusBadRequest || !strings.Contains(plain.Body.String(), "ARTIFACT_NOT_READABLE") {
		t.Fatalf("non-editable artifact=%d %s", plain.Code, plain.Body.String())
	}
}

func TestEditableArtifactRejectsInvalidYAMLAndXML(t *testing.T) {
	handler := testHandler(t, nil)
	projectID := createTestProject(t, handler, "syntax")
	uploadTestArtifact(t, handler, projectID, "config-template", "broken.yml", []byte("server: [\n"), http.StatusUnprocessableEntity)
	uploadTestArtifact(t, handler, projectID, "file", "broken.xml", []byte("<root><child></root>"), http.StatusUnprocessableEntity)
	uploadTestArtifact(t, handler, projectID, "file", "two-roots.xml", []byte("<first/><second/>"), http.StatusUnprocessableEntity)
}

func awaitGeneration(t *testing.T, handler http.Handler, projectID, generationID string) domain.Generation {
	t.Helper()
	for attempt := 0; attempt < 200; attempt++ {
		result := httptest.NewRecorder()
		handler.ServeHTTP(result, httptest.NewRequest("GET", "http://localhost/api/v1/projects/"+projectID+"/generations/"+generationID, nil))
		if result.Code != 200 {
			t.Fatalf("get generation=%d %s", result.Code, result.Body.String())
		}
		var generation domain.Generation
		if err := json.Unmarshal(result.Body.Bytes(), &generation); err != nil {
			t.Fatal(err)
		}
		if generation.Status == "succeeded" || generation.Status == "failed" {
			return generation
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("generation did not finish")
	return domain.Generation{}
}

func createTestProject(t *testing.T, handler http.Handler, name string) string {
	t.Helper()
	result := httptest.NewRecorder()
	handler.ServeHTTP(result, jsonRequest("POST", "/api/v1/projects", `{"name":"`+name+`","description":""}`))
	if result.Code != 201 {
		t.Fatal(result.Body.String())
	}
	return strings.TrimPrefix(result.Header().Get("Location"), "/api/v1/projects/")
}
func uploadTestArtifact(t *testing.T, handler http.Handler, projectID, kind, name string, body []byte, want int) string {
	t.Helper()
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)
	_ = writer.WriteField("kind", kind)
	part, err := writer.CreateFormFile("file", name)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write(body)
	_ = writer.Close()
	request := httptest.NewRequest("POST", "http://localhost/api/v1/projects/"+projectID+"/artifacts", &requestBody)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	result := httptest.NewRecorder()
	handler.ServeHTTP(result, request)
	if result.Code != want {
		t.Fatalf("upload=%d %s", result.Code, result.Body.String())
	}
	if want != 201 {
		return ""
	}
	var artifact domain.Artifact
	if json.Unmarshal(result.Body.Bytes(), &artifact) != nil {
		t.Fatal("invalid artifact response")
	}
	return artifact.ID
}
func jarFixture(t *testing.T) []byte {
	t.Helper()
	var body bytes.Buffer
	writer := zip.NewWriter(&body)
	entry, err := writer.Create("META-INF/MANIFEST.MF")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.WriteString(entry, "Manifest-Version: 1.0\n")
	_ = writer.Close()
	return body.Bytes()
}

func unsafeTarFixture(t *testing.T) []byte {
	t.Helper()
	var body bytes.Buffer
	writer := tar.NewWriter(&body)
	content := []byte("unsafe")
	if err := writer.WriteHeader(&tar.Header{Name: "../outside", Mode: 0o600, Size: int64(len(content))}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return body.Bytes()
}
func packageBlueprint(jarID, configID string) string {
	config := ""
	if configID != "" {
		config = `,"configFiles":[{"artifactRef":"` + configID + `","remotePath":"/app/overpass/commerce/config/application.yml"}]`
	}
	return `{"apiVersion":"studio.deployer/v1","deployment":{"ssh":{"user":"deploy","keyPath":"${DEPLOY_SSH_KEY_PATH}","knownHostsPath":"${DEPLOY_KNOWN_HOSTS}","hostKeyChecking":"strict"}},"nodes":[{"id":"vm","kind":"vm","name":"qa-was","spec":{"provider":"on-prem","host":"10.0.0.10","deployment":{"bootstrap":{"packages":["curl"]}}}},{"id":"app","kind":"java","name":"commerce","spec":{"artifactRef":"` + jarID + `","javaVersion":"21","maxHeap":"1g","ports":[{"name":"main","servicePort":8080,"protocol":"TCP"}],"deployment":{"minHeap":"256m"` + config + `}}}],"placements":[{"workloadRef":"app","targetRef":"vm","mode":"process"}],"connections":[]}`
}
