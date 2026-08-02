package cli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tq "github.com/tsvsheet/go-tq"

	"github.com/tsvsheet/tq.go/internal/constants"
)

// How a failure leaves the process: which exit code, on which stream, and how
// much of the query's position survives into the message. A CLI's error
// behaviour is its contract with every script that calls it, so it is pinned
// apart from the command surface itself.

// TestRun_ExitCodes drives Run end to end per exit-code class, asserting both
// the code and the stderr shape: success is silent; every failure is exactly
// one diagnostic line naming the sentinel.
func TestRun_ExitCodes(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		inStderr string
		args     []string
		code     int
	}{
		{name: "success", args: []string{"select name"}, input: repos, code: exitOK},
		{
			name:     "program error",
			args:     []string{"select nope"},
			input:    repos,
			code:     exitError,
			inStderr: string(tq.ErrUnknownColumn),
		},
		{
			name:     "strict error",
			args:     []string{"--strict", "derive d = [stars] / 0"},
			input:    repos,
			code:     exitError,
			inStderr: string(tq.ErrStrict),
		},
		{
			name:     "limit error",
			args:     []string{"--max-cells", "2", "select name"},
			input:    repos,
			code:     exitError,
			inStderr: string(tq.ErrLimit),
		},
		{
			name:     "syntax error",
			args:     []string{"bogus"},
			input:    repos,
			code:     exitSyntaxError,
			inStderr: string(tq.ErrSyntax),
		},
		{
			name:     "usage error",
			args:     []string{"--frobnicate", "select name"},
			input:    repos,
			code:     exitError,
			inStderr: string(constants.ErrUsage),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withStdin(t, tc.input)
			errOut := withStderr(t)
			code := Run(context.Background(), "test", append([]string{name}, tc.args...))
			assert.Equal(t, tc.code, code)
			if tc.inStderr == "" {
				assert.Empty(t, errOut.String())
				return
			}
			assert.Equal(t, 1, strings.Count(errOut.String(), "\n"), "diagnostic must be one line: %q", errOut.String())
			assert.Contains(t, errOut.String(), tc.inStderr)
		})
	}
}

func TestExitCode(t *testing.T) {
	errOut := withStderr(t)
	_, err := configureLogger(context.Background(), Command("test"))
	require.NoError(t, err)

	assert.Equal(t, exitOK, exitCode(nil))
	assert.Equal(t, exitSyntaxError, exitCode(tq.ErrSyntax.With(nil)))
	for _, err := range []error{
		tq.ErrUnknownColumn.With(nil),
		tq.ErrCellRef.With(nil),
		tq.ErrHeaderless.With(nil),
		tq.ErrStrict.With(nil),
		tq.ErrLimit.With(nil),
		errors.New("boom"),
	} {
		assert.Equal(t, exitError, exitCode(err))
	}
	assert.NotEmpty(t, errOut.String())
}

// TestRun_SyntaxErrorCarriesPosition asserts the exit-2 diagnostic names the
// failing position (R3: syntax error with position).
func TestRun_SyntaxErrorCarriesPosition(t *testing.T) {
	withStdin(t, repos)
	errOut := withStderr(t)
	code := Run(context.Background(), "test", []string{name, "select name |\nbogus"})
	assert.Equal(t, exitSyntaxError, code)
	assert.Contains(t, errOut.String(), "line")
}

// TestCLI_ProgramErrors asserts each program-vs-data failure surfaces as its
// go-tq sentinel (R3's exit-1 class) and each argument failure as this repo's
// own sentinel.
func TestCLI_ProgramErrors(t *testing.T) {
	cases := []struct {
		want  error
		name  string
		input string
		args  []string
	}{
		{name: "unknown column", args: []string{"select nope"}, input: repos, want: tq.ErrUnknownColumn},
		{name: "cell reference", args: []string{"where A1 > 1"}, input: repos, want: tq.ErrCellRef},
		{
			name:  "rename headerless",
			args:  []string{"--no-header", "rename [1] as x"},
			input: "a\tb\n",
			want:  tq.ErrHeaderless,
		},
		{
			name:  "strict error value",
			args:  []string{"--strict", "derive d = [stars] / 0"},
			input: repos,
			want:  tq.ErrStrict,
		},
		{
			name:  "input over max-cells",
			args:  []string{"--max-cells", "2", "select name"},
			input: repos,
			want:  tq.ErrLimit,
		},
		{name: "syntax error", args: []string{"bogus"}, input: repos, want: tq.ErrSyntax},
		{name: "missing query", args: []string{}, input: repos, want: constants.ErrMissingArgument},
		{
			name:  "extra positional argument",
			args:  []string{"select name", "repos.tsv", "surplus"},
			input: repos,
			want:  constants.ErrTooManyArguments,
		},
		{
			name:  "unknown flag",
			args:  []string{"--frobnicate", "select name"},
			input: repos,
			want:  constants.ErrUsage,
		},
		{
			name:  "invalid at",
			args:  []string{"--at", "not-a-time", "select name"},
			input: repos,
			want:  constants.ErrInvalidAt,
		},
		{
			name:  "unopenable file",
			args:  []string{"select name", "no/such/file.tsv"},
			input: repos,
			want:  constants.ErrOpenFile,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withStdin(t, tc.input)
			_, err := runCLI(t, tc.args...)
			require.Error(t, err)
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

// TestCLI_UsageErrorKeepsStdoutClean asserts a flag-parse failure writes
// nothing to stdout — no help dump into a downstream pipeline — and surfaces
// as ErrUsage wrapping urfave/cli's diagnostic.
func TestCLI_UsageErrorKeepsStdoutClean(t *testing.T) {
	withStdin(t, repos)
	out, err := runCLI(t, "--frobnicate", "select name")
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrUsage)
	assert.Empty(t, out)
}
