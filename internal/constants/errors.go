// Package constants declares tq's sentinel error values. The error mechanism
// (the matchable string type) lives in the shared gomatic/go-error library;
// these values are this package's own, and cover only the argument and flag
// handling this repo owns — every tq semantic error is a go-tq sentinel.
package constants

// The library's package is named errs; the explicit alias states that beside
// the import path (go-error), which does not name it.
import errs "github.com/gomatic/go-error"

// Keep these constants sorted alphabetically.
const (
	ErrInvalidAt        errs.Const = "invalid --at time"
	ErrManPage          errs.Const = "failed to render man page"
	ErrMissingArgument  errs.Const = "missing required argument"
	ErrOpenFile         errs.Const = "failed to open file"
	ErrTooManyArguments errs.Const = "too many arguments"
	ErrUnsupportedShell errs.Const = "unsupported shell"
	ErrUsage            errs.Const = "invalid usage"
)
