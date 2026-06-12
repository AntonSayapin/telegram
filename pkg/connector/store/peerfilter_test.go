package store

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
	"go.mau.fi/util/dbutil"

	"go.mau.fi/mautrix-telegram/pkg/connector/ids"
)

func newPeerFilterTestStore(t *testing.T) (*ScopedStore, context.Context) {
	t.Helper()
	ctx := context.Background()
	rawDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = rawDB.Close()
	})
	db, err := dbutil.NewWithDB(rawDB, "sqlite3")
	require.NoError(t, err)
	container := NewStore(db, dbutil.NoopLogger)
	require.NoError(t, container.Upgrade(ctx))
	return container.GetScopedStore(1234), ctx
}

func TestPeerFilterOverrideStore(t *testing.T) {
	scoped, ctx := newPeerFilterTestStore(t)

	state, ok, err := scoped.GetPeerFilterOverride(ctx, ids.PeerTypeChannel, 12345)
	require.NoError(t, err)
	require.False(t, ok)
	require.Empty(t, state)

	require.NoError(t, scoped.SetPeerFilterOverride(ctx, ids.PeerTypeChannel, 12345, PeerFilterOverrideAllow))
	state, ok, err = scoped.GetPeerFilterOverride(ctx, ids.PeerTypeChannel, 12345)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, PeerFilterOverrideAllow, state)

	require.NoError(t, scoped.SetPeerFilterOverride(ctx, ids.PeerTypeChannel, 12345, PeerFilterOverrideDeny))
	state, ok, err = scoped.GetPeerFilterOverride(ctx, ids.PeerTypeChannel, 12345)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, PeerFilterOverrideDeny, state)

	require.NoError(t, scoped.ClearPeerFilterOverride(ctx, ids.PeerTypeChannel, 12345))
	state, ok, err = scoped.GetPeerFilterOverride(ctx, ids.PeerTypeChannel, 12345)
	require.NoError(t, err)
	require.False(t, ok)
	require.Empty(t, state)

	require.Error(t, scoped.SetPeerFilterOverride(ctx, ids.PeerTypeChannel, 12345, "maybe"))
}
