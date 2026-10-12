package crosswalkvm

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"
)

func TestZIPCancellationClosesBlockedArtifactReader(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reader, writer := io.Pipe()
	defer writer.Close()
	opened := make(chan struct{})
	done := make(chan error, 1)
	bundle := Bundle{Files: []PackageFile{{Path: "app.jar", Artifact: Artifact{ID: "jar", Size: 1, Open: func() (io.ReadCloser, error) { close(opened); return reader, nil }}}}}
	go func() { done <- WriteZIPContext(ctx, io.Discard, bundle) }()
	<-opened
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error=%v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("ZIP reader did not stop on cancellation")
	}
}
