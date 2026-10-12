package ssh

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	cryptossh "golang.org/x/crypto/ssh"
)

type blockedCommand struct {
	started, closed chan struct{}
	once            sync.Once
}

func (s *blockedCommand) Run(string) error              { close(s.started); <-s.closed; return nil }
func (s *blockedCommand) Signal(cryptossh.Signal) error { return nil }
func (s *blockedCommand) Close() error                  { s.once.Do(func() { close(s.closed) }); return nil }

func TestCommandCancellationStopsBlockedWait(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	session := &blockedCommand{started: make(chan struct{}), closed: make(chan struct{})}
	defer session.Close()
	done := make(chan error, 1)
	go func() { done <- runCommandContext(ctx, session, "blocked") }()
	<-session.started
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error=%v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled command wait did not end")
	}
}

func TestPreCancelledCommandDoesNotRun(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	session := &blockedCommand{started: make(chan struct{}), closed: make(chan struct{})}
	if err := runCommandContext(ctx, session, "must not run"); !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
	select {
	case <-session.started:
		t.Fatal("command started after cancellation")
	default:
	}
}
