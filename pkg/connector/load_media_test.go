package connector

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"go.mau.fi/mautrix-telegram/pkg/connector/ids"
	"go.mau.fi/mautrix-telegram/pkg/gotd/tg"
)

func TestShouldLoadMediaForPeer(t *testing.T) {
	truePtr := boolPtr(true)
	falsePtr := boolPtr(false)
	ctx := context.Background()
	tc := &TelegramClient{
		main: &TelegramConnector{
			Config: TelegramConfig{
				Filter: FilterConfig{
					Users:    PeerFilterConfig{LoadMedia: truePtr},
					Groups:   PeerFilterConfig{LoadMedia: falsePtr},
					Channels: PeerFilterConfig{LoadMedia: falsePtr},
				},
			},
		},
	}

	require.True(t, tc.shouldLoadMediaForPeer(ctx, ids.PeerTypeUser, 1))
	require.False(t, tc.shouldLoadMediaForPeer(ctx, ids.PeerTypeChat, 2))
	require.False(t, tc.shouldLoadMediaForPeer(ctx, ids.PeerTypeChannel, 3))
	require.True(t, tc.shouldLoadMediaForPeer(ctx, ids.PeerType("unknown"), 4))

	missing := &TelegramClient{main: &TelegramConnector{}}
	require.True(t, missing.shouldLoadMediaForPeer(ctx, ids.PeerTypeChannel, 5))
}

func TestShouldLoadMediaForTelegramPeer(t *testing.T) {
	falsePtr := boolPtr(false)
	ctx := context.Background()
	tc := &TelegramClient{
		main: &TelegramConnector{
			Config: TelegramConfig{
				Filter: FilterConfig{
					Users:    PeerFilterConfig{},
					Groups:   PeerFilterConfig{LoadMedia: falsePtr},
					Channels: PeerFilterConfig{LoadMedia: falsePtr},
				},
			},
		},
	}

	require.True(t, tc.shouldLoadMediaForTelegramPeer(ctx, &tg.PeerUser{UserID: 1}))
	require.False(t, tc.shouldLoadMediaForTelegramPeer(ctx, &tg.PeerChat{ChatID: 2}))
	require.False(t, tc.shouldLoadMediaForTelegramPeer(ctx, &tg.PeerChannel{ChannelID: 3}))
	require.True(t, tc.shouldLoadMediaForTelegramPeer(ctx, nil))
}

func TestFormatMediaPlaceholder(t *testing.T) {
	require.Equal(t, "[media: 1]", formatMediaPlaceholder(1, ""))
	require.Equal(t, "[media: 1] caption", formatMediaPlaceholder(1, "caption"))
	require.Equal(t, "[media: 1]", formatMediaPlaceholder(1, " \n\t "))
	require.Equal(t, "[media: 3]", formatMediaPlaceholder(3, ""))
	require.Equal(t, "[media: 3] caption", formatMediaPlaceholder(3, " caption "))
	require.Equal(t, "[media: 1] caption", formatMediaPlaceholder(0, "caption"))
}
