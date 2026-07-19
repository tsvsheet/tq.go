package cli

import (
	"context"
	"errors"
	"log/slog"

	golog "github.com/gomatic/go-log"
	tq "github.com/tsvsheet/go-tq"
	"github.com/urfave/cli/v3"

	"github.com/tsvsheet/tq.go/internal/constants"
)

const (
	name        = "tq"
	usage       = "Query TSV with a pipeline of relational verbs."
	description = `tq runs a query — a pipeline of relational verbs (select, drop, where,
derive, rename, sort, distinct, limit, offset, group) whose embedded
expressions are tsvsheet formulas — over a TSV or .tsvt table and writes the
resulting table as TSV to stdout.

The query is the first positional argument; the table is the second — omitted
(or "-") reads stdin. The first data row is the header unless --no-header.
Input containing =formula cells is computed first, so the query sees values
(--raw skips this pass). The table is loaded fully into memory; --max-cells
bounds what a run admits.

Exit status: 0 success; 2 query syntax error; 1 any other error (invalid
usage, unreadable input, or a program error against the data).

tq writes only the result table to stdout, so it composes in unix pipelines:
  tq 'where [stars] > 1000 | sort -stars | limit 10' repos.tsv
  cat repos.tsv | tq 'select name, stars' | tq 'where [stars] > 500'`
)

// exit codes.
const (
	exitOK          = 0
	exitError       = 1
	exitSyntaxError = 2
)

// command names.
const (
	cmdComplete = "completion"
	cmdMan      = "man"
)

// builtinCompletionName renames urfave/cli's auto-added (hidden) shell-completion
// command so it does not collide with this repo's own visible `completion`
// command. EnableShellCompletion still drives on-the-fly <TAB> completion via the
// --generate-shell-completion flag; the renamed built-in only supplies the
// per-shell script templates that the `completion` command delegates to.
const builtinCompletionName = "__completion"

// argQuery is the root command's ArgsUsage: the query is required, the table
// may be omitted to read stdin.
const argQuery = "<query> [file]"

// Version is a build version string, supplied by main (ldflags -X) and threaded
// into the command rather than held in a package-level variable.
type Version string

// loggerConfig holds the global logging flags, bound on the root command.
var loggerConfig golog.LoggerConfig

// Command builds the root tq command with the given version. The root action
// runs the query itself; `completion` is the only subcommand. A Before hook
// configures the default structured logger from the global flags so that the
// one-line exit diagnostics log consistently to stderr.
func Command(v Version) *cli.Command {
	var cfg runConfig
	return &cli.Command{
		Name:                       name,
		Usage:                      usage,
		Description:                description,
		ArgsUsage:                  argQuery,
		Version:                    string(v),
		EnableShellCompletion:      true,
		ShellCompletionCommandName: builtinCompletionName,
		Before:                     configureLogger,
		OnUsageError:               usageError,
		Flags: append(
			loggerFlags(),
			&cli.BoolFlag{
				Name:        flagNoHeader,
				Usage:       "treat every row as data: no header, positional column references only",
				Destination: &cfg.isHeaderless,
			},
			&cli.BoolFlag{
				Name:        flagStrict,
				Usage:       "abort on the first expression producing an error value (exit 1)",
				Destination: &cfg.isStrict,
			},
			&cli.BoolFlag{
				Name:        flagRaw,
				Usage:       "skip the compute-first pass: every cell is verbatim text, =formulas included",
				Destination: &cfg.isRaw,
			},
			&cli.IntFlag{
				Name:        flagMaxCells,
				Usage:       "cap on the input cells (rows × columns) admitted; 0 = unbounded",
				Destination: &cfg.maxCells,
			},
			&cli.StringFlag{
				Name:        flagAt,
				Usage:       "RFC 3339 instant that volatile functions (NOW, TODAY) read; defaults to process start",
				Destination: &cfg.atText,
			},
		),
		Commands: []*cli.Command{completionCommand(), manCommand()},
		Action: streamAction(func(s Streams, args positional) error {
			opts, err := cfg.options(now())
			if err != nil {
				return err
			}
			return runQuery(s, args, opts)
		}),
	}
}

// configureLogger installs the default structured logger from the parsed
// logging flags, so every diagnostic is a single structured stderr line.
func configureLogger(ctx context.Context, _ *cli.Command) (context.Context, error) {
	slog.SetDefault(loggerConfig.NewLogger(stderr))
	return ctx, nil
}

// usageError wraps a flag-parse failure (unknown flag, malformed value) in
// this repo's ErrUsage sentinel and suppresses urfave/cli's default handling —
// which would print the diagnostic a second time and dump the full help text
// to stdout, into any downstream pipeline. The wrapped error surfaces as the
// ordinary one-line exit-1 diagnostic instead.
func usageError(_ context.Context, _ *cli.Command, err error, _ bool) error {
	return constants.ErrUsage.With(err)
}

// loggerFlags builds the global --log-level / --log-format flags.
func loggerFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:        "log-level",
			Sources:     cli.EnvVars("TQ_LOG_LEVEL"),
			Value:       "info",
			Usage:       "Logging level (debug, info, warn, error)",
			Destination: (*string)(&loggerConfig.LogLevel),
		},
		&cli.StringFlag{
			Name:        "log-format",
			Sources:     cli.EnvVars("TQ_LOG_FORMAT"),
			Value:       "text",
			Usage:       "Log output format (text, json)",
			Destination: (*string)(&loggerConfig.LogFormat),
		},
	}
}

// Run builds and runs the CLI, returning the process exit code: 0 success,
// 2 query syntax error, 1 any other error. The default structured logger is
// installed before the command runs, so a failure that precedes flag parsing
// (a usage error, which skips Before) still diagnoses in the same one-line
// format; Before then re-installs it with the parsed logging flags.
func Run(ctx context.Context, version Version, args []string) int {
	slog.SetDefault(golog.LoggerConfig{}.NewLogger(stderr))
	err := Command(version).Run(ctx, args)
	return exitCode(err)
}

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
