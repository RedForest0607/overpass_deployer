package localstore

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGeneratedCancellationClosesPipeAndRemovesTemporaryBlob(t *testing.T) {
	root := t.TempDir()
	store, _ := New(filepath.Join(root, "blobs"), root)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, key, err := store.PutGeneratedContext(ctx, func(ctx context.Context, w io.Writer) error {
			if _, err := w.Write([]byte("partial")); err != nil {
				return err
			}
			close(started)
			<-ctx.Done()
			return ctx.Err()
		}, 100)
		if key != "" {
			done <- errors.New("cancelled package committed")
			return
		}
		done <- err
	}()
	<-started
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error=%v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled generation did not finish")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("temporary files remain: %v", entries)
	}
}

func TestPreCancelledGenerationDoesNotInvokeWriter(t *testing.T) {
	root := t.TempDir()
	store, _ := New(filepath.Join(root, "blobs"), root)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := store.PutGeneratedContext(ctx, func(context.Context, io.Writer) error {
		t.Fatal("writer called after cancellation")
		return nil
	}, 100)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
}
