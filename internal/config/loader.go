package config

import (
	"fmt"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

// replaceEnvVariables replaces ${VAR} in the raw content with the corresponding environment variables.
func replaceEnvVariables(content []byte) []byte {
	re := regexp.MustCompile(`\$\{([a-zA-Z_][a-zA-Z0-9_]*)\}`)
	return re.ReplaceAllFunc(content, func(match []byte) []byte {
		envName := string(match[2 : len(match)-1])
		if val, exists := os.LookupEnv(envName); exists {
			return []byte(val)
		}
		return match // Leave unresolved to be caught by validator
	})
}

// Load loads a config file from the given path, substitutes env vars, and validates it.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	substitutedData := replaceEnvVariables(data)

	var cfg Config
	if err := yaml.Unmarshal(substitutedData, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	if err := ValidateAndApplyDefaults(&cfg); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return &cfg, nil
}
