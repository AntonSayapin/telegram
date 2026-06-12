package connector

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestPeerFilterManualOnlyConfig(t *testing.T) {
	var missing TelegramConfig
	require.NoError(t, yaml.Unmarshal([]byte(`
filter:
  channels:
    enabled: true
    mode: blacklist
    list: []
`), &missing))
	require.False(t, missing.Filter.Channels.ManualOnly)

	var present TelegramConfig
	require.NoError(t, yaml.Unmarshal([]byte(`
filter:
  channels:
    enabled: true
    mode: blacklist
    manual_only: true
    list: []
`), &present))
	require.True(t, present.Filter.Channels.ManualOnly)
}
