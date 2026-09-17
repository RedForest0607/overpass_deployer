package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"studio-backend/internal/sqlite"
	"studio-backend/internal/storage"
)

const (
	Format          = "overpass-studio-backup/v1"
	manifestPath    = "manifest.json"
	databasePath    = "data/studio.db"
	maxManifestSize = int64(4 << 20)
)

type Entry struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type Manifest struct {
	Format    string    `json:"format"`
	CreatedAt time.Time `json:"createdAt"`
	Entries   []Entry   `json:"entries"`
}

type source struct {
	entry Entry
	file  string
	data  []byte
}

func Create(ctx context.Context, dataDir, outputPath string) (Manifest, error) {
	if err := storage.Inspect(dataDir); err != nil {
		return Manifest{}, fmt.Errorf("inspect backup source: %w", err)
	}
	absoluteData, err := filepath.Abs(dataDir)
	if err != nil {
		return Manifest{}, fmt.Errorf("resolve backup source: %w", err)
	}
	database, err := sqlite.Snapshot(ctx, filepath.Join(absoluteData, "studio.db"))
	if err != nil {
		return Manifest{}, err
	}
	sources := []source{{entry: entryForBytes(databasePath, database), data: database}}
	for _, directory := range []struct {
		name string
		path string
	}{
		{name: "artifacts", path: filepath.Join(absoluteData, "artifacts")},
		{name: "packages", path: filepath.Join(absoluteData, "packages")},
	} {
		values, collectErr := collect(directory.path, path.Join("data", directory.name))
		if collectErr != nil {
			return Manifest{}, collectErr
		}
		sources = append(sources, values...)
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].entry.Path < sources[j].entry.Path })
	manifest := Manifest{Format: Format, CreatedAt: time.Now().UTC(), Entries: make([]Entry, len(sources))}
	for index, value := range sources {
		manifest.Entries[index] = value.entry
	}
	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return Manifest{}, fmt.Errorf("encode backup manifest: %w", err)
	}
	manifestJSON = append(manifestJSON, '\n')

	output, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return Manifest{}, fmt.Errorf("create backup archive without overwrite: %w", err)
	}
	gzipWriter := gzip.NewWriter(output)
	tarWriter := tar.NewWriter(gzipWriter)
	writeErr := writeBytes(tarWriter, manifestPath, manifestJSON)
	for _, value := range sources {
		if writeErr != nil {
			break
		}
		if value.file == "" {
			writeErr = writeBytes(tarWriter, value.entry.Path, value.data)
		} else {
			writeErr = writeFile(tarWriter, value)
		}
	}
	if err := tarWriter.Close(); writeErr == nil {
		writeErr = err
	}
	if err := gzipWriter.Close(); writeErr == nil {
		writeErr = err
	}
	if err := output.Sync(); writeErr == nil {
		writeErr = err
	}
	if err := output.Close(); writeErr == nil {
		writeErr = err
	}
	if writeErr != nil {
		return Manifest{}, fmt.Errorf("write backup archive; partial output was kept at %q: %w", outputPath, writeErr)
	}
	return manifest, nil
}

func Verify(archivePath string) (Manifest, error) {
	reader, closeArchive, err := openArchive(archivePath)
	if err != nil {
		return Manifest{}, err
	}
	defer closeArchive()
	manifest, err := readManifest(reader)
	if err != nil {
		return Manifest{}, err
	}
	expected, err := validateManifest(manifest)
	if err != nil {
		return Manifest{}, err
	}
	seen := make(map[string]struct{}, len(expected))
	for {
		header, nextErr := reader.Next()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			return Manifest{}, fmt.Errorf("read backup entry: %w", nextErr)
		}
		entry, ok := expected[header.Name]
		if !ok || header.Typeflag != tar.TypeReg || header.Size != entry.Size {
			return Manifest{}, fmt.Errorf("unexpected backup entry %q", header.Name)
		}
		if _, duplicate := seen[header.Name]; duplicate {
			return Manifest{}, fmt.Errorf("duplicate backup entry %q", header.Name)
		}
		hash := sha256.New()
		written, copyErr := io.Copy(hash, reader)
		if copyErr != nil || written != entry.Size || hex.EncodeToString(hash.Sum(nil)) != entry.SHA256 {
			return Manifest{}, fmt.Errorf("backup entry %q failed checksum verification", header.Name)
		}
		seen[header.Name] = struct{}{}
	}
	if len(seen) != len(expected) {
		return Manifest{}, fmt.Errorf("backup archive has %d of %d manifest entries", len(seen), len(expected))
	}
	return manifest, nil
}

func Restore(ctx context.Context, archivePath, dataDir string) (Manifest, error) {
	manifest, err := Verify(archivePath)
	if err != nil {
		return Manifest{}, err
	}
	absoluteTarget, err := filepath.Abs(dataDir)
	if err != nil {
		return Manifest{}, fmt.Errorf("resolve restore target: %w", err)
	}
	if absoluteTarget == string(filepath.Separator) {
		return Manifest{}, errors.New("filesystem root cannot be a restore target")
	}
	if _, err := os.Lstat(absoluteTarget); err == nil {
		return Manifest{}, fmt.Errorf("restore target %q already exists", absoluteTarget)
	} else if !os.IsNotExist(err) {
		return Manifest{}, fmt.Errorf("inspect restore target: %w", err)
	}
	parent := filepath.Dir(absoluteTarget)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return Manifest{}, fmt.Errorf("create restore parent: %w", err)
	}
	staging, err := os.MkdirTemp(parent, "."+filepath.Base(absoluteTarget)+".restore-")
	if err != nil {
		return Manifest{}, fmt.Errorf("create restore staging directory: %w", err)
	}
	if err := os.Chmod(staging, 0o700); err != nil {
		return Manifest{}, fmt.Errorf("secure restore staging directory %q: %w", staging, err)
	}
	if err := extract(archivePath, staging); err != nil {
		return Manifest{}, fmt.Errorf("extract backup; incomplete staging data was kept at %q: %w", staging, err)
	}
	for _, directory := range []string{"artifacts", "packages", "tmp"} {
		if err := os.MkdirAll(filepath.Join(staging, directory), 0o700); err != nil {
			return Manifest{}, fmt.Errorf("create restored %s directory at %q: %w", directory, staging, err)
		}
	}
	if err := sqlite.CheckIntegrity(ctx, filepath.Join(staging, "studio.db")); err != nil {
		return Manifest{}, fmt.Errorf("validate restored database at %q: %w", staging, err)
	}
	if err := storage.Inspect(staging); err != nil {
		return Manifest{}, fmt.Errorf("validate restored permissions at %q: %w", staging, err)
	}
	if err := os.Rename(staging, absoluteTarget); err != nil {
		return Manifest{}, fmt.Errorf("publish restored data from %q: %w", staging, err)
	}
	return manifest, nil
}

func collect(root, archiveRoot string) ([]source, error) {
	var sources []source
	err := filepath.WalkDir(root, func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("backup source %q is not a regular file", filePath)
		}
		relative, err := filepath.Rel(root, filePath)
		if err != nil {
			return err
		}
		hash, err := checksum(filePath)
		if err != nil {
			return err
		}
		sources = append(sources, source{
			entry: Entry{Path: path.Join(archiveRoot, filepath.ToSlash(relative)), Size: info.Size(), SHA256: hash},
			file:  filePath,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("collect backup files from %q: %w", root, err)
	}
	return sources, nil
}

func entryForBytes(name string, value []byte) Entry {
	sum := sha256.Sum256(value)
	return Entry{Path: name, Size: int64(len(value)), SHA256: hex.EncodeToString(sum[:])}
}

func checksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func writeBytes(writer *tar.Writer, name string, value []byte) error {
	if err := writer.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(value)), Typeflag: tar.TypeReg}); err != nil {
		return err
	}
	_, err := writer.Write(value)
	return err
}

func writeFile(writer *tar.Writer, value source) error {
	if err := writer.WriteHeader(&tar.Header{Name: value.entry.Path, Mode: 0o600, Size: value.entry.Size, Typeflag: tar.TypeReg}); err != nil {
		return err
	}
	file, err := os.Open(value.file)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(writer, hash), file)
	if err != nil {
		return err
	}
	if written != value.entry.Size || hex.EncodeToString(hash.Sum(nil)) != value.entry.SHA256 {
		return fmt.Errorf("backup source %q changed while it was being archived", value.file)
	}
	return nil
}

func openArchive(archivePath string) (*tar.Reader, func() error, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return nil, nil, fmt.Errorf("open backup archive: %w", err)
	}
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		file.Close()
		return nil, nil, fmt.Errorf("open backup compression: %w", err)
	}
	closeArchive := func() error {
		gzipErr := gzipReader.Close()
		fileErr := file.Close()
		if gzipErr != nil {
			return gzipErr
		}
		return fileErr
	}
	return tar.NewReader(gzipReader), closeArchive, nil
}

func readManifest(reader *tar.Reader) (Manifest, error) {
	header, err := reader.Next()
	if err != nil {
		return Manifest{}, fmt.Errorf("read backup manifest: %w", err)
	}
	if header.Name != manifestPath || header.Typeflag != tar.TypeReg || header.Size <= 0 || header.Size > maxManifestSize {
		return Manifest{}, errors.New("backup manifest must be the first regular archive entry")
	}
	var manifest Manifest
	decoder := json.NewDecoder(io.LimitReader(reader, maxManifestSize))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode backup manifest: %w", err)
	}
	return manifest, nil
}

func validateManifest(manifest Manifest) (map[string]Entry, error) {
	if manifest.Format != Format || manifest.CreatedAt.IsZero() || len(manifest.Entries) == 0 {
		return nil, errors.New("unsupported or incomplete backup manifest")
	}
	entries := make(map[string]Entry, len(manifest.Entries))
	databaseFound := false
	for _, entry := range manifest.Entries {
		if !validDataPath(entry.Path) || entry.Size < 0 || len(entry.SHA256) != 64 {
			return nil, fmt.Errorf("invalid backup manifest entry %q", entry.Path)
		}
		if _, err := hex.DecodeString(entry.SHA256); err != nil {
			return nil, fmt.Errorf("invalid checksum for backup entry %q", entry.Path)
		}
		if _, exists := entries[entry.Path]; exists {
			return nil, fmt.Errorf("duplicate backup manifest entry %q", entry.Path)
		}
		entries[entry.Path] = entry
		databaseFound = databaseFound || entry.Path == databasePath
	}
	if !databaseFound {
		return nil, errors.New("backup manifest does not contain data/studio.db")
	}
	return entries, nil
}

func validDataPath(name string) bool {
	if name != path.Clean(name) || strings.Contains(name, "\\") {
		return false
	}
	if name == databasePath {
		return true
	}
	return strings.HasPrefix(name, "data/artifacts/") || strings.HasPrefix(name, "data/packages/")
}

func extract(archivePath, staging string) error {
	reader, closeArchive, err := openArchive(archivePath)
	if err != nil {
		return err
	}
	defer closeArchive()
	if _, err := readManifest(reader); err != nil {
		return err
	}
	for {
		header, nextErr := reader.Next()
		if errors.Is(nextErr, io.EOF) {
			return nil
		}
		if nextErr != nil {
			return nextErr
		}
		relative := strings.TrimPrefix(header.Name, "data/")
		target := filepath.Join(staging, filepath.FromSlash(relative))
		if !strings.HasPrefix(target, staging+string(filepath.Separator)) || !validDataPath(header.Name) {
			return fmt.Errorf("unsafe restore entry %q", header.Name)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return err
		}
		written, copyErr := io.Copy(file, reader)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if written != header.Size {
			return fmt.Errorf("restore entry %q size changed", header.Name)
		}
	}
}
