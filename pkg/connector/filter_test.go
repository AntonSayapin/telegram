package connector

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
	"go.mau.fi/util/dbutil"

	"go.mau.fi/mautrix-telegram/pkg/connector/ids"
	"go.mau.fi/mautrix-telegram/pkg/connector/store"
	"go.mau.fi/mautrix-telegram/pkg/gotd/tg"
)

func testTelegramClient(t *testing.T, filter PeerFilterConfig) (*TelegramClient, context.Context) {
	t.Helper()
	ctx := context.Background()
	rawDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = rawDB.Close()
	})
	db, err := dbutil.NewWithDB(rawDB, "sqlite3")
	require.NoError(t, err)
	container := store.NewStore(db, dbutil.NoopLogger)
	require.NoError(t, container.Upgrade(ctx))
	tc := &TelegramClient{
		main: &TelegramConnector{
			Config: TelegramConfig{
				Filter: FilterConfig{
					Channels: filter,
				},
			},
		},
		ScopedStore: container.GetScopedStore(1234),
	}
	require.NoError(t, tc.main.Config.PostProcess())
	return tc, ctx
}

func TestPeerFilterDecisions(t *testing.T) {
	truePtr := boolPtr(true)
	falsePtr := boolPtr(false)
	tests := []struct {
		name       string
		filter     PeerFilterConfig
		setupState string
		wantAuto   bool
		wantManual bool
	}{
		{
			name:       "enabled false rejects automatic and manual",
			filter:     PeerFilterConfig{Enabled: falsePtr, Mode: "blacklist"},
			wantAuto:   false,
			wantManual: false,
		},
		{
			name:       "blacklist empty manual false allows automatic and manual",
			filter:     PeerFilterConfig{Enabled: truePtr, Mode: "blacklist"},
			wantAuto:   true,
			wantManual: true,
		},
		{
			name:       "blacklist empty manual only rejects automatic and allows manual",
			filter:     PeerFilterConfig{Enabled: truePtr, Mode: "blacklist", ManualOnly: true},
			wantAuto:   false,
			wantManual: true,
		},
		{
			name:       "whitelist missing manual only rejects automatic and manual",
			filter:     PeerFilterConfig{Enabled: truePtr, Mode: "whitelist", ManualOnly: true},
			wantAuto:   false,
			wantManual: false,
		},
		{
			name:       "whitelist listed manual only rejects automatic and allows manual",
			filter:     PeerFilterConfig{Enabled: truePtr, Mode: "whitelist", ManualOnly: true, List: []string{"-10012345"}},
			wantAuto:   false,
			wantManual: true,
		},
		{
			name:       "manual allow permits automatic with manual only",
			filter:     PeerFilterConfig{Enabled: truePtr, Mode: "blacklist", ManualOnly: true},
			setupState: store.PeerFilterOverrideAllow,
			wantAuto:   true,
			wantManual: true,
		},
		{
			name:       "manual deny rejects automatic even when config allows",
			filter:     PeerFilterConfig{Enabled: truePtr, Mode: "blacklist"},
			setupState: store.PeerFilterOverrideDeny,
			wantAuto:   false,
			wantManual: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc, ctx := testTelegramClient(t, tt.filter)
			if tt.setupState != "" {
				require.NoError(t, tc.ScopedStore.SetPeerFilterOverride(ctx, ids.PeerTypeChannel, 12345, tt.setupState))
			}
			require.Equal(t, tt.wantAuto, tc.allowPeerForAutomatic(ctx, &tg.PeerChannel{ChannelID: 12345}))
			require.Equal(t, tt.wantManual, tc.allowPeerForManualBridge(ctx, ids.PeerTypeChannel, 12345))
		})
	}
}
