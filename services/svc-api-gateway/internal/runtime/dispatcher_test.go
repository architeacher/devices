package runtime

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("creates service context with default values", func(t *testing.T) {
		t.Parallel()

		serviceCtx := New()

		require.NotNil(t, serviceCtx)
		require.NotNil(t, serviceCtx.shutdownChannel)
		require.Nil(t, serviceCtx.deps)
		require.Nil(t, serviceCtx.serverReady)
	})

	t.Run("creates service context with options", func(t *testing.T) {
		t.Parallel()

		ch := make(chan os.Signal, 1)
		serviceCtx := New(
			WithServiceTermination(ch),
			WithWaitingForServer(),
		)

		require.NotNil(t, serviceCtx)
		require.Equal(t, ch, serviceCtx.shutdownChannel)
		require.NotNil(t, serviceCtx.serverReady)
	})
}
