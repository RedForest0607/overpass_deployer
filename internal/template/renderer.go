package template

import (
	"embed"
	"fmt"
	"os"
	"text/template"

	"go-deployer/internal/config"
)

//go:embed templates/*.tmpl
var defaultTemplates embed.FS

// Render renders a template either from a custom path or falls back to an embedded default.
// It returns the path to a newly created temporary file.
func Render(tmplPath string, defaultName string, data config.ScriptData) (string, error) {
	var tmplContent []byte
	var err error

	if tmplPath != "" {
		// Use user-provided template
		tmplContent, err = os.ReadFile(tmplPath)
		if err != nil {
			return "", fmt.Errorf("reading custom template %s: %w", tmplPath, err)
		}
	} else {
		// Use embedded default template
		content, err := defaultTemplates.ReadFile(fmt.Sprintf("templates/%s.tmpl", defaultName))
		if err != nil {
			return "", fmt.Errorf("reading default template %s: %w", defaultName, err)
		}
		tmplContent = content
	}

	t, err := template.New(defaultName).Parse(string(tmplContent))
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
