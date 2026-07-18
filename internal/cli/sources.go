// Package cli is the tq command tier: it wires the engine (the
// github.com/tsvsheet/go-tq library) to urfave/cli commands with strict unix
// stdin/stdout discipline. The query is the first positional argument and the
// table the second; an omitted table (or "-") is read from stdin. Command
// logic lives in stream-injected functions so it is fully testable; the
// cli.Command wrappers only bind flags and streams.
package cli

import (
	"io"
	"os"

	"github.com/tsvsheet/tq.go/internal/constants"
)

// Streams are a command's injected I/O: input, output, and diagnostics. Real
// runs wire os.Stdin/Stdout/Stderr; tests wire buffers.
type Streams struct {
	In  io.Reader
	Out io.Writer
	Err io.Writer
}

// sourcePath is a positional table path. Empty or "-" means stdin.
type sourcePath string

// stdinMarker is the conventional stdin path.
const stdinMarker = "-"

// isStdin reports whether the path selects standard input.
func (p sourcePath) isStdin() bool { return p == "" || p == stdinMarker }

// closeFunc releases an opened source; it is a no-op for stdin.
type closeFunc func() error

// open returns a reader for the path, using stdin when the path selects it.
func (p sourcePath) open(stdin io.Reader) (io.Reader, closeFunc, error) {
	if p.isStdin() {
		return stdin, noClose, nil
	}
	file, err := os.Open(string(p))
	if err != nil {
		return nil, nil, constants.ErrOpenFile.With(err)
	}
	return file, file.Close, nil
}

// noClose is the release for a source that must not be closed (stdin).
func noClose() error { return nil }
