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
