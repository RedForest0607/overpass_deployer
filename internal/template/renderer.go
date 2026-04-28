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

// Render는 사용자 지정 템플릿 또는 내장 템플릿을 렌더링해 임시 파일 경로를 반환한다.
func Render(tmplPath string, defaultName string, data any) (string, error) {
	tmplContent, err := readTemplateContent(tmplPath, defaultName)
	if err != nil {
		return "", err
	}

	t, err := template.New(defaultName).
		Funcs(template.FuncMap{
			"lookup":        lookupValue,
			"defaultString": defaultString,
		}).
		Option("missingkey=error").
		Parse(string(tmplContent))
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

// IsEmbeddedTemplateRef는 경로가 embedded: 접두사를 사용하는 내장 템플릿 참조인지 확인한다.
func IsEmbeddedTemplateRef(path string) bool {
	return strings.HasPrefix(path, embeddedTemplatePrefix)
}

// ValidateEmbeddedTemplateRef는 내장 템플릿 참조가 실제 임베디드 파일을 가리키는지 검증한다.
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

// readTemplateContent는 템플릿 우선순위에 따라 기본, 내장 참조, 사용자 파일 내용을 읽는다.
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

// readEmbeddedTemplate은 go:embed로 포함된 기본 템플릿 파일을 읽는다.
func readEmbeddedTemplate(name string) ([]byte, error) {
	content, err := defaultTemplates.ReadFile(fmt.Sprintf("templates/%s", name))
	if err != nil {
		return nil, fmt.Errorf("reading embedded template %s: %w", name, err)
	}
	return content, nil
}

// lookupValue는 템플릿에서 선택 값 조회 시 없는 키를 nil로 처리하기 위한 헬퍼다.
func lookupValue(data any, key string) any {
	values, ok := data.(map[string]any)
	if !ok {
		return nil
	}

	value, exists := values[key]
	if !exists {
		return nil
	}
	return value
}

// defaultString은 템플릿 문자열 값이 비었을 때 안전한 fallback을 제공한다.
func defaultString(value any, fallback string) string {
	text, ok := value.(string)
	if !ok || text == "" {
		return fallback
	}
	return text
}
