package connector

import (
	"testing"

	"github.com/stretchr/testify/require"
	"maunium.net/go/mautrix/bridgev2/commands"
	"maunium.net/go/mautrix/bridgev2/networkid"

	"go.mau.fi/mautrix-telegram/pkg/connector/ids"
)

func TestManualCommandsDoNotOverrideStandardBridgeCommands(t *testing.T) {
	require.Equal(t, "manualbridge", cmdManualBridge.Name)
	require.Equal(t, "manualunbridge", cmdManualUnbridge.Name)
	require.NotEqual(t, commands.CommandBridge.Name, cmdManualBridge.Name)
	require.NotEqual(t, commands.CommandUnbridge.Name, cmdManualUnbridge.Name)
}

func TestParseManualBridgeArgs(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		wantLoginID   networkid.UserLoginID
		wantPeerType  ids.PeerType
		wantPeerID    int64
		wantOverwrite bool
		wantNonFlags  []string
		wantErr       bool
	}{
		{
			name:         "channel",
			args:         []string{"-1001234567890"},
			wantPeerType: ids.PeerTypeChannel,
			wantPeerID:   1234567890,
			wantNonFlags: []string{"-1001234567890"},
		},
		{
			name:          "overwrite before id",
			args:          []string{"--overwrite", "-1001234567890"},
			wantPeerType:  ids.PeerTypeChannel,
			wantPeerID:    1234567890,
			wantOverwrite: true,
			wantNonFlags:  []string{"-1001234567890"},
		},
		{
			name:          "overwrite after id",
			args:          []string{"-1001234567890", "--overwrite"},
			wantPeerType:  ids.PeerTypeChannel,
			wantPeerID:    1234567890,
			wantOverwrite: true,
			wantNonFlags:  []string{"-1001234567890"},
		},
		{
			name:          "login id",
			args:          []string{"telegram:123", "chat:987"},
			wantLoginID:   "telegram:123",
			wantPeerType:  ids.PeerTypeChat,
			wantPeerID:    987,
			wantNonFlags:  []string{"telegram:123", "chat:987"},
			wantOverwrite: false,
		},
		{
			name:    "unknown flag",
			args:    []string{"--delete-old-room", "-1001234567890"},
			wantErr: true,
		},
		{
			name:    "missing id",
			args:    []string{"--overwrite"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, nonFlags, err := parseManualBridgeArgs(tt.args)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantLoginID, got.LoginID)
			require.Equal(t, tt.wantPeerType, got.PeerType)
			require.Equal(t, tt.wantPeerID, got.PeerID)
			require.Equal(t, ids.MakePortalID(tt.wantPeerType, tt.wantPeerID), got.PortalID)
			require.Equal(t, tt.wantOverwrite, got.Overwrite)
			require.Equal(t, tt.wantNonFlags, nonFlags)
		})
	}
}

func TestManualBridgeOverwriteDoesNotBypassStaticFilter(t *testing.T) {
	truePtr := boolPtr(true)
	tc, ctx := testTelegramClient(t, PeerFilterConfig{
		Enabled:    truePtr,
		Mode:       "whitelist",
		ManualOnly: true,
		List:       []string{"-100999"},
	})

	parsed, _, err := parseManualBridgeArgs([]string{"--overwrite", "-1001234567890"})
	require.NoError(t, err)
	require.True(t, parsed.Overwrite)
	require.False(t, tc.allowPeerForManualBridge(ctx, parsed.PeerType, parsed.PeerID))
}

func TestParseManualBridgeIdentifier(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantPeerType ids.PeerType
		wantPeerID   int64
		wantPortalID networkid.PortalID
		wantErr      bool
	}{
		{
			name:         "negative telegram channel id",
			input:        "-1001234567890",
			wantPeerType: ids.PeerTypeChannel,
			wantPeerID:   1234567890,
			wantPortalID: "channel:1234567890",
		},
		{
			name:         "typed channel id",
			input:        "channel:1234567890",
			wantPeerType: ids.PeerTypeChannel,
			wantPeerID:   1234567890,
			wantPortalID: "channel:1234567890",
		},
		{
			name:         "typed chat id",
			input:        "chat:123456789",
			wantPeerType: ids.PeerTypeChat,
			wantPeerID:   123456789,
			wantPortalID: "chat:123456789",
		},
		{
			name:         "typed user id",
			input:        "user:123456789",
			wantPeerType: ids.PeerTypeUser,
			wantPeerID:   123456789,
			wantPortalID: "user:123456789",
		},
		{
			name:         "plain positive id is user",
			input:        "123456789",
			wantPeerType: ids.PeerTypeUser,
			wantPeerID:   123456789,
			wantPortalID: "user:123456789",
		},
		{
			name:    "invalid id",
			input:   "channel:not-a-number",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			peerType, peerID, portalID, ok, err := parseManualBridgeIdentifier(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				require.False(t, ok)
				return
			}
			require.NoError(t, err)
			require.True(t, ok)
			require.Equal(t, tt.wantPeerType, peerType)
			require.Equal(t, tt.wantPeerID, peerID)
			require.Equal(t, tt.wantPortalID, portalID)
		})
	}
}
