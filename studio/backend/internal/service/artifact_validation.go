package service

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/xml"
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"studio-backend/internal/domain"
	"unicode/utf8"
)

func validateArtifact(kind domain.ArtifactKind, name, filePath string, size int64) error {
	if size == 0 {
		return errors.New("artifact must not be empty")
	}
	switch kind {
	case domain.ArtifactJAR:
		if strings.ToLower(filepath.Ext(name)) != ".jar" {
			return errors.New("JAR artifact must use .jar extension")
		}
		return validateZIP(filePath)
	case domain.ArtifactConfig, domain.ArtifactEnvExample, domain.ArtifactScriptValues:
		body, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}
		if !utf8.Valid(body) || bytes.IndexByte(body, 0) >= 0 {
			return errors.New("text artifact must be UTF-8 without NUL bytes")
		}
		return validateEditableTextSyntax(name, body)
	case domain.ArtifactArchive:
		lower := strings.ToLower(name)
		if !(strings.HasSuffix(lower, ".zip") || strings.HasSuffix(lower, ".tar") || strings.HasSuffix(lower, ".tgz") || strings.HasSuffix(lower, ".tar.gz")) {
			return errors.New("archive must be .zip, .tar, .tgz, or .tar.gz")
		}
		if strings.HasSuffix(lower, ".zip") {
			return validateZIP(filePath)
		}
		return validateTar(filePath, strings.HasSuffix(lower, ".tgz") || strings.HasSuffix(lower, ".tar.gz"))
	}
	if isEditableTextName(name) {
		body, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}
		if !utf8.Valid(body) || bytes.IndexByte(body, 0) >= 0 {
			return errors.New("editable text artifact must be UTF-8 without NUL bytes")
		}
		return validateEditableTextSyntax(name, body)
	}
	return nil
}

func validateEditableTextSyntax(name string, body []byte) error {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".yml"), strings.HasSuffix(lower, ".yaml"):
		decoder := yaml.NewDecoder(bytes.NewReader(body))
		for {
			var value any
			if err := decoder.Decode(&value); errors.Is(err, io.EOF) {
				return nil
			} else if err != nil {
				return fmt.Errorf("YAML syntax is invalid: %w", err)
			}
		}
	case strings.HasSuffix(lower, ".xml"):
		decoder := xml.NewDecoder(bytes.NewReader(body))
		depth := 0
		roots := 0
		for {
			token, err := decoder.Token()
			if errors.Is(err, io.EOF) {
				if roots != 1 {
					return errors.New("XML syntax is invalid: exactly one root element is required")
				}
				return nil
			}
			if err != nil {
				return fmt.Errorf("XML syntax is invalid: %w", err)
			}
			switch value := token.(type) {
			case xml.StartElement:
				if depth == 0 {
					roots++
					if roots > 1 {
						return errors.New("XML syntax is invalid: multiple root elements")
					}
				}
				depth++
			case xml.EndElement:
				depth--
			case xml.CharData:
				if depth == 0 && strings.TrimSpace(string(value)) != "" {
					return errors.New("XML syntax is invalid: text outside the root element")
				}
			}
		}
	default:
		return nil
	}
}

func isEditableTextName(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".yml") ||
		strings.HasSuffix(lower, ".yaml") ||
		strings.HasSuffix(lower, ".xml")
}

func validateZIP(filePath string) error {
	reader, err := zip.OpenReader(filePath)
	if err != nil {
		return errors.New("artifact is not a valid ZIP/JAR")
	}
	defer reader.Close()
	if len(reader.File) == 0 {
		return errors.New("ZIP/JAR must contain at least one entry")
	}
	if len(reader.File) > 100000 {
		return errors.New("ZIP/JAR has too many entries")
	}
	var total uint64
	for _, entry := range reader.File {
		mode := entry.FileInfo().Mode()
		if strings.Contains(entry.Name, "\\") || (!mode.IsRegular() && !mode.IsDir()) {
			return errors.New("ZIP/JAR contains an unsafe entry type or path")
		}
		clean := path.Clean(entry.Name)
		if strings.HasPrefix(clean, "/") || clean == ".." || strings.HasPrefix(clean, "../") {
			return errors.New("ZIP/JAR contains an unsafe entry path")
		}
		if entry.UncompressedSize64 > 100<<20 && entry.CompressedSize64 > 0 && entry.UncompressedSize64/entry.CompressedSize64 > 100 {
			return errors.New("ZIP/JAR compression ratio is too high")
		}
		total += entry.UncompressedSize64
		if total > 1<<30 {
			return errors.New("ZIP/JAR uncompressed size is too large")
		}
	}
	return nil
}

func validateTar(filePath string, compressed bool) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	var source io.Reader = file
	if compressed {
		reader, err := gzip.NewReader(file)
		if err != nil {
			return errors.New("archive is not a valid gzip stream")
		}
		defer reader.Close()
		source = reader
	}
	reader := tar.NewReader(source)
	entries := 0
	var total int64
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return errors.New("archive is not a valid TAR")
		}
		entries++
		if entries > 100000 {
			return errors.New("TAR has too many entries")
		}
		clean := path.Clean(header.Name)
		if strings.Contains(header.Name, "\\") || strings.HasPrefix(clean, "/") || clean == ".." || strings.HasPrefix(clean, "../") {
			return errors.New("TAR contains an unsafe entry path")
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA && header.Typeflag != tar.TypeDir {
			return errors.New("TAR contains an unsafe entry type")
		}
		if header.Size < 0 || total > 1<<30-header.Size {
			return errors.New("TAR uncompressed size is too large")
		}
		total += header.Size
	}
	if entries == 0 {
		return errors.New("TAR must contain at least one entry")
	}
	return nil
}
