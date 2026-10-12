package localstore

import (
	"context"
	"io"
)

// ContextWriter checks cancellation between writes. An underlying blocked I/O
// must additionally be closed by its owner (as PutGeneratedContext does).
type ContextWriter struct {
	Context context.Context
	Writer  io.Writer
}

func (w ContextWriter) Write(body []byte) (int, error) {
	if err := w.Context.Err(); err != nil {
		return 0, err
	}
	return w.Writer.Write(body)
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(body []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(body)
}

// PutGeneratedContext owns the pipe lifecycle. The callback must honor ctx
// during work outside pipe writes, especially when reading input artifacts.
func (s *Store) PutGeneratedContext(ctx context.Context, write func(context.Context, io.Writer) error, limit int64) (Staged, string, error) {
	if err := ctx.Err(); err != nil {
		return Staged{}, "", err
	}
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	stop := context.AfterFunc(ctx, func() {
		_ = reader.CloseWithError(ctx.Err())
		_ = writer.CloseWithError(ctx.Err())
	})
	defer stop()
	done := make(chan error, 1)
	go func() {
		err := write(ctx, ContextWriter{Context: ctx, Writer: writer})
		_ = writer.CloseWithError(err)
		done <- err
	}()
	staged, err := s.StageContext(ctx, reader, limit)
	if err != nil {
		_ = reader.CloseWithError(err)
	}
	writeErr := <-done
	if err != nil {
		return Staged{}, "", err
	}
	defer s.Abort(staged)
	if err := ctx.Err(); err != nil {
		return Staged{}, "", err
	}
	if writeErr != nil {
		return Staged{}, "", writeErr
	}
	key, err := s.Commit(staged)
	return staged, key, err
}
