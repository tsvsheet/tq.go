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

func TestRun_Version(t *testing.T) {
	assert.Equal(t, exitOK, Run(context.Background(), "1.2.3", []string{name, "--version"}))
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

// failWriter always errors, exercising the write-failure passthrough.
type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

// writeTemp writes content to a file under t.TempDir and returns its path.
func writeTemp(t *testing.T, base, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := dir + "/" + base
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}
