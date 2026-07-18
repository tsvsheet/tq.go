// Package constants declares tq's sentinel error values. The error mechanism
// (the matchable string type) lives in the shared gomatic/go-error library;
// these values are this package's own, and cover only the argument and flag
// handling this repo owns — every tq semantic error is a go-tq sentinel.
package constants

// Imported bare (the package is named error); this file declares only sentinels
// and uses no builtin error type, so each declaration reads errs.Const.
import errs "github.com/gomatic/go-error"

// Keep these constants sorted alphabetically.
const (
	ErrInvalidAt        errs.Const = "invalid --at time"
	ErrMissingArgument  errs.Const = "missing required argument"
	ErrOpenFile         errs.Const = "failed to open file"
	ErrUnsupportedShell errs.Const = "unsupported shell"
)
