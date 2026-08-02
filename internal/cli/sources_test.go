package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tsvsheet/tq.go/internal/constants"
)

// TestNoCloseLeavesStdinOpenForWhoeverOwnsIt pins why stdin gets a no-op
// release rather than the file's Close. The process does not own stdin — the
// shell does — and closing it would break the next reader in a pipeline and any
// later read in the same process, a failure that surfaces far from its cause.
// Every source is released the same way by the caller, so the difference has to
// live in the release it is handed.
func TestNoCloseLeavesStdinOpenForWhoeverOwnsIt(t *testing.T) {
	t.Parallel()
	stdin := strings.NewReader("first\nsecond\n")

	reader, release, err := sourcePath("-").open(stdin)
	require.NoError(t, err)
	require.NoError(t, release(), "releasing stdin reports no error")

	rest, err := io.ReadAll(reader)

	require.NoError(t, err, "and the stream is still readable afterwards")
	assert.Equal(t, "first\nsecond\n", string(rest))
}

// TestOpenReleasesAFileButNotStdin pins the other half: a file the process did
// open is a file the process must close, or a long-running command leaks
// descriptors until it cannot open another.
func TestOpenReleasesAFileButNotStdin(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "in.tsv")
	require.NoError(t, os.WriteFile(path, []byte("a\tb\n"), 0o600))

	reader, release, err := sourcePath(path).open(strings.NewReader(""))
	require.NoError(t, err)
	require.NotNil(t, reader)
	require.NoError(t, release())

	assert.Error(t, release(), "the file is genuinely closed, so a second release fails")
}

// TestOpenReportsAMissingFileAsItsOwnSentinel pins the error a caller matches.
func TestOpenReportsAMissingFileAsItsOwnSentinel(t *testing.T) {
	t.Parallel()
	_, _, err := sourcePath(filepath.Join(t.TempDir(), "nope.tsv")).open(strings.NewReader(""))

	assert.ErrorIs(t, err, constants.ErrOpenFile)
}
