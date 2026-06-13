package connector

import (
	"testing"

	"github.com/stretchr/testify/require"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/commands"
	"maunium.net/go/mautrix/bridgev2/networkid"

	"go.mau.fi/mautrix-telegram/pkg/connector/ids"
)

func TestParseResyncNamesArgs(t *testing.T) {
	t.Run("empty args", func(t *testing.T) {
		filter, err := parseResyncNamesArgs(nil)
		require.NoError(t, err)
		require.False(t, filter.DryRun)
		require.Nil(t, filter.PeerType)
	})

	t.Run("dry run", func(t *testing.T) {
		filter, err := parseResyncNamesArgs([]string{"--dry-run"})
		require.NoError(t, err)
		require.True(t, filter.DryRun)
		require.Nil(t, filter.PeerType)
	})

	t.Run("channels dry run", func(t *testing.T) {
		filter, err := parseResyncNamesArgs([]string{"channels", "--dry-run"})
		require.NoError(t, err)
		require.True(t, filter.DryRun)
		require.NotNil(t, filter.PeerType)
		require.Equal(t, ids.PeerTypeChannel, *filter.PeerType)
	})

	t.Run("users", func(t *testing.T) {
		filter, err := parseResyncNamesArgs([]string{"users", "--dry-run"})
		require.NoError(t, err)
		require.True(t, filter.DryRun)
		require.NotNil(t, filter.PeerType)
		require.Equal(t, ids.PeerTypeUser, *filter.PeerType)
	})

	t.Run("conflicting peer filters", func(t *testing.T) {
		_, err := parseResyncNamesArgs([]string{"users", "channels"})
		require.Error(t, err)
	})

	t.Run("unknown arg", func(t *testing.T) {
		_, err := parseResyncNamesArgs([]string{"--force"})
		require.Error(t, err)
	})
}

func TestResyncNameFromChatInfo(t *testing.T) {
	tc := &TelegramClient{
		main: &TelegramConnector{
			Config: TelegramConfig{
				PortalNamePrefix: "[TG] ",
				PortalNameSuffix: " (tg)",
			},
		},
	}

	rawName := "Topic - Forum Group"
	name, ok := tc.resyncNameFromChatInfo(&bridgev2.ChatInfo{Name: &rawName})
	require.True(t, ok)
	require.Equal(t, "[TG] Topic - Forum Group (tg)", name)

	userName := "John Doe"
	name, ok = tc.resyncNameFromChatInfo(&bridgev2.ChatInfo{Name: &userName})
	require.True(t, ok)
	require.Equal(t, "[TG] John Doe (tg)", name)

	formattedName := "[TG] Topic - Forum Group (tg)"
	name, ok = tc.resyncNameFromChatInfo(&bridgev2.ChatInfo{Name: &formattedName})
	require.True(t, ok)
	require.Equal(t, "[TG] Topic - Forum Group (tg)", name)

	_, ok = tc.resyncNameFromChatInfo(nil)
	require.False(t, ok)
}

func TestFormatResyncNamesReply(t *testing.T) {
	result := resyncNamesResult{
		DryRun:    true,
		Scanned:   4,
		Updated:   2,
		Unchanged: 1,
		Skipped:   1,
		Failed:    0,
		ChangeExamples: []resyncNamesChangeExample{
			{
				PortalKey: networkid.PortalKey{ID: "channel:123"},
				OldName:   "Forum Group",
				NewName:   "Forum Group (tg)",
			},
		},
		IssueExamples: []resyncNamesIssueExample{
			{
				PortalKey: networkid.PortalKey{ID: "chat:456"},
				Reason:    "current Telegram chat info has no room name",
			},
		},
	}

	rendered := formatResyncNamesReply(result)
	require.Contains(t, rendered, "Dry run: no Matrix rooms were updated.")
	require.Contains(t, rendered, "* scanned: 4")
	require.Contains(t, rendered, "* would update: 2")
	require.Contains(t, rendered, "* unchanged: 1")
	require.Contains(t, rendered, "* skipped: 1")
	require.Contains(t, rendered, "* failed: 0")
	require.Contains(t, rendered, "`channel:123`: `Forum Group` -> `Forum Group (tg)`")
	require.Contains(t, rendered, "`chat:456`: current Telegram chat info has no room name")
}

func TestResyncNamesCommandRegistration(t *testing.T) {
	require.Equal(t, "resync-names", cmdResyncNames.Name)
	require.Contains(t, cmdResyncNames.Aliases, "resyncnames")
	require.Contains(t, cmdResyncNames.Aliases, "sync-names")
	require.True(t, cmdResyncNames.RequiresAdmin)
	require.NotEqual(t, commands.CommandBridge.Name, cmdResyncNames.Name)
	require.NotEqual(t, commands.CommandUnbridge.Name, cmdResyncNames.Name)
	require.NotEqual(t, cmdSyncChats.Name, cmdResyncNames.Name)
}
