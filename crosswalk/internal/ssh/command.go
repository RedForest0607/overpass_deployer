package ssh

import (
	"context"

	cryptossh "golang.org/x/crypto/ssh"
)

type commandSession interface {
	Run(string) error
	Signal(cryptossh.Signal) error
	Close() error
}

// Cancellation ends the local wait and requests remote termination. It does
// not prove a remote mutation was rolled back; callers must inspect before retry.
func runCommandContext(ctx context.Context, session commandSession, command string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- session.Run(command) }()
	select {
	case err := <-done:
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	case <-ctx.Done():
		// The transport owner closes the connection; do not wait for a server
		// that ignores TERM or a stalled Signal reply.
		go func() { _ = session.Signal(cryptossh.SIGTERM); _ = session.Close() }()
		return ctx.Err()
	}
}
