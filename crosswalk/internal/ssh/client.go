package ssh

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"sync"
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
	return c.RunContext(context.Background(), cmd)
}

// DefaultCommandTimeout is separate from the SSH connection timeout. Package
// installation may legitimately take longer than a connection handshake.
const DefaultCommandTimeout = 10 * time.Minute

func (c *Client) RunContext(ctx context.Context, cmd string) (string, error) {
	return c.run(ctx, cmd, nil)
}

// RunWithStdin은 비밀값을 명령 인자에 포함하지 않고 원격 프로세스의 표준 입력으로 전달한다.
func (c *Client) RunWithStdin(cmd string, input io.Reader) (string, error) {
	return c.RunWithStdinContext(context.Background(), cmd, input)
}

func (c *Client) RunWithStdinContext(ctx context.Context, cmd string, input io.Reader) (string, error) {
	return c.run(ctx, cmd, input)
}

func (c *Client) run(parent context.Context, cmd string, input io.Reader) (string, error) {
	ctx, cancel := context.WithTimeout(parent, DefaultCommandTimeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	// Closing the transport also releases a blocked NewSession or session.Run.
	stop := context.AfterFunc(ctx, func() { _ = c.client.Close() })
	defer stop()
	session, err := c.client.NewSession()
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("creating ssh session: %w", err)
	}
	defer session.Close()
	if input != nil {
		session.Stdin = input
	}

	var buf synchronizedBuffer
	session.Stdout = &buf
	session.Stderr = &buf

	err = runCommandContext(ctx, session, cmd)
	outStr := buf.String()

	if err != nil {
		return outStr, fmt.Errorf("running command '%s': %w", cmd, err)
	}
	return outStr, nil
}

// synchronizedBuffer는 SSH가 별도 goroutine에서 stdout과 stderr를 기록해도 안전하게 결과를 모은다.
type synchronizedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *synchronizedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *synchronizedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// Start는 원격 명령을 비동기로 시작하고 표준 출력을 스트리밍할 reader와 세션을 반환한다.
// 호출자는 로그 스트리밍을 멈출 때 반드시 반환된 session을 닫아야 한다.
func (c *Client) Start(cmd string) (*StreamingSession, io.Reader, error) {
	session, err := c.client.NewSession()
	if err != nil {
		return nil, nil, fmt.Errorf("creating ssh session: %w", err)
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		session.Close()
		return nil, nil, fmt.Errorf("creating ssh stdout pipe: %w", err)
	}

	if err := session.Start(cmd); err != nil {
		session.Close()
		return nil, nil, fmt.Errorf("starting command '%s': %w", cmd, err)
	}

	return &StreamingSession{session: session}, stdout, nil
}

// StreamingSession은 종료 시 원격 프로세스에 TERM 신호를 전달한다.
type StreamingSession struct{ session *ssh.Session }

func (s *StreamingSession) Close() error {
	_ = s.session.Signal(ssh.SIGTERM)
	completed := make(chan struct{})
	go func() {
		_ = s.session.Wait()
		close(completed)
	}()

	select {
	case <-completed:
		return nil
	case <-time.After(2 * time.Second):
		return s.session.Close()
	}
}

// RunSudo는 주어진 원격 명령을 sudo 접두어와 함께 실행한다.
func (c *Client) RunSudo(cmd string) (string, error) {
	return c.Run(fmt.Sprintf("sudo %s", cmd))
}

// Close는 SSH 연결 리소스를 닫는다.
func (c *Client) Close() error {
	return c.client.Close()
}
