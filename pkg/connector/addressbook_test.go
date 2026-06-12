package connector

import (
	"testing"

	"github.com/stretchr/testify/require"

	"go.mau.fi/mautrix-telegram/pkg/connector/ids"
	"go.mau.fi/mautrix-telegram/pkg/connector/store"
)

func TestParseAddressBookArgs(t *testing.T) {
	t.Run("empty args", func(t *testing.T) {
		filter, err := parseAddressBookArgs(nil)
		require.NoError(t, err)
		require.Nil(t, filter.PeerType)
		require.Nil(t, filter.Bridged)
		require.Empty(t, filter.Search)
		require.Equal(t, 1, filter.Page)
		require.Equal(t, addressBookDefaultLimit, filter.Limit)
	})

	t.Run("channels page 2", func(t *testing.T) {
		filter, err := parseAddressBookArgs([]string{"channels", "page", "2"})
		require.NoError(t, err)
		require.NotNil(t, filter.PeerType)
		require.Equal(t, ids.PeerTypeChannel, *filter.PeerType)
		require.Equal(t, 2, filter.Page)
	})

	t.Run("search", func(t *testing.T) {
		filter, err := parseAddressBookArgs([]string{"search", "laserum"})
		require.NoError(t, err)
		require.Equal(t, "laserum", filter.Search)
		require.Equal(t, 1, filter.Page)
	})

	t.Run("channels search page", func(t *testing.T) {
		filter, err := parseAddressBookArgs([]string{"channels", "search", "laserum", "page", "2"})
		require.NoError(t, err)
		require.NotNil(t, filter.PeerType)
		require.Equal(t, ids.PeerTypeChannel, *filter.PeerType)
		require.Equal(t, "laserum", filter.Search)
		require.Equal(t, 2, filter.Page)
	})

	t.Run("status filters", func(t *testing.T) {
		filter, err := parseAddressBookArgs([]string{"unbridged", "manual", "denied"})
		require.NoError(t, err)
		require.NotNil(t, filter.Bridged)
		require.False(t, *filter.Bridged)
		require.NotNil(t, filter.ManualOnly)
		require.True(t, *filter.ManualOnly)
		require.Equal(t, store.PeerFilterOverrideDeny, filter.ManualState)
	})

	t.Run("invalid page", func(t *testing.T) {
		_, err := parseAddressBookArgs([]string{"page", "nope"})
		require.Error(t, err)
	})
}

func TestAddressBookIDFormatting(t *testing.T) {
	require.Equal(t, "-1001234567890", formatAddressBookFriendlyID(ids.PeerTypeChannel, 1234567890))
	require.Equal(t, "chat:123", formatAddressBookFriendlyID(ids.PeerTypeChat, 123))
	require.Equal(t, "user:123", formatAddressBookFriendlyID(ids.PeerTypeUser, 123))

	require.Equal(t, "channel:1234567890 / -1001234567890", formatAddressBookPortalID(ids.PeerTypeChannel, 1234567890))
	require.Equal(t, "chat:123", formatAddressBookPortalID(ids.PeerTypeChat, 123))
	require.Equal(t, "user:123", formatAddressBookPortalID(ids.PeerTypeUser, 123))
}

func TestAddressBookStatusFormatting(t *testing.T) {
	require.Equal(t, "manual_only, not bridged", formatAddressBookStatus(addressBookEntry{
		Enabled:    true,
		ManualOnly: true,
		Bridged:    false,
	}))
	require.Equal(t, "auto, bridged", formatAddressBookStatus(addressBookEntry{
		Enabled: true,
		Bridged: true,
	}))
	require.Equal(t, "disabled, not bridged", formatAddressBookStatus(addressBookEntry{
		Enabled: false,
		Bridged: false,
	}))

	entry := addressBookEntry{
		PeerType:      ids.PeerTypeChannel,
		PeerID:        1234567890,
		PortalID:      "channel:1234567890",
		FriendlyID:    "-1001234567890",
		Name:          "Laserum",
		Enabled:       true,
		ManualOnly:    true,
		ConfigAllowed: true,
		ManualState:   "none",
	}
	rendered := formatAddressBookEntry(1, entry, "!tg")
	require.Contains(t, rendered, "Manual state: none")
	require.Contains(t, rendered, "Command: `!tg bridge -1001234567890`")

	entry.ManualState = store.PeerFilterOverrideAllow
	require.Contains(t, formatAddressBookEntry(1, entry, "!tg"), "Manual state: allow")
	entry.ManualState = store.PeerFilterOverrideDeny
	require.Contains(t, formatAddressBookEntry(1, entry, "!tg"), "Manual state: deny")
}

func TestAddressBookEntryMatchesFilter(t *testing.T) {
	peerType := ids.PeerTypeChannel
	bridged := false
	manualOnly := true
	entry := addressBookEntry{
		PeerType:    ids.PeerTypeChannel,
		PortalID:    "channel:1234567890",
		FriendlyID:  "-1001234567890",
		Name:        "Laserum",
		Bridged:     false,
		ManualOnly:  true,
		ManualState: store.PeerFilterOverrideDeny,
	}
	require.True(t, addressBookEntryMatchesFilter(entry, AddressBookFilter{
		PeerType:    &peerType,
		Bridged:     &bridged,
		ManualOnly:  &manualOnly,
		ManualState: store.PeerFilterOverrideDeny,
		Search:      "laser",
		Page:        1,
		Limit:       addressBookDefaultLimit,
	}))
	require.False(t, addressBookEntryMatchesFilter(entry, AddressBookFilter{Search: "missing"}))
}
