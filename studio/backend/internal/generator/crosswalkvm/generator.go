package crosswalkvm

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"studio-backend/internal/domain"
)

const ID = "crosswalk-vm"
const Version = "1.0.0"

type Artifact struct {
	ID           string
	OriginalName string
	Kind         string
	Size         int64
	SHA256       string
	Open         func() (io.ReadCloser, error)
}

type Resolver interface {
	Resolve(context.Context, string) (Artifact, error)
}

type PackageFile struct {
	Path     string
	Artifact Artifact
}
type Bundle struct {
	DeployYAML []byte
	Files      []PackageFile
}

type blueprint struct {
	APIVersion string                    `json:"apiVersion"`
	Deployment *domain.DeploymentProfile `json:"deployment,omitempty"`
	Nodes      []node                    `json:"nodes"`
	Placements []placement               `json:"placements"`
}
type node struct {
	ID, Kind, Name string
	Spec           json.RawMessage `json:"spec"`
}
type placement struct{ WorkloadRef, TargetRef, Mode string }
type vmSpec struct {
	Provider, Host string
	Deployment     *domain.VMDeployment `json:"deployment,omitempty"`
}
type javaSpec struct {
	ArtifactRef string `json:"artifactRef"`
	JavaVersion string `json:"javaVersion"`
	MaxHeap     string `json:"maxHeap"`
	Ports       []struct {
		ServicePort int `json:"servicePort"`
	} `json:"ports"`
	Deployment *domain.JavaDeployment `json:"deployment,omitempty"`
}

type builder struct {
	ctx      context.Context
	resolver Resolver
	files    map[string]Artifact
}

func Generate(ctx context.Context, raw json.RawMessage, environment string, resolver Resolver) (Bundle, error) {
	if resolver == nil {
		return Bundle{}, errors.New("artifact resolver is required")
	}
	var doc blueprint
	if err := json.Unmarshal(raw, &doc); err != nil {
		return Bundle{}, fmt.Errorf("decode blueprint: %w", err)
	}
	if doc.APIVersion != "studio.deployer/v1" {
		return Bundle{}, errors.New("unsupported blueprint version")
	}
	if doc.Deployment == nil {
		return Bundle{}, errors.New("deployment profile is required")
	}
	profile := doc.Deployment
	if strings.TrimSpace(profile.SSH.User) == "" || strings.TrimSpace(profile.SSH.KeyPath) == "" {
		return Bundle{}, errors.New("deployment.ssh.user and keyPath are required")
	}

	b := &builder{ctx: ctx, resolver: resolver, files: map[string]Artifact{}}
	c := chart{SSH: toSSH(profile.SSH), Bastion: toBastion(profile.Bastion)}
	for _, rule := range profile.DirectoryDefaults {
		c.DirectoryDefaults.Rules = append(c.DirectoryDefaults.Rules, chartDirectoryRule{Prefix: rule.Prefix, Owner: rule.Owner, Group: rule.Group})
	}

	byID := make(map[string]node, len(doc.Nodes))
	servers := make(map[string]*chartServer)
	serverOrder := make([]string, 0)
	for _, n := range doc.Nodes {
		byID[n.ID] = n
		if n.Kind != "vm" {
			continue
		}
		var spec vmSpec
		if err := json.Unmarshal(n.Spec, &spec); err != nil {
			return Bundle{}, fmt.Errorf("decode VM %s: %w", n.ID, err)
		}
		d := spec.Deployment
		server := chartServer{Host: spec.Host, Name: n.Name, Tags: []string{strings.ToLower(environment)}}
		if d != nil {
			server.Tags, server.SSHPort, server.BastionHost, server.BastionSSHPort = fallbackTags(d.Tags, environment), d.SSHPort, d.BastionHost, d.BastionSSHPort
			server.Bootstrap = toBootstrap(d.Bootstrap)
			server.Directories = append([]string(nil), d.Directories...)
			var err error
			if server.ExtraFiles, err = b.deliveries(d.Files, "files/server/"+n.Name); err != nil {
				return Bundle{}, fmt.Errorf("VM %s files: %w", n.Name, err)
			}
			if server.Utils, err = b.utilities(d.Utilities, n.Name); err != nil {
				return Bundle{}, fmt.Errorf("VM %s utilities: %w", n.Name, err)
			}
		}
		servers[n.ID] = &server
		serverOrder = append(serverOrder, n.ID)
	}

	for _, p := range doc.Placements {
		if p.Mode != "process" {
			continue
		}
		server := servers[p.TargetRef]
		n := byID[p.WorkloadRef]
		if server == nil || n.Kind != "java" {
			continue
		}
		app, err := b.application(n)
		if err != nil {
			return Bundle{}, err
		}
		server.Apps = append(server.Apps, app)
	}
	for _, id := range serverOrder {
		c.Servers = append(c.Servers, *servers[id])
	}
	if len(c.Servers) == 0 {
		return Bundle{}, errors.New("at least one VM is required")
	}
	body, err := yaml.Marshal(c)
	if err != nil {
		return Bundle{}, fmt.Errorf("render deploy.yml: %w", err)
	}
	files := make([]PackageFile, 0, len(b.files))
	for filePath, artifact := range b.files {
		files = append(files, PackageFile{Path: filePath, Artifact: artifact})
	}
	sortPackageFiles(files)
	return Bundle{DeployYAML: body, Files: files}, nil
}

func (b *builder) application(n node) (chartApp, error) {
	var spec javaSpec
	if err := json.Unmarshal(n.Spec, &spec); err != nil {
		return chartApp{}, fmt.Errorf("decode Java app %s: %w", n.Name, err)
	}
	if spec.ArtifactRef == "" {
		return chartApp{}, fmt.Errorf("Java app %s artifactRef is required", n.Name)
	}
	d := spec.Deployment
	if d == nil {
		d = &domain.JavaDeployment{}
	}
	baseDir := d.BaseDir
	if baseDir == "" {
		baseDir = "/app/overpass/" + n.Name
	}
	jarRemote := d.JarRemotePath
	if jarRemote == "" {
		jarRemote = baseDir + "/lib/app.jar"
	}
	jarPath, err := b.add(spec.ArtifactRef, d.JarPackagePath, "assets/"+n.Name)
	if err != nil {
		return chartApp{}, fmt.Errorf("Java app %s JAR: %w", n.Name, err)
	}
	app := chartApp{Name: n.Name, BaseDir: baseDir, Port: firstPort(spec), Jar: chartJar{LocalPath: "./" + jarPath, RemotePath: jarRemote}, JVM: chartJVM{MinHeap: d.MinHeap, MaxHeap: spec.MaxHeap, JavaOpts: d.JavaOptions}, ExtraOpts: d.ExtraOptions}
	if app.JVM.MinHeap == "" {
		app.JVM.MinHeap = "256m"
	}
	if app.ConfigFiles, err = b.deliveries(d.ConfigFiles, "config/"+n.Name); err != nil {
		return chartApp{}, fmt.Errorf("Java app %s config files: %w", n.Name, err)
	}
	if app.ExtraFiles, err = b.deliveries(d.ExtraFiles, "files/app/"+n.Name); err != nil {
		return chartApp{}, fmt.Errorf("Java app %s extra files: %w", n.Name, err)
	}
	app.Script, err = b.script(d.Script, n.Name, baseDir)
	if err != nil {
		return chartApp{}, fmt.Errorf("Java app %s script: %w", n.Name, err)
	}
	return app, nil
}

func (b *builder) script(value *domain.ScriptProfile, appName, baseDir string) (chartScript, error) {
	if value == nil {
		return chartScript{Mode: "template", Template: "embedded:server.sh.tmpl", RemotePath: baseDir + "/bin/server.sh"}, nil
	}
	result := chartScript{Mode: value.Mode, Template: value.Template, RemotePath: value.RemotePath, RemoteDir: value.RemoteDir}
	var err error
	if value.ArtifactRef != "" {
		if result.LocalPath, err = b.add(value.ArtifactRef, value.PackagePath, "scripts/"+appName); err != nil {
			return chartScript{}, err
		}
		result.LocalPath = "./" + result.LocalPath
	}
	if value.ValuesArtifactRef != "" {
		if result.ValuesFile, err = b.add(value.ValuesArtifactRef, value.ValuesPackagePath, "values/"+appName); err != nil {
			return chartScript{}, err
		}
		result.ValuesFile = "./" + result.ValuesFile
	}
	return result, nil
}

func (b *builder) deliveries(values []domain.ArtifactDelivery, defaultDir string) ([]chartFile, error) {
	result := make([]chartFile, 0, len(values))
	for _, value := range values {
		local, err := b.add(value.ArtifactRef, value.PackagePath, defaultDir)
		if err != nil {
			return nil, err
		}
		result = append(result, chartFile{LocalPath: "./" + local, RemotePath: value.RemotePath, Chmod: value.Chmod, Extract: toExtract(value.Extract)})
	}
	return result, nil
}

func (b *builder) utilities(values []domain.UtilityProfile, serverName string) ([]chartUtility, error) {
	result := make([]chartUtility, 0, len(values))
	for _, value := range values {
		files, err := b.deliveries(value.Files, "files/utility/"+serverName+"/"+value.Name)
		if err != nil {
			return nil, err
		}
		result = append(result, chartUtility{Name: value.Name, Directories: value.Directories, Files: files, Services: toServices(value.Services)})
	}
	return result, nil
}

func (b *builder) add(ref, requestedPath, defaultDir string) (string, error) {
	if strings.TrimSpace(ref) == "" {
		return "", errors.New("artifactRef is required")
	}
	artifact, err := b.resolver.Resolve(b.ctx, ref)
	if err != nil {
		return "", fmt.Errorf("resolve artifact %q: %w", ref, err)
	}
	filePath := requestedPath
	if filePath == "" {
		filePath = path.Join(defaultDir, safeBase(artifact.OriginalName))
	}
	filePath, err = safePackagePath(filePath)
	if err != nil {
		return "", err
	}
	if existing, ok := b.files[filePath]; ok && existing.ID != artifact.ID {
		return "", fmt.Errorf("package path %q is used by multiple artifacts", filePath)
	}
	b.files[filePath] = artifact
	return filePath, nil
}

func WriteZIP(w io.Writer, bundle Bundle) error {
	zw := zip.NewWriter(w)
	if err := writeZIPBytes(zw, "deploy.yml", bundle.DeployYAML); err != nil {
		return err
	}
	seen := map[string]struct{}{"deploy.yml": {}}
	for _, file := range bundle.Files {
		clean, err := safePackagePath(file.Path)
		if err != nil {
			return err
		}
		if _, exists := seen[clean]; exists {
			return fmt.Errorf("duplicate ZIP path %q", clean)
		}
		seen[clean] = struct{}{}
		entry, err := zw.Create(clean)
		if err != nil {
			return fmt.Errorf("create ZIP entry %s: %w", file.Path, err)
		}
		reader, err := file.Artifact.Open()
		if err != nil {
			return fmt.Errorf("open artifact %s: %w", file.Artifact.ID, err)
		}
		hash := sha256.New()
		written, copyErr := io.Copy(io.MultiWriter(entry, hash), reader)
		closeErr := reader.Close()
		if copyErr != nil {
			return fmt.Errorf("copy artifact %s: %w", file.Artifact.ID, copyErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close artifact %s: %w", file.Artifact.ID, closeErr)
		}
		if written != file.Artifact.Size || (file.Artifact.SHA256 != "" && hex.EncodeToString(hash.Sum(nil)) != file.Artifact.SHA256) {
			return fmt.Errorf("artifact %s changed while packaging", file.Artifact.ID)
		}
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("close ZIP: %w", err)
	}
	return nil
}

func writeZIPBytes(zw *zip.Writer, name string, body []byte) error {
	entry, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = entry.Write(body)
	return err
}

var unsafeBaseCharacters = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func safeBase(name string) string {
	value := unsafeBaseCharacters.ReplaceAllString(path.Base(strings.ReplaceAll(name, "\\", "/")), "-")
	value = strings.Trim(value, ".-")
	if value == "" {
		return "artifact.bin"
	}
	return value
}
func safePackagePath(value string) (string, error) {
	if strings.Contains(value, "\\") || strings.HasPrefix(value, "/") {
		return "", fmt.Errorf("package path %q must be relative and use forward slashes", value)
	}
	clean := path.Clean(value)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || clean != value {
		return "", fmt.Errorf("unsafe package path %q", value)
	}
	return clean, nil
}
