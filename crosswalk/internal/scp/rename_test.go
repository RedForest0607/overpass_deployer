package scp

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkg/sftp"
)

type testRenamer struct {
	supported       bool
	posix, standard bool
	err             error
}

type pipeConnection struct {
	*io.PipeReader
	*io.PipeWriter
}

func (p pipeConnection) Close() error {
	return errors.Join(p.PipeReader.Close(), p.PipeWriter.Close())
}

// Exercise actual SFTP requests, rather than a fake that silently overwrites
// existing destinations for every kind of rename.
func TestAtomicUploadOverSFTPReplacesExistingFile(t *testing.T) {
	serverInput, clientOutput := io.Pipe()
	clientInput, serverOutput := io.Pipe()
	connection := pipeConnection{serverInput, serverOutput}
	server, err := sftp.NewServer(connection)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	go func() { _ = server.Serve() }()
	client, err := sftp.NewClientPipe(clientInput, clientOutput)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	defer clientInput.Close()
	target := filepath.Join(t.TempDir(), "app.jar")
	if err := os.WriteFile(target, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := uploadFileAtomically(sftpFileSystem{client: client}, strings.NewReader("new"), target, io.Discard); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(target)
	if err != nil || string(content) != "new" {
		t.Fatalf("replacement failed: %q, %v", content, err)
	}
	files, err := os.ReadDir(filepath.Dir(target))
	if err != nil || len(files) != 1 {
		t.Fatalf("temporary files remain: %v, %v", files, err)
	}
}

func (r *testRenamer) HasExtension(string) (string, bool) { return "1", r.supported }
func (r *testRenamer) PosixRename(string, string) error   { r.posix = true; return r.err }
func (r *testRenamer) Rename(string, string) error        { r.standard = true; return r.err }

func TestReplaceRemoteFileSelectsAtomicExtension(t *testing.T) {
	for _, supported := range []bool{true, false} {
		for _, failure := range []bool{true, false} {
			r := &testRenamer{supported: supported}
			if failure {
				r.err = errors.New("rename failed")
			}
			err := replaceRemoteFile(r, "/app.tmp", "/app.jar")
			if !errors.Is(err, r.err) || r.posix != supported || r.standard == supported {
				t.Fatalf("incorrect rename or unsafe fallback: %#v, %v", r, err)
			}
		}
	}
}
