package constants_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tsvsheet/tq.go/internal/constants"
)

// TestSentinelsDistinctAndMatchable pins the sentinel contract: every value is
// non-empty, unique (errors.Is must never cross-match), and matchable through
// a With-wrapped cause.
func TestSentinelsDistinctAndMatchable(t *testing.T) {
	t.Parallel()

	sentinels := []error{
		constants.ErrInvalidAt,
		constants.ErrMissingArgument,
		constants.ErrOpenFile,
		constants.ErrTooManyArguments,
		constants.ErrUnsupportedShell,
		constants.ErrUsage,
	}
	seen := map[string]bool{}
	for _, s := range sentinels {
		assert.NotEmpty(t, s.Error())
		assert.False(t, seen[s.Error()], "duplicate sentinel text %q", s.Error())
		seen[s.Error()] = true
	}
	assert.ErrorIs(t, constants.ErrInvalidAt.With(nil, "at", "x"), constants.ErrInvalidAt)
	assert.NotErrorIs(t, constants.ErrInvalidAt.With(nil), constants.ErrOpenFile)
}
