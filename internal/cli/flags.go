package cli

import (
	"context"
	"io"
	"os"
	"time"

	tq "github.com/tsvsheet/go-tq"
	"github.com/urfave/cli/v3"

	"github.com/tsvsheet/tq.go/internal/constants"
)

// stdin is indirected so tests can substitute an input stream.
var stdin io.Reader = os.Stdin

// stderr is indirected so tests can capture diagnostics.
var stderr io.Writer = os.Stderr

// now is indirected so tests can fix the default --at instant (process start).
var now = time.Now

// The run-configuration flag names, each mapping 1:1 onto a tq.Options field.
const (
	flagNoHeader = "no-header"
	flagStrict   = "strict"
	flagRaw      = "raw"
	flagMaxCells = "max-cells"
	flagAt       = "at"
)

// positional is a command's positional arguments. The query and the table
// source are positional — never flags — so invocations read as
// `tq 'select name' data.tsv`.
type positional []string

// at returns the i-th positional argument as a source path, or "" (meaning
// stdin) when the argument is absent.
func (p positional) at(i int) sourcePath {
	if i < len(p) {
		return sourcePath(p[i])
	}
	return ""
}

// text returns the i-th positional argument verbatim, or "" when absent.
func (p positional) text(i int) string {
	if i < len(p) {
		return p[i]
	}
	return ""
}

// streamAction adapts a positional-args + stream-injected function to a cli
// Action, wiring stdout from the command writer and stderr from the indirected
// stream, and the positional arguments from the parsed command line.
func streamAction(fn func(Streams, positional) error) cli.ActionFunc {
	return func(_ context.Context, c *cli.Command) error {
		streams := Streams{In: stdin, Out: c.Root().Writer, Err: stderr}
		return fn(streams, positional(c.Args().Slice()))
	}
}

// runConfig collects the run-configuration flag values the root command binds;
// options maps them onto the engine's per-run tq.Options.
type runConfig struct {
	atText       string
	maxCells     int
	isHeaderless bool
	isStrict     bool
	isRaw        bool
}

// options maps the bound flag values onto the engine's per-run tq.Options.
// start is the default --at instant (process start); an explicit --at must be
// RFC 3339 or the run fails with ErrInvalidAt.
func (c runConfig) options(start time.Time) (tq.Options, error) {
	at := start
	if c.atText != "" {
		parsed, err := time.Parse(time.RFC3339, c.atText)
		if err != nil {
			return tq.Options{}, constants.ErrInvalidAt.With(err, "at", c.atText)
		}
		at = parsed
	}
	return tq.Options{
		At:           at,
		MaxCells:     c.maxCells,
		IsHeaderless: c.isHeaderless,
		IsStrict:     c.isStrict,
		IsRaw:        c.isRaw,
	}, nil
}
