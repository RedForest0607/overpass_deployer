package template

import (
	"embed"
	"fmt"
	"os"
	"strings"
	"text/template"
)

//go:embed templates/*.tmpl
var defaultTemplates embed.FS

const embeddedTemplatePrefix = "embedded:"

// Render renders a template either from a custom path or falls back to an embedded default.
// It returns the path to a newly created temporary file.
func Render(tmplPath string, defaultName string, data any) (string, error) {
	tmplContent, err := readTemplateContent(tmplPath, defaultName)
	if err != nil {
		return "", err
	}

	t, err := template.New(defaultName).Option("missingkey=error").Parse(string(tmplContent))
	if err != nil {
		return "", fmt.Errorf("parsing template: %w", err)
	}

	tmpFile, err := os.CreateTemp("", fmt.Sprintf("%s-*", defaultName))
	if err != nil {
		return "", fmt.Errorf("creating temp file for template: %w", err)
	}

	if err := t.Execute(tmpFile, data); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("executing template: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return "", fmt.Errorf("closing temp file: %w", err)
	}

	return tmpFile.Name(), nil
}

func IsEmbeddedTemplateRef(path string) bool {
	return strings.HasPrefix(path, embeddedTemplatePrefix)
}

func ValidateEmbeddedTemplateRef(path string) error {
	if !IsEmbeddedTemplateRef(path) {
		return nil
	}

	_, err := readEmbeddedTemplate(strings.TrimPrefix(path, embeddedTemplatePrefix))
	if err != nil {
		return err
	}
	return nil
}

func readTemplateContent(tmplPath string, defaultName string) ([]byte, error) {
	switch {
	case tmplPath == "":
		return readEmbeddedTemplate(defaultName + ".tmpl")
	case IsEmbeddedTemplateRef(tmplPath):
		return readEmbeddedTemplate(strings.TrimPrefix(tmplPath, embeddedTemplatePrefix))
	default:
		content, err := os.ReadFile(tmplPath)
		if err != nil {
			return nil, fmt.Errorf("reading custom template %s: %w", tmplPath, err)
		}
		return content, nil
	}
}

func readEmbeddedTemplate(name string) ([]byte, error) {
	content, err := defaultTemplates.ReadFile(fmt.Sprintf("templates/%s", name))
	if err != nil {
		return nil, fmt.Errorf("reading embedded template %s: %w", name, err)
	}
	return content, nil
}
