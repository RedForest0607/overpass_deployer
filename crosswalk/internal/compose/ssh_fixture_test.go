package compose

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/pkg/sftp"
	sshlib "golang.org/x/crypto/ssh"
)

type composeSSHFixture struct {
	listener net.Listener
	root     string
	stateDir string
	keyPath  string
	binDir   string
}

func startComposeSSHFixture(t *testing.T) *composeSSHFixture {
	t.Helper()
	root := t.TempDir()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("start fixture listener: %v", err)
	}
	fixture := &composeSSHFixture{
		listener: listener,
		root:     root,
		stateDir: filepath.Join(root, "docker-state"),
		keyPath:  writeComposeFixtureKey(t, root),
		binDir:   filepath.Join(root, "bin"),
	}
	if err := os.MkdirAll(fixture.binDir, 0o700); err != nil {
		t.Fatalf("create fixture bin directory: %v", err)
	}
	installComposeFixtureCommand(t, filepath.Join("..", "..", "tests", "compose", "fake-docker.sh"), filepath.Join(fixture.binDir, "docker"))
	installComposeFixtureCommand(t, filepath.Join("..", "..", "tests", "compose", "fake-mv.sh"), filepath.Join(fixture.binDir, "mv"))

	serverConfig := &sshlib.ServerConfig{NoClientAuth: true}
	serverConfig.AddHostKey(generateComposeFixtureSigner(t))
	go fixture.serve(serverConfig)
	t.Cleanup(func() { _ = listener.Close() })
	return fixture
}

func (f *composeSSHFixture) port() int {
	return f.listener.Addr().(*net.TCPAddr).Port
}

func (f *composeSSHFixture) serve(config *sshlib.ServerConfig) {
	for {
		connection, err := f.listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			continue
		}
		go f.handleConnection(connection, config)
	}
}

func (f *composeSSHFixture) handleConnection(connection net.Conn, config *sshlib.ServerConfig) {
	_, channels, requests, err := sshlib.NewServerConn(connection, config)
	if err != nil {
		return
	}
	go sshlib.DiscardRequests(requests)
	for channelRequest := range channels {
		if channelRequest.ChannelType() != "session" {
			_ = channelRequest.Reject(sshlib.UnknownChannelType, "session channel required")
			continue
		}
		channel, requests, err := channelRequest.Accept()
		if err == nil {
			go f.handleSession(channel, requests)
		}
	}
}

func (f *composeSSHFixture) handleSession(channel sshlib.Channel, requests <-chan *sshlib.Request) {
	defer channel.Close()
	for request := range requests {
		switch request.Type {
		case "exec":
			var payload struct{ Command string }
			if err := sshlib.Unmarshal(request.Payload, &payload); err != nil {
				_ = request.Reply(false, nil)
				return
			}
			_ = request.Reply(true, nil)
			f.runCommand(channel, payload.Command)
			return
		case "subsystem":
			var payload struct{ Name string }
			if err := sshlib.Unmarshal(request.Payload, &payload); err != nil || payload.Name != "sftp" {
				_ = request.Reply(false, nil)
				return
			}
			_ = request.Reply(true, nil)
			server, err := sftp.NewServer(channel)
			if err == nil {
				err = server.Serve()
				_ = server.Close()
			}
			if err != nil && !errors.Is(err, io.EOF) {
				f.sendExitStatus(channel, 1)
			} else {
				f.sendExitStatus(channel, 0)
			}
			return
		default:
			_ = request.Reply(false, nil)
		}
	}
}

func (f *composeSSHFixture) runCommand(channel sshlib.Channel, command string) {
	process := exec.Command("/bin/sh", "-c", command)
	process.Dir = f.root
	process.Env = append(os.Environ(),
		"PATH="+f.binDir+":"+os.Getenv("PATH"),
		"COMPOSE_FIXTURE_STATE="+f.stateDir,
		"COMPOSE_FIXTURE_TOKEN=fixture-token",
	)
	process.Stdin = channel
	process.Stdout = channel
	process.Stderr = channel.Stderr()
	status := uint32(0)
	if err := process.Run(); err != nil {
		status = 1
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			status = uint32(exitError.ExitCode())
		}
	}
	f.sendExitStatus(channel, status)
}

func (f *composeSSHFixture) sendExitStatus(channel sshlib.Channel, status uint32) {
	_, _ = channel.SendRequest("exit-status", false, sshlib.Marshal(struct{ Status uint32 }{Status: status}))
}

func installComposeFixtureCommand(t *testing.T, source, target string) {
	t.Helper()
	content, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("read fixture command %s: %v", source, err)
	}
	if err := os.WriteFile(target, content, 0o700); err != nil {
		t.Fatalf("install fixture command %s: %v", target, err)
	}
}

func generateComposeFixtureSigner(t *testing.T) sshlib.Signer {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate fixture host key: %v", err)
	}
	signer, err := sshlib.NewSignerFromKey(privateKey)
	if err != nil {
		t.Fatalf("create fixture host signer: %v", err)
	}
	return signer
}

func writeComposeFixtureKey(t *testing.T, directory string) string {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate fixture client key: %v", err)
	}
	encoded, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("marshal fixture client key: %v", err)
	}
	keyPath := filepath.Join(directory, "id_rsa")
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded}), 0o600); err != nil {
		t.Fatalf("write fixture client key: %v", err)
	}
	return keyPath
}
