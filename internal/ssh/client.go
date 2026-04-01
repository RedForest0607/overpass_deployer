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

func (c *Client) Host() string {
	return c.host
}

func (c *Client) RawClient() *ssh.Client {
	return c.client
}

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

func (c *Client) RunSudo(cmd string) (string, error) {
	return c.Run(fmt.Sprintf("sudo %s", cmd))
}

func (c *Client) Close() error {
	return c.client.Close()
}
