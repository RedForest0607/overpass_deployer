package docker

import (
	"errors"
	"strings"
)

const dockerVersionCommand = "docker version --format '{{.Server.Version}}'"

// verifyDockerEngine은 Docker CLI 존재 여부와 daemon socket 접근 권한을 함께 확인한다.
// 원격 출력에는 환경별 정보가 섞일 수 있으므로 호출자 로그로 전달하지 않는다.
func verifyDockerEngine(runner remoteRunner) error {
	version, err := runner.Run(dockerVersionCommand)
	if err != nil || strings.TrimSpace(version) == "" {
		return errors.New("Docker daemon is unavailable or the SSH user lacks permission; install Docker and verify daemon socket access")
	}
	return nil
}
