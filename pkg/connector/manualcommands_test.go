package connector

import (
	"testing"

	"github.com/stretchr/testify/require"
	"maunium.net/go/mautrix/bridgev2/networkid"

	"go.mau.fi/mautrix-telegram/pkg/connector/ids"
)

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
