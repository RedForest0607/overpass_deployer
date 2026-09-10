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
	return load(path, ValidateAndApplyDefaults)
}

// LoadForMonitoring은 원격 상태와 로그 확인에 필요한 최소 설정만 검증한다.
// 배포 자산의 로컬 존재 여부는 확인하지 않아, 대상 bastion에서도 독립적으로 실행할 수 있다.
func LoadForMonitoring(path string) (*Config, error) {
	return load(path, ValidateForMonitoring)
}

func load(path string, validate func(*Config) error) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	substitutedData := replaceEnvVariables(data)

	var cfg Config
	if err := yaml.Unmarshal(substitutedData, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return &cfg, nil
}
