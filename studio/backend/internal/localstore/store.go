package localstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var ErrTooLarge = errors.New("blob exceeds size limit")

type Store struct{ root, temporary string }
type Staged struct {
	Path, SHA256 string
	Size         int64
}

func (s *Store) CleanupUnreferenced(referenced map[string]struct{}, olderThan time.Time) (int, error) {
	removed := 0
	err := filepath.WalkDir(s.root, func(filePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(s.root, filePath)
		if err != nil {
			return err
		}
		key := filepath.ToSlash(relative)
		base := filepath.Base(filePath)
		parent := filepath.Base(filepath.Dir(filePath))
		if len(base) != 64 || parent != base[:2] {
			return nil
		}
		if _, err := hex.DecodeString(base); err != nil {
			return nil
		}
		if _, exists := referenced[key]; exists {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.ModTime().Before(olderThan) {
			return nil
		}
		if err := os.Remove(filePath); err != nil {
			return err
		}
		removed++
		return nil
	})
	return removed, err
}

func (s *Store) CleanupTemporary(olderThan time.Time) (int, error) {
	entries, err := os.ReadDir(s.temporary)
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "upload-") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return removed, err
		}
		if !info.ModTime().Before(olderThan) {
			continue
		}
		if err := os.Remove(filepath.Join(s.temporary, entry.Name())); err != nil {
			return removed, err
		}
		removed++
	}
	return removed, nil
}

func New(root, temporary string) (*Store, error) {
	if root == "" || temporary == "" {
		return nil, errors.New("blob root and temporary directory are required")
	}
	return &Store{root: root, temporary: temporary}, nil
}

func (s *Store) Stage(reader io.Reader, limit int64) (Staged, error) {
	return s.StageContext(context.Background(), reader, limit)
}

func (s *Store) StageContext(ctx context.Context, reader io.Reader, limit int64) (Staged, error) {
	if err := ctx.Err(); err != nil {
		return Staged{}, err
	}
	file, err := os.CreateTemp(s.temporary, "upload-*")
	if err != nil {
		return Staged{}, fmt.Errorf("create temporary blob: %w", err)
	}
	name := file.Name()
	ok := false
	defer func() {
		_ = file.Close()
		if !ok {
			_ = os.Remove(name)
		}
	}()
	hash := sha256.New()
	written, err := io.Copy(ContextWriter{Context: ctx, Writer: io.MultiWriter(file, hash)}, io.LimitReader(contextReader{ctx: ctx, reader: reader}, limit+1))
	if err != nil {
		if contextErr := ctx.Err(); contextErr != nil {
			return Staged{}, contextErr
		}
		return Staged{}, fmt.Errorf("write temporary blob: %w", err)
	}
	if written > limit {
		return Staged{}, ErrTooLarge
	}
	if err := ctx.Err(); err != nil {
		return Staged{}, err
	}
	if err := file.Sync(); err != nil {
		return Staged{}, fmt.Errorf("sync temporary blob: %w", err)
	}
	if err := file.Close(); err != nil {
		return Staged{}, fmt.Errorf("close temporary blob: %w", err)
	}
	ok = true
	return Staged{Path: name, SHA256: hex.EncodeToString(hash.Sum(nil)), Size: written}, nil
}

func (s *Store) Commit(staged Staged) (string, error) {
	if len(staged.SHA256) != 64 {
		return "", errors.New("invalid staged checksum")
	}
	key := filepath.Join(staged.SHA256[:2], staged.SHA256)
	target := filepath.Join(s.root, key)
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return "", fmt.Errorf("create blob directory: %w", err)
	}
	if _, err := os.Stat(target); err == nil {
		_ = os.Remove(staged.Path)
		return filepath.ToSlash(key), nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err := os.Rename(staged.Path, target); err != nil {
		return "", fmt.Errorf("commit blob: %w", err)
	}
	if err := os.Chmod(target, 0o600); err != nil {
		return "", fmt.Errorf("secure blob: %w", err)
	}
	return filepath.ToSlash(key), nil
}

func (s *Store) Abort(staged Staged) {
	if staged.Path != "" {
		_ = os.Remove(staged.Path)
	}
}

func (s *Store) Open(key string) (*os.File, error) {
	clean := filepath.Clean(filepath.FromSlash(key))
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return nil, errors.New("unsafe blob key")
	}
	return os.Open(filepath.Join(s.root, clean))
}

func (s *Store) PutGenerated(write func(io.Writer) error, limit int64) (Staged, string, error) {
	return s.PutGeneratedContext(context.Background(), func(_ context.Context, w io.Writer) error { return write(w) }, limit)
}
