package ssh

import (
	"bytes"
	"fmt"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
)

type Runner interface {
	Run(cmd string) (string, error)
	RunSudo(cmd string) (string, error)
	Host() string
	Close() error
}

type Client struct {
	client *ssh.Client
	host   string
}

// Connect는 개인키 인증과 호스트 키 검증 정책을 적용해 SSH 클라이언트를 생성한다.
func Connect(user, host, keyPath, hostKeyChecking, knownHostsPath string, port int, timeoutSec int) (*Client, error) {
	key, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("reading ssh key: %w", err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("parsing ssh key: %w", err)
	}

	hostKeyCallback, err := NewHostKeyCallback(hostKeyChecking, knownHostsPath)
	if err != nil {
		return nil, fmt.Errorf("building host key callback: %w", err)
	}

	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: hostKeyCallback,
		Timeout:         time.Duration(timeoutSec) * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("dialing ssh: %w", err)
	}

	return &Client{
		client: client,
		host:   host,
	}, nil
}

// Host는 로그와 오류 메시지에 사용할 원격 호스트명을 반환한다.
func (c *Client) Host() string {
	return c.host
}

// RawClient는 SFTP처럼 하위 라이브러리가 필요한 원본 SSH 클라이언트를 반환한다.
func (c *Client) RawClient() *ssh.Client {
	return c.client
}

// Run은 원격 명령을 실행하고 stdout/stderr를 함께 수집해 호출자에게 돌려준다.
func (c *Client) Run(cmd string) (string, error) {
	session, err := c.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("creating ssh session: %w", err)
	}
	defer session.Close()

	var buf bytes.Buffer
	session.Stdout = &buf
	session.Stderr = &buf

	err = session.Run(cmd)
	outStr := buf.String()

	if err != nil {
		return outStr, fmt.Errorf("running command '%s': %w", cmd, err)
	}
	return outStr, nil
}

// RunSudo는 주어진 원격 명령을 sudo 접두어와 함께 실행한다.
func (c *Client) RunSudo(cmd string) (string, error) {
	return c.Run(fmt.Sprintf("sudo %s", cmd))
}

// Close는 SSH 연결 리소스를 닫는다.
func (c *Client) Close() error {
	return c.client.Close()
}
