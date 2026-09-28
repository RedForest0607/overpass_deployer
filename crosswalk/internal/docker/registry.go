package docker

import (
	"fmt"
	"os"
	"strings"

	"go-deployer/internal/config"
	"go-deployer/internal/ssh"
)

// RegistryToken은 credential 값을 보존한 채 존재 여부만 검증한다.
func RegistryToken(environmentName string) (string, error) {
	token, exists := os.LookupEnv(environmentName)
	if !exists || strings.TrimSpace(token) == "" {
		return "", fmt.Errorf("registry token environment variable %s is not set", environmentName)
	}
	return token, nil
}

func registryToken(environmentName string) (string, error) {
	return RegistryToken(environmentName)
}

// LoginRegistry는 token을 명령 인자가 아닌 표준 입력으로만 전달한다.
// 실행 오류와 원격 출력은 credential 노출 방지를 위해 상위 오류에 포함하지 않는다.
func LoginRegistry(runner RemoteRunner, registry config.DockerRegistryConfig, token string) error {
	command := "docker login " + ssh.ShellQuote(registry.Host) +
		" --username " + ssh.ShellQuote(registry.Username) + " --password-stdin"
	if _, err := runner.RunWithStdin(command, strings.NewReader(token+"\n")); err != nil {
		return fmt.Errorf("registry login to %s failed", registry.Host)
	}
	return nil
}

func loginRegistry(runner RemoteRunner, registry config.DockerRegistryConfig, token string) error {
	return LoginRegistry(runner, registry, token)
}
