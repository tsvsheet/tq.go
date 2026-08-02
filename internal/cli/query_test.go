package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tq "github.com/tsvsheet/go-tq"
	tsvsheet "github.com/tsvsheet/go-tsvsheet"

	"github.com/tsvsheet/tq.go/internal/constants"
)

// Running one query end to end: where its input comes from, where its output
// goes, and what each side reports when it fails. app_test.go covers the
// command surface around this — flags, exit codes, help.

func TestRunQuery_ReadError(t *testing.T) {
	t.Parallel()

	streams := Streams{In: failReader{}, Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	err := runQuery(streams, positional{"select name"}, tq.Options{})
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrReadInput)
}

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

func TestPositional(t *testing.T) {
	t.Parallel()

	args := positional{"first", "second"}
	assert.Equal(t, sourcePath("first"), args.at(0))
	assert.Equal(t, sourcePath("second"), args.at(1))
	assert.Equal(t, sourcePath(""), args.at(2)) // missing → stdin
	assert.Equal(t, "first", args.text(0))
	assert.Equal(t, "", args.text(5)) // missing → empty
}
