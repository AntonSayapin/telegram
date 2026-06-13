package connector

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/commands"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/id"

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
				RoomID:    "!stale:example.com",
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
	require.Contains(t, rendered, "`chat:456 / !stale:example.com`: current Telegram chat info has no room name")
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

func TestResyncPortalNameSkipsInaccessibleRoomBeforeUpdate(t *testing.T) {
	portal := makeResyncNamesTestPortal("Old name")
	var resolved, updated bool
	result := resyncNamesResult{}

	resyncPortalNameWithHooks(nil, portal, resyncNamesFilter{}, &result, resyncNamesHooks{
		checkRoomAccessible: func(_ *commands.Event, _ *bridgev2.Portal) error {
			return errors.New("failed to ensure joined: M_UNKNOWN: cannot join remote room")
		},
		resolveName: func(_ *commands.Event, _ *bridgev2.Portal) (string, bool, error) {
			resolved = true
			return "New name", true, nil
		},
		updateName: func(_ *commands.Event, _ *bridgev2.Portal, _ string) error {
			updated = true
			return nil
		},
	})

	require.False(t, resolved)
	require.False(t, updated)
	require.Equal(t, 0, result.Updated)
	require.Equal(t, 1, result.Skipped)
	require.Equal(t, 0, result.Failed)
	require.Len(t, result.IssueExamples, 1)
	require.Equal(t, portal.MXID, result.IssueExamples[0].RoomID)
	require.Contains(t, result.IssueExamples[0].Reason, "Matrix room inaccessible or bot not joined")
	require.Contains(t, result.IssueExamples[0].Reason, "cannot join remote room")
}

func TestResyncPortalNameDryRunDoesNotUpdate(t *testing.T) {
	portal := makeResyncNamesTestPortal("Old name")
	var updated bool
	result := resyncNamesResult{DryRun: true}

	resyncPortalNameWithHooks(nil, portal, resyncNamesFilter{DryRun: true}, &result, resyncNamesHooks{
		checkRoomAccessible: func(_ *commands.Event, _ *bridgev2.Portal) error {
			return nil
		},
		resolveName: func(_ *commands.Event, _ *bridgev2.Portal) (string, bool, error) {
			return "New name", true, nil
		},
		updateName: func(_ *commands.Event, _ *bridgev2.Portal, _ string) error {
			updated = true
			return nil
		},
	})

	require.False(t, updated)
	require.Equal(t, 1, result.Updated)
	require.Equal(t, 0, result.Skipped)
	require.Equal(t, 0, result.Failed)
	require.Len(t, result.ChangeExamples, 1)
	require.Equal(t, "Old name", result.ChangeExamples[0].OldName)
	require.Equal(t, "New name", result.ChangeExamples[0].NewName)
}

func TestResyncPortalNameAccessibleRoomUpdates(t *testing.T) {
	portal := makeResyncNamesTestPortal("Old name")
	var updated bool
	result := resyncNamesResult{}

	resyncPortalNameWithHooks(nil, portal, resyncNamesFilter{}, &result, resyncNamesHooks{
		checkRoomAccessible: func(_ *commands.Event, _ *bridgev2.Portal) error {
			return nil
		},
		resolveName: func(_ *commands.Event, _ *bridgev2.Portal) (string, bool, error) {
			return "New name", true, nil
		},
		updateName: func(_ *commands.Event, portal *bridgev2.Portal, newName string) error {
			updated = true
			portal.Name = newName
			portal.NameSet = true
			return nil
		},
	})

	require.True(t, updated)
	require.Equal(t, 1, result.Updated)
	require.Equal(t, 0, result.Skipped)
	require.Equal(t, 0, result.Failed)
	require.Equal(t, "New name", portal.Name)
}

func makeResyncNamesTestPortal(name string) *bridgev2.Portal {
	return &bridgev2.Portal{
		Portal: &database.Portal{
			PortalKey: networkid.PortalKey{ID: "channel:123"},
			MXID:      id.RoomID("!room:example.com"),
			Name:      name,
			NameSet:   true,
		},
	}
}
