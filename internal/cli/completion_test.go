package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tsvsheet/tq.go/internal/constants"
)

// TestCLI_CompletionScripts proves R4: `tq completion <shell>` emits a
// non-empty script for each supported shell, naming the command.
func TestCLI_CompletionScripts(t *testing.T) {
	for _, shell := range supportedShells {
		t.Run(string(shell), func(t *testing.T) {
			out, err := runCLI(t, cmdComplete, string(shell))
			require.NoError(t, err)
			assert.NotEmpty(t, out)
			assert.Contains(t, out, name)
		})
	}
}

func TestCLI_CompletionMissingShell(t *testing.T) {
	_, err := runCLI(t, cmdComplete)
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrMissingArgument)
}

func TestCLI_CompletionUnsupportedShell(t *testing.T) {
	_, err := runCLI(t, cmdComplete, "powershell")
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrUnsupportedShell)
}

func TestShellName_Supported(t *testing.T) {
	t.Parallel()

	assert.True(t, shellName("bash").supported())
	assert.True(t, shellName("zsh").supported())
	assert.True(t, shellName("fish").supported())
	assert.False(t, shellName("powershell").supported())
}

func TestSupportedShellList(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "bash, zsh, fish", supportedShellList())
}

// TestCompletionRendererResolvesEverySupportedShell pins the coupling between
// the shell list and the subcommands EnableShellCompletion installs. The two
// are declared in different places, and completionRenderer dereferences what it
// finds — so a shell added to the list and forgotten in the framework wiring
// would not misbehave, it would panic, and only for the user of that shell.
// Driving the real command is the point: the built-in exists only once the
// framework has run, which is exactly the coupling under test.
func TestCompletionRendererResolvesEverySupportedShell(t *testing.T) {
	for _, shell := range supportedShells {
		out, err := runCLI(t, cmdComplete, string(shell))

		require.NoError(t, err, "%s is in supportedShells, so a renderer must exist for it", shell)
		assert.NotEmpty(t, out)
	}

	_, err := runCLI(t, cmdComplete, "nonesuch")

	assert.ErrorIs(t, err, constants.ErrUnsupportedShell, "and an unlisted shell is refused, not dereferenced")
}
