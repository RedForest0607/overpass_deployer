package template

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadTemplateData reads a YAML map from disk and substitutes ${VAR} placeholders
// using the current process environment.
func LoadTemplateData(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading template values %s: %w", path, err)
	}

	substituted := replaceEnvVariables(data)

	values := make(map[string]any)
	if err := yaml.Unmarshal(substituted, &values); err != nil {
		return nil, fmt.Errorf("unmarshaling template values %s: %w", path, err)
	}
	if unresolvedPath, unresolvedValue := findUnresolvedEnv(values, ""); unresolvedPath != "" {
		return nil, fmt.Errorf("template values %s contains unresolved environment variable at %s: %s", path, unresolvedPath, unresolvedValue)
	}

	return values, nil
}

func MergeTemplateData(base map[string]any, overrides map[string]any) map[string]any {
	merged := make(map[string]any, len(base)+len(overrides))

	for key, value := range base {
		merged[key] = value
	}
	for key, value := range overrides {
		merged[key] = value
	}

	return merged
}

func replaceEnvVariables(content []byte) []byte {
	re := regexp.MustCompile(`\$\{([a-zA-Z_][a-zA-Z0-9_]*)\}`)
	return re.ReplaceAllFunc(content, func(match []byte) []byte {
		envName := string(match[2 : len(match)-1])
		if val, exists := os.LookupEnv(envName); exists {
			return []byte(val)
		}
		return match
	})
}

func findUnresolvedEnv(value any, path string) (string, string) {
	switch typed := value.(type) {
	case map[string]any:
		for key, nested := range typed {
			nextPath := key
			if path != "" {
				nextPath = path + "." + key
			}
			if unresolvedPath, unresolvedValue := findUnresolvedEnv(nested, nextPath); unresolvedPath != "" {
				return unresolvedPath, unresolvedValue
			}
		}
	case []any:
		for index, nested := range typed {
			nextPath := fmt.Sprintf("%s[%d]", path, index)
			if unresolvedPath, unresolvedValue := findUnresolvedEnv(nested, nextPath); unresolvedPath != "" {
				return unresolvedPath, unresolvedValue
			}
		}
	case string:
		if strings.Contains(typed, "${") && strings.Contains(typed, "}") {
			return path, typed
		}
	}

	return "", ""
}
