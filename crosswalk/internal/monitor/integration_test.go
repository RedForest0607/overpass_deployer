package monitor

import (
	"bufio"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"go-deployer/internal/config"
	sshlib "golang.org/x/crypto/ssh"
)

func TestServiceLocalSSHIntegration(t *testing.T) {
	server := startLocalSSHServer(t)
	keyPath := writeClientKey(t)
	knownHostsPath := filepath.Join(t.TempDir(), "known_hosts")

	cfg := &config.Config{
		SSH: config.SSHConfig{
			User:            "local-test",
			KeyPath:         keyPath,
			KnownHosts:      knownHostsPath,
			HostKeyChecking: config.HostKeyAcceptNew,
			Port:            server.port(),
			TimeoutSec:      3,
		},
		Servers: []config.ServerConfig{{
			Name: "local-server",
			Host: "127.0.0.1",
			Apps: []config.AppConfig{{
				Name:    "sample-api",
				BaseDir: "/app/sample-api",
				Port:    8080,
				Script:  config.ScriptConfig{RemotePath: "/app/sample-api/bin/server.sh"},
			}},
		}},
	}

	service := NewService()
	statuses := service.Collect(cfg)
	if len(statuses) != 1 {
		t.Fatalf("Collect() returned %d statuses, want 1", len(statuses))
	}
	if statuses[0].State != StateRunning || statuses[0].PID != "4242" {
		t.Fatalf("Collect() = %#v, want a running process with pid 4242", statuses[0])
	}

	output, err := service.Control(cfg, Target{ServerIndex: 0, AppIndex: 0}, ActionRestart)
	if err != nil {
		t.Fatalf("Control() returned error: %v", err)
	}
	if !strings.Contains(output, "restart completed") {
		t.Fatalf("Control() output = %q, want restart result", output)
	}

	stream, err := service.OpenLog(cfg, Target{ServerIndex: 0, AppIndex: 0})
	if err != nil {
		t.Fatalf("OpenLog() returned error: %v", err)
	}

	reader := bufio.NewReader(stream.Reader)
	firstLine, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("reading first log line: %v", err)
	}
	secondLine, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("reading second log line: %v", err)
	}
	if firstLine != "first local log line\n" || secondLine != "second local log line\n" {
		t.Fatalf("unexpected log lines: %q %q", firstLine, secondLine)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("closing log stream: %v", err)
	}

	if !server.receivedStatusQuery() || !server.receivedRestart() || !server.receivedLogTail() || !server.receivedTermSignal() {
		t.Fatalf("expected status, restart, and log commands; got %q", server.commands())
	}
	if _, err := os.Stat(knownHostsPath); err != nil {
		t.Fatalf("accept-new should create known_hosts: %v", err)
	}
}

type localSSHServer struct {
	listener         net.Listener
	config           *sshlib.ServerConfig
	mu               sync.Mutex
	receivedCommands []string
	receivedSignals  []string
}

func startLocalSSHServer(t *testing.T) *localSSHServer {
	t.Helper()
	signer := generateSigner(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen on localhost: %v", err)
	}
	server := &localSSHServer{
		listener: listener,
		config:   &sshlib.ServerConfig{NoClientAuth: true},
	}
	server.config.AddHostKey(signer)

	go server.serve()
	t.Cleanup(func() { _ = listener.Close() })
	return server
}

func (s *localSSHServer) serve() {
	for {
		connection, err := s.listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			continue
		}
		go s.handleConnection(connection)
	}
}

func (s *localSSHServer) handleConnection(connection net.Conn) {
	_, channels, requests, err := sshlib.NewServerConn(connection, s.config)
	if err != nil {
		return
	}
	go sshlib.DiscardRequests(requests)
	for channel := range channels {
		if channel.ChannelType() != "session" {
			_ = channel.Reject(sshlib.UnknownChannelType, "only session channels are supported")
			continue
		}
		client, requests, err := channel.Accept()
		if err != nil {
			continue
		}
		go s.handleSession(client, requests)
	}
}

func (s *localSSHServer) handleSession(channel sshlib.Channel, requests <-chan *sshlib.Request) {
	defer channel.Close()
	for request := range requests {
		if request.Type != "exec" {
			_ = request.Reply(false, nil)
			continue
		}
		var payload struct{ Command string }
		if err := sshlib.Unmarshal(request.Payload, &payload); err != nil {
			_ = request.Reply(false, nil)
			return
		}
		_ = request.Reply(true, nil)
		s.recordCommand(payload.Command)

		switch {
		case strings.Contains(payload.Command, "pgrep -f"):
			_, _ = channel.Write([]byte("state=running\npid=4242\nprocess=00:01 0.1 0.2 java -jar sample-api.jar\n"))
			s.sendExitStatus(channel, 0)
			return
		case strings.Contains(payload.Command, "tail -n 200 -F"):
			_, _ = channel.Write([]byte("first local log line\nsecond local log line\n"))
			for signal := range requests {
				if signal.Type == "signal" {
					s.recordSignal(string(signal.Payload))
					s.sendExitStatus(channel, 143)
					return
				}
			}
			return
		case strings.Contains(payload.Command, "restart"):
			_, _ = channel.Write([]byte("restart completed\n"))
			s.sendExitStatus(channel, 0)
			return
		default:
			_, _ = channel.Write([]byte("unexpected command\n"))
			s.sendExitStatus(channel, 1)
			return
		}
	}
}

func (s *localSSHServer) sendExitStatus(channel sshlib.Channel, status uint32) {
	_, _ = channel.SendRequest("exit-status", false, sshlib.Marshal(struct{ Status uint32 }{Status: status}))
}

func (s *localSSHServer) port() int {
	return s.listener.Addr().(*net.TCPAddr).Port
}

func (s *localSSHServer) recordCommand(command string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.receivedCommands = append(s.receivedCommands, command)
}

func (s *localSSHServer) commands() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.receivedCommands...)
}

func (s *localSSHServer) receivedStatusQuery() bool {
	return containsCommand(s.commands(), "pgrep -f")
}

func (s *localSSHServer) receivedRestart() bool {
	return containsCommand(s.commands(), "restart")
}

func (s *localSSHServer) receivedLogTail() bool {
	return containsCommand(s.commands(), "tail -n 200 -F")
}

func (s *localSSHServer) recordSignal(signal string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.receivedSignals = append(s.receivedSignals, signal)
}

func (s *localSSHServer) receivedTermSignal() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, signal := range s.receivedSignals {
		if strings.Contains(signal, "TERM") {
			return true
		}
	}
	return false
}

func containsCommand(commands []string, fragment string) bool {
	for _, command := range commands {
		if strings.Contains(command, fragment) {
			return true
		}
	}
	return false
}

func generateSigner(t *testing.T) sshlib.Signer {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate server key: %v", err)
	}
	signer, err := sshlib.NewSignerFromKey(privateKey)
	if err != nil {
		t.Fatalf("create server signer: %v", err)
	}
	return signer
}

func writeClientKey(t *testing.T) string {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate client key: %v", err)
	}
	encoded, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("marshal client key: %v", err)
	}
	path := filepath.Join(t.TempDir(), "id_rsa")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded}), 0o600); err != nil {
		t.Fatalf("write client key: %v", err)
	}
	return path
}
