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
const ChartFormat = "crosswalk/v1"

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
type hostedServiceSpec struct {
	Location   string                    `json:"location"`
	Deployment *domain.UtilityDeployment `json:"deployment,omitempty"`
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
	c := chart{Format: ChartFormat, SSH: toSSH(profile.SSH), Bastion: toBastion(profile.Bastion), Bootstrap: toBootstrap(profile.Bootstrap)}
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
		// Container placements are owned by the independent crosswalk-docker
		// generator so one Blueprint can produce either package type.
		if p.Mode == "container" {
			continue
		}
		if p.Mode != "process" {
			return Bundle{}, fmt.Errorf("placement %s -> %s uses unsupported Crosswalk mode %q", p.WorkloadRef, p.TargetRef, p.Mode)
		}
		server := servers[p.TargetRef]
		if server == nil {
			return Bundle{}, fmt.Errorf("placement target %q is not a VM", p.TargetRef)
		}
		n, exists := byID[p.WorkloadRef]
		if !exists {
			return Bundle{}, fmt.Errorf("placement workload %q does not exist", p.WorkloadRef)
		}
		switch n.Kind {
		case "java":
			app, err := b.application(n)
			if err != nil {
				return Bundle{}, err
			}
			server.Apps = append(server.Apps, app)
		case "redis", "elasticsearch":
			utility, err := b.hostedUtility(n, server.Name)
			if err != nil {
				return Bundle{}, err
			}
			for _, existing := range server.Utils {
				if existing.Name == utility.Name {
					return Bundle{}, fmt.Errorf("VM %s utility name %q is duplicated", server.Name, utility.Name)
				}
			}
			server.Utils = append(server.Utils, utility)
		default:
			return Bundle{}, fmt.Errorf("placement workload %s has unsupported Crosswalk kind %q", n.Name, n.Kind)
		}
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
	jarPath, err := b.add(spec.ArtifactRef, d.JarPackagePath, "assets/"+n.Name, "jar")
	if err != nil {
		return chartApp{}, fmt.Errorf("Java app %s JAR: %w", n.Name, err)
	}
	app := chartApp{Name: n.Name, Tags: append([]string(nil), d.Tags...), BaseDir: baseDir, Port: firstPort(spec), Jar: chartJar{LocalPath: "./" + jarPath, RemotePath: jarRemote}, JVM: chartJVM{MinHeap: d.MinHeap, MaxHeap: spec.MaxHeap, JavaOpts: d.JavaOptions}, ExtraOpts: d.ExtraOptions}
	if app.JVM.MinHeap == "" {
		app.JVM.MinHeap = "256m"
	}
	if app.ConfigFiles, err = b.configDeliveries(d.ConfigFiles, "config/"+n.Name); err != nil {
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

func (b *builder) hostedUtility(n node, serverName string) (chartUtility, error) {
	var spec hostedServiceSpec
	if err := json.Unmarshal(n.Spec, &spec); err != nil {
		return chartUtility{}, fmt.Errorf("decode %s %s: %w", n.Kind, n.Name, err)
	}
	if spec.Location != "hosted" {
		return chartUtility{}, fmt.Errorf("%s %s must be hosted when placed on a VM", n.Kind, n.Name)
	}
	if spec.Deployment == nil || (len(spec.Deployment.Directories) == 0 && len(spec.Deployment.Files) == 0 && len(spec.Deployment.Services) == 0) {
		return chartUtility{}, fmt.Errorf("hosted %s %s deployment must define directories, files, or services", n.Kind, n.Name)
	}
	files, err := b.deliveries(spec.Deployment.Files, "files/utility/"+serverName+"/"+n.Name)
	if err != nil {
		return chartUtility{}, fmt.Errorf("%s %s files: %w", n.Kind, n.Name, err)
	}
	return chartUtility{Name: n.Name, Directories: append([]string(nil), spec.Deployment.Directories...), Files: files, Services: toServices(spec.Deployment.Services)}, nil
}

func (b *builder) script(value *domain.ScriptProfile, appName, baseDir string) (chartScript, error) {
	if value == nil {
		return chartScript{Mode: "template", Template: "embedded:server.sh.tmpl", RemotePath: baseDir + "/bin/server.sh"}, nil
	}
	mode := value.Mode
	// Early Studio drafts used "local" before the Crosswalk contract was
	// aligned on "local-file". Keep stored drafts packageable while only
	// accepting the canonical value in newly validated blueprints.
	if mode == "local" {
		mode = "local-file"
	}
	if mode != "template" && mode != "local-file" {
		return chartScript{}, fmt.Errorf("unsupported script mode %q", value.Mode)
	}
	if mode == "local-file" && strings.TrimSpace(value.ArtifactRef) == "" {
		return chartScript{}, errors.New("local-file script requires artifactRef")
	}
	if mode == "local-file" && (strings.TrimSpace(value.Template) != "" || strings.TrimSpace(value.ValuesArtifactRef) != "") {
		return chartScript{}, errors.New("local-file script cannot use template or valuesArtifactRef")
	}
	result := chartScript{Mode: mode, Template: value.Template, RemotePath: value.RemotePath, RemoteDir: value.RemoteDir}
	var err error
	if value.ArtifactRef != "" {
		if result.LocalPath, err = b.add(value.ArtifactRef, value.PackagePath, "scripts/"+appName, "file"); err != nil {
			return chartScript{}, err
		}
		result.LocalPath = "./" + result.LocalPath
	}
	if value.ValuesArtifactRef != "" {
		if result.ValuesFile, err = b.add(value.ValuesArtifactRef, value.ValuesPackagePath, "values/"+appName, "script-values"); err != nil {
			return chartScript{}, err
		}
		result.ValuesFile = "./" + result.ValuesFile
	}
	return result, nil
}

func (b *builder) deliveries(values []domain.ArtifactDelivery, defaultDir string) ([]chartFile, error) {
	result := make([]chartFile, 0, len(values))
	for _, value := range values {
		allowedKinds := []string{"file", "archive", "config-template", "env-example"}
		if value.Extract != nil && value.Extract.Enabled {
			allowedKinds = []string{"archive"}
		}
		local, err := b.add(value.ArtifactRef, value.PackagePath, defaultDir, allowedKinds...)
		if err != nil {
			return nil, err
		}
		if value.Extract != nil && value.Extract.Enabled && (!isCrosswalkExtractableArchive(b.files[local].OriginalName) || !isCrosswalkExtractableArchive(local)) {
			return nil, fmt.Errorf("artifact %q cannot be extracted by Crosswalk; expected .tar, .tar.gz, or .tgz", value.ArtifactRef)
		}
		result = append(result, chartFile{LocalPath: "./" + local, RemotePath: value.RemotePath, Chmod: value.Chmod, Extract: toExtract(value.Extract)})
	}
	return result, nil
}

func (b *builder) configDeliveries(values []domain.ConfigArtifactDelivery, defaultDir string) ([]chartConfigFile, error) {
	result := make([]chartConfigFile, 0, len(values))
	for _, value := range values {
		local, err := b.add(value.ArtifactRef, value.PackagePath, defaultDir, "config-template", "env-example", "file")
		if err != nil {
			return nil, err
		}
		result = append(result, chartConfigFile{LocalPath: "./" + local, RemotePath: value.RemotePath})
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

func (b *builder) add(ref, requestedPath, defaultDir string, allowedKinds ...string) (string, error) {
	if strings.TrimSpace(ref) == "" {
		return "", errors.New("artifactRef is required")
	}
	artifact, err := b.resolver.Resolve(b.ctx, ref)
	if err != nil {
		return "", fmt.Errorf("resolve artifact %q: %w", ref, err)
	}
	if !containsString(allowedKinds, artifact.Kind) {
		return "", fmt.Errorf(
			"artifact %q kind %q is not allowed; expected one of %s",
			ref,
			artifact.Kind,
			strings.Join(allowedKinds, ", "),
		)
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

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func isCrosswalkExtractableArchive(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".tar") || strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz")
}

func WriteZIP(w io.Writer, bundle Bundle) error {
	return WriteZIPContext(context.Background(), w, bundle)
}

func WriteZIPContext(ctx context.Context, w io.Writer, bundle Bundle) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	zw := zip.NewWriter(w)
	if err := writeZIPBytes(zw, "deploy.yml", bundle.DeployYAML); err != nil {
		return err
	}
	seen := map[string]struct{}{"deploy.yml": {}}
	for _, file := range bundle.Files {
		if err := ctx.Err(); err != nil {
			return err
		}
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
		written, copyErr, closeErr := copyArtifactContext(ctx, io.MultiWriter(entry, hash), reader)
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
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("close ZIP: %w", err)
	}
	return nil
}

func copyArtifactContext(ctx context.Context, writer io.Writer, reader io.ReadCloser) (int64, error, error) {
	stop := context.AfterFunc(ctx, func() { _ = reader.Close() })
	written, copyErr := io.Copy(writer, reader)
	stop()
	closeErr := reader.Close()
	if err := ctx.Err(); err != nil {
		copyErr = err
	}
	return written, copyErr, closeErr
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
