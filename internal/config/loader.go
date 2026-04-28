package config

import (
	"fmt"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

// replaceEnvVariables는 설정 파일의 ${VAR} 플레이스홀더를 현재 환경 변수 값으로 치환한다.
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

// Load는 설정 파일을 읽고 환경 변수 치환, YAML 파싱, 기본값 적용과 검증을 한 번에 수행한다.
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
