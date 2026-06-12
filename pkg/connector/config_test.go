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

func TestPeerFilterLoadMediaConfig(t *testing.T) {
	var missing TelegramConfig
	require.NoError(t, yaml.Unmarshal([]byte(`
filter:
  users:
    enabled: true
    mode: blacklist
    list: []
`), &missing))
	require.NotNil(t, missing.Filter.Users.LoadMedia)
	require.True(t, *missing.Filter.Users.LoadMedia)

	var loadMediaTrue TelegramConfig
	require.NoError(t, yaml.Unmarshal([]byte(`
filter:
  users:
    enabled: true
    mode: blacklist
    load_media: true
    list: []
`), &loadMediaTrue))
	require.NotNil(t, loadMediaTrue.Filter.Users.LoadMedia)
	require.True(t, *loadMediaTrue.Filter.Users.LoadMedia)

	var loadMediaFalse TelegramConfig
	require.NoError(t, yaml.Unmarshal([]byte(`
filter:
  channels:
    enabled: true
    mode: blacklist
    load_media: false
    list: []
`), &loadMediaFalse))
	require.NotNil(t, loadMediaFalse.Filter.Channels.LoadMedia)
	require.False(t, *loadMediaFalse.Filter.Channels.LoadMedia)
}

func TestPortalNameConfigDefaults(t *testing.T) {
	var cfg TelegramConfig
	require.NoError(t, yaml.Unmarshal([]byte(`
filter:
  channels:
    enabled: true
    mode: blacklist
    list: []
`), &cfg))
	require.Empty(t, cfg.PortalNamePrefix)
	require.Empty(t, cfg.PortalNameSuffix)
}
