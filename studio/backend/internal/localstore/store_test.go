package localstore

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStageCommitAndOpen(t *testing.T) {
	root := t.TempDir()
	store, err := New(filepath.Join(root, "blobs"), root)
	if err != nil {
		t.Fatal(err)
	}
	staged, err := store.Stage(bytes.NewBufferString("content"), 10)
	if err != nil {
		t.Fatal(err)
	}
	key, err := store.Commit(staged)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := store.Open(key)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(reader)
	_ = reader.Close()
	if string(body) != "content" {
		t.Fatalf("body=%q", body)
	}
	if _, err := store.Open("../secret"); err == nil {
		t.Fatal("unsafe key accepted")
	}
}

func TestCleanupOnlyRemovesOldUnreferencedBlobsAndTemporaryFiles(t *testing.T) {
	root := t.TempDir()
	store, err := New(filepath.Join(root, "blobs"), filepath.Join(root, "temporary"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "temporary"), 0o700); err != nil {
		t.Fatal(err)
	}
	staged, err := store.Stage(bytes.NewBufferString("orphan"), 100)
	if err != nil {
		t.Fatal(err)
	}
	key, err := store.Commit(staged)
	if err != nil {
		t.Fatal(err)
	}
	blobPath := filepath.Join(root, "blobs", filepath.FromSlash(key))
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(blobPath, old, old); err != nil {
		t.Fatal(err)
	}
	if removed, err := store.CleanupUnreferenced(map[string]struct{}{key: {}}, time.Now().Add(-24*time.Hour)); err != nil || removed != 0 {
		t.Fatalf("referenced cleanup=%d err=%v", removed, err)
	}
	if removed, err := store.CleanupUnreferenced(map[string]struct{}{}, time.Now().Add(-24*time.Hour)); err != nil || removed != 1 {
		t.Fatalf("orphan cleanup=%d err=%v", removed, err)
	}
	if _, err := os.Stat(blobPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("orphan still exists: %v", err)
	}

	temporaryPath := filepath.Join(root, "temporary", "upload-stale")
	if err := os.WriteFile(temporaryPath, []byte("partial"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(temporaryPath, old, old); err != nil {
		t.Fatal(err)
	}
	if removed, err := store.CleanupTemporary(time.Now().Add(-24 * time.Hour)); err != nil || removed != 1 {
		t.Fatalf("temporary cleanup=%d err=%v", removed, err)
	}
}

func TestStageRejectsOversizeAndGeneratedWriterDoesNotHang(t *testing.T) {
	root := t.TempDir()
	store, _ := New(filepath.Join(root, "blobs"), root)
	if _, err := store.Stage(bytes.NewBufferString("12345"), 4); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("error=%v", err)
	}
	_, _, err := store.PutGenerated(func(w io.Writer) error {
		_, err := io.CopyN(w, bytes.NewReader(make([]byte, 1024)), 1024)
		return err
	}, 10)
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("generated error=%v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "upload-") {
			t.Fatalf("temporary upload was not removed: %s", entry.Name())
		}
	}
}
