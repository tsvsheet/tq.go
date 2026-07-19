package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tq "github.com/tsvsheet/go-tq"
	tsvsheet "github.com/tsvsheet/go-tsvsheet"

	"github.com/tsvsheet/tq.go/internal/constants"
)

// repos is the shared header-mode fixture.
const repos = "name\tstars\tforks\tlang\n" +
	"alpha\t1500\t3\tgo\n" +
	"beta\t900\t2\tgo\n" +
	"gamma\t2000\t8\trust\n" +
	"delta\t900\t2\tgo\n"

// withStdin swaps the package stdin for the duration of a test.
func withStdin(t *testing.T, in string) {
	t.Helper()
	prev := stdin
	stdin = strings.NewReader(in)
	t.Cleanup(func() { stdin = prev })
}

// withStderr swaps the package stderr for a buffer for the duration of a test
// and returns it, so diagnostics can be asserted.
func withStderr(t *testing.T) *bytes.Buffer {
	t.Helper()
	prev := stderr
	buf := &bytes.Buffer{}
	stderr = buf
	t.Cleanup(func() { stderr = prev })
	return buf
}

// withNow fixes the process-start clock for the duration of a test.
func withNow(t *testing.T, at time.Time) {
	t.Helper()
	prev := now
	now = func() time.Time { return at }
	t.Cleanup(func() { now = prev })
}

// runCLI runs the root command with args, capturing stdout.
func runCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := Command("test")
	var out bytes.Buffer
	cmd.Writer = &out
	err := cmd.Run(context.Background(), append([]string{name}, args...))
	return out.String(), err
}

func TestCommand_Shape(t *testing.T) {
	cmd := Command("v1")
	assert.Equal(t, name, cmd.Name)
	assert.Equal(t, "v1", cmd.Version)

	names := make([]string, len(cmd.Commands))
	for i, c := range cmd.Commands {
		names[i] = c.Name
	}
	assert.ElementsMatch(t, []string{cmdComplete, cmdMan}, names)
}

// TestCLI_Golden drives the cli layer over (input, query, expected stdout)
// triples: R1 file/stdin selection, R2 flag mapping, and TSV-out discipline.
func TestCLI_Golden(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
		args  []string
	}{
		{
			name:  "select projects columns",
			args:  []string{"select name, stars"},
			input: repos,
			want:  "name\tstars\nalpha\t1500\nbeta\t900\ngamma\t2000\ndelta\t900\n",
		},
		{
			name:  "pipeline where sort limit",
			args:  []string{"where [stars] > 500 | sort -stars | limit 2"},
			input: repos,
			want:  "name\tstars\tforks\tlang\ngamma\t2000\t8\trust\nalpha\t1500\t3\tgo\n",
		},
		{
			name:  "no-header addresses columns by position and emits no header",
			args:  []string{"--no-header", "select [2], [1]"},
			input: "alpha\t1500\nbeta\t900\n",
			want:  "1500\talpha\n900\tbeta\n",
		},
		{
			name:  "formula input computes first",
			args:  []string{"select b"},
			input: "a\tb\n1\t=A2*2\n",
			want:  "b\n2\n",
		},
		{
			name:  "raw skips the compute pass",
			args:  []string{"--raw", "select b"},
			input: "a\tb\n1\t=A2*2\n",
			want:  "b\n=A2*2\n",
		},
		{
			name:  "at pins volatile functions",
			args:  []string{"--at", "2001-02-03T04:05:06Z", "derive y = year(now()) | select y | limit 1"},
			input: repos,
			want:  "y\n2001\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withStdin(t, tc.input)
			out, err := runCLI(t, tc.args...)
			require.NoError(t, err)
			assert.Equal(t, tc.want, out)
		})
	}
}

func TestCLI_ReadsFileArgument(t *testing.T) {
	path := writeTemp(t, "repos.tsv", repos)
	out, err := runCLI(t, "select name", path)
	require.NoError(t, err)
	assert.Equal(t, "name\nalpha\nbeta\ngamma\ndelta\n", out)
}

func TestCLI_DashReadsStdin(t *testing.T) {
	withStdin(t, repos)
	out, err := runCLI(t, "select name", "-")
	require.NoError(t, err)
	assert.Equal(t, "name\nalpha\nbeta\ngamma\ndelta\n", out)
}

// TestCLI_Composition proves output re-parses as input to a second run with
// the header intact (the acceptance composition triple).
func TestCLI_Composition(t *testing.T) {
	withStdin(t, repos)
	first, err := runCLI(t, "select name, stars")
	require.NoError(t, err)

	withStdin(t, first)
	second, err := runCLI(t, "where [stars] > 1000")
	require.NoError(t, err)
	assert.Equal(t, "name\tstars\nalpha\t1500\ngamma\t2000\n", second)
}

func TestCLI_DefaultAtIsProcessStart(t *testing.T) {
	withNow(t, time.Date(1999, 12, 31, 23, 0, 0, 0, time.UTC))
	withStdin(t, repos)
	out, err := runCLI(t, "derive y = year(now()) | select y | limit 1")
	require.NoError(t, err)
	assert.Equal(t, "y\n1999\n", out)
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

// TestRun_SyntaxErrorCarriesPosition asserts the exit-2 diagnostic names the
// failing position (R3: syntax error with position).
func TestRun_SyntaxErrorCarriesPosition(t *testing.T) {
	withStdin(t, repos)
	errOut := withStderr(t)
	code := Run(context.Background(), "test", []string{name, "select name |\nbogus"})
	assert.Equal(t, exitSyntaxError, code)
	assert.Contains(t, errOut.String(), "line")
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

func TestRun_Version(t *testing.T) {
	assert.Equal(t, exitOK, Run(context.Background(), "1.2.3", []string{name, "--version"}))
}

func TestPositional(t *testing.T) {
	t.Parallel()

	args := positional{"first", "second"}
	assert.Equal(t, sourcePath("first"), args.at(0))
	assert.Equal(t, sourcePath("second"), args.at(1))
	assert.Equal(t, sourcePath(""), args.at(2)) // missing → stdin
	assert.Equal(t, "first", args.text(0))
	assert.Equal(t, "", args.text(5)) // missing → empty
}

func TestRunConfig_Options(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)

	opts, err := runConfig{maxCells: 9, isHeaderless: true, isStrict: true, isRaw: true}.options(start)
	require.NoError(t, err)
	assert.Equal(t, tq.Options{At: start, MaxCells: 9, IsHeaderless: true, IsStrict: true, IsRaw: true}, opts)

	opts, err = runConfig{atText: "2001-02-03T04:05:06Z"}.options(start)
	require.NoError(t, err)
	assert.Equal(t, time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC), opts.At)

	_, err = runConfig{atText: "yesterday"}.options(start)
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrInvalidAt)
}

func TestConfigureLogger(t *testing.T) {
	withStderr(t)
	_, err := configureLogger(context.Background(), Command("test"))
	require.NoError(t, err)
}

// failReader always errors, exercising the read-failure passthrough.
type failReader struct{}

func (failReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

func TestRunQuery_ReadError(t *testing.T) {
	t.Parallel()

	streams := Streams{In: failReader{}, Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	err := runQuery(streams, positional{"select name"}, tq.Options{})
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrReadInput)
}

// failWriter always errors, exercising the write-failure passthrough.
type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestRunQuery_WriteError(t *testing.T) {
	t.Parallel()

	streams := Streams{In: strings.NewReader(repos), Out: failWriter{}, Err: &bytes.Buffer{}}
	err := runQuery(streams, positional{"select name"}, tq.Options{})
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrWriteFile)
}

func TestSourcePath_Open(t *testing.T) {
	t.Parallel()

	path := writeTemp(t, "t.tsv", "a\n1\n")
	reader, release, err := sourcePath(path).open(strings.NewReader(""))
	require.NoError(t, err)
	assert.NotNil(t, reader)
	require.NoError(t, release())

	in := strings.NewReader("x")
	reader, release, err = sourcePath("").open(in)
	require.NoError(t, err)
	assert.Equal(t, in, reader)
	require.NoError(t, release()) // noClose

	_, _, err = sourcePath("no/such/file.tsv").open(in)
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrOpenFile)
}

// writeTemp writes content to a file under t.TempDir and returns its path.
func writeTemp(t *testing.T, base, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := dir + "/" + base
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}
