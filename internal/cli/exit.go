// How a failure leaves the process.
//
// app.go builds and runs the command; this file decides what the shell sees
// when that run fails — which exit code, and the one line on stderr that
// explains it. That mapping is the CLI's contract with every script that calls
// it, and it changes for different reasons than the command surface does: a new
// flag does not move an exit code, and a new failure mode does not add a flag.
package cli

import (
	"context"
	"errors"
	"log/slog"

	tq "github.com/tsvsheet/go-tq"
	"github.com/urfave/cli/v3"

	"github.com/tsvsheet/tq.go/internal/constants"
)

// exitCode maps a run error to a process exit code with a one-line stderr
// diagnostic: a tq query syntax error (go-tq's ErrSyntax, carrying position)
// is exit 2; every other error — the program-vs-data sentinels
// (ErrUnknownColumn, ErrCellRef, ErrHeaderless, ErrStrict, ErrLimit) included
// — is exit 1.
func exitCode(err error) int {
	switch {
	case err == nil:
		return exitOK
	case errors.Is(err, tq.ErrSyntax):
		slog.Error(name, "error", err)
		return exitSyntaxError
	default:
		slog.Error(name, "error", err)
		return exitError
	}
}

// usageError wraps a flag-parse failure (unknown flag, malformed value) in
// this repo's ErrUsage sentinel and suppresses urfave/cli's default handling —
// which would print the diagnostic a second time and dump the full help text
// to stdout, into any downstream pipeline. The wrapped error surfaces as the
// ordinary one-line exit-1 diagnostic instead.
func usageError(_ context.Context, _ *cli.Command, err error, _ bool) error {
	return constants.ErrUsage.With(err)
}
