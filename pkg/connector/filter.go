package connector

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"github.com/rs/zerolog"

	"maunium.net/go/mautrix/bridgev2/networkid"

	"go.mau.fi/mautrix-telegram/pkg/connector/ids"
	"go.mau.fi/mautrix-telegram/pkg/connector/store"
	"go.mau.fi/mautrix-telegram/pkg/gotd/tg"
)

func (tc *TelegramClient) allowPeer(ctx context.Context, peer tg.PeerClass) bool {
	return tc.allowPeerForAutomatic(ctx, peer)
}

func (tc *TelegramClient) allowPeerForAutomatic(ctx context.Context, peer tg.PeerClass) bool {
	peerType, peerID, ok := tc.peerTypeAndID(peer)
	if !ok {
		zerolog.Ctx(ctx).Warn().
			Str("peer_type", string(peerType)).
			Int64("peer_id", peerID).
			Msg("Rejecting unknown Telegram peer")
		return false
	}

	return tc.allowPeerForAutomaticByID(ctx, peerType, peerID)
}

func (tc *TelegramClient) allowPeerForAutomaticByID(ctx context.Context, peerType ids.PeerType, peerID int64) bool {
	if !tc.allowPeerByConfig(ctx, peerType, peerID) {
		return false
	}

	if state, ok := tc.getManualPeerState(ctx, peerType, peerID); ok {
		switch state {
		case store.PeerFilterOverrideDeny:
			zerolog.Ctx(ctx).Debug().
				Str("peer_type", string(peerType)).
				Int64("peer_id", peerID).
				Str("manual_state", state).
				Msg("Rejecting Telegram peer because manual override denies automatic handling")
			return false
		case store.PeerFilterOverrideAllow:
			return true
		}
	}

	if tc.isManualOnlyPeer(peerType) {
		zerolog.Ctx(ctx).Debug().
			Str("peer_type", string(peerType)).
			Int64("peer_id", peerID).
			Bool("manual_only", true).
			Msg("Rejecting Telegram peer for automatic handling because manual_only is enabled")
		return false
	}

	return true
}

func (tc *TelegramClient) allowPortalKeyForAutomatic(ctx context.Context, portalKey networkid.PortalKey) bool {
	peerType, peerID, _, err := ids.ParsePortalID(portalKey.ID)
	if err != nil {
		zerolog.Ctx(ctx).Warn().
			Err(err).
			Str("portal_id", string(portalKey.ID)).
			Msg("Rejecting Telegram portal because portal ID could not be parsed")
		return false
	}
	return tc.allowPeerForAutomaticByID(ctx, peerType, peerID)
}

func (tc *TelegramClient) allowPeerForManualBridge(ctx context.Context, peerType ids.PeerType, peerID int64) bool {
	return tc.allowPeerByConfig(ctx, peerType, peerID)
}

func (tc *TelegramClient) shouldLoadMediaForPeer(
	ctx context.Context,
	peerType ids.PeerType,
	peerID int64,
) bool {
	_ = ctx
	_ = peerID
	filter := tc.filterForPeerType(peerType)
	return filter.LoadMedia == nil || *filter.LoadMedia
}

func (tc *TelegramClient) shouldLoadMediaForTelegramPeer(ctx context.Context, peer tg.PeerClass) bool {
	peerType, peerID, ok := tc.peerTypeAndID(peer)
	if !ok {
		zerolog.Ctx(ctx).Debug().
			Str("peer_type", string(peerType)).
			Int64("peer_id", peerID).
			Msg("Allowing Telegram media load because peer type could not be parsed")
		return true
	}
	return tc.shouldLoadMediaForPeer(ctx, peerType, peerID)
}

func (tc *TelegramClient) allowPeerByConfig(
	ctx context.Context,
	peerType ids.PeerType,
	peerID int64,
) bool {
	filter := tc.filterForPeerType(peerType)

	if filter.Enabled != nil && !*filter.Enabled {
		zerolog.Ctx(ctx).Debug().
			Str("peer_type", string(peerType)).
			Int64("peer_id", peerID).
			Msg("Rejecting Telegram peer because peer type is disabled")
		return false
	}

	switch strings.ToLower(filter.Mode) {
	case "whitelist":
		allowed := matchesPeerFilter(ctx, peerType, peerID, filter.List)
		if !allowed {
			zerolog.Ctx(ctx).Debug().
				Str("peer_type", string(peerType)).
				Int64("peer_id", peerID).
				Msg("Rejecting Telegram peer because it is not in whitelist")
		}
		return allowed

	case "blacklist":
		blocked := matchesPeerFilter(ctx, peerType, peerID, filter.List)
		if blocked {
			zerolog.Ctx(ctx).Debug().
				Str("peer_type", string(peerType)).
				Int64("peer_id", peerID).
				Msg("Rejecting Telegram peer because it is in blacklist")
		}
		return !blocked

	case "", "none":
		return true

	default:
		zerolog.Ctx(ctx).Warn().
			Str("mode", filter.Mode).
			Str("peer_type", string(peerType)).
			Int64("peer_id", peerID).
			Msg("Unknown Telegram peer filter mode, rejecting peer")
		return false
	}
}

func (tc *TelegramClient) filterForPeerType(peerType ids.PeerType) PeerFilterConfig {
	switch peerType {
	case ids.PeerTypeUser:
		return tc.main.Config.Filter.Users
	case ids.PeerTypeChat:
		return tc.main.Config.Filter.Groups
	case ids.PeerTypeChannel:
		return tc.main.Config.Filter.Channels
	default:
		return PeerFilterConfig{
			Enabled: boolPtr(false),
			Mode:    "blacklist",
			List:    nil,
		}
	}
}

func (tc *TelegramClient) peerTypeAndID(peer tg.PeerClass) (ids.PeerType, int64, bool) {
	switch p := peer.(type) {
	case *tg.PeerUser:
		return ids.PeerTypeUser, p.UserID, p.UserID != 0
	case *tg.PeerChat:
		return ids.PeerTypeChat, p.ChatID, p.ChatID != 0
	case *tg.PeerChannel:
		return ids.PeerTypeChannel, p.ChannelID, p.ChannelID != 0
	default:
		return "", 0, false
	}
}

func (tc *TelegramClient) isManualOnlyPeer(peerType ids.PeerType) bool {
	return tc.filterForPeerType(peerType).ManualOnly
}

func (tc *TelegramClient) getManualPeerState(
	ctx context.Context,
	peerType ids.PeerType,
	peerID int64,
) (state string, ok bool) {
	if tc.ScopedStore == nil {
		return "", false
	}
	state, ok, err := tc.ScopedStore.GetPeerFilterOverride(ctx, peerType, peerID)
	if err != nil {
		zerolog.Ctx(ctx).Warn().
			Err(err).
			Str("peer_type", string(peerType)).
			Int64("peer_id", peerID).
			Msg("Failed to load Telegram peer filter override, rejecting automatic handling")
		return store.PeerFilterOverrideDeny, true
	}
	return state, ok
}

func (tc *TelegramClient) setManualPeerAllowed(ctx context.Context, peerType ids.PeerType, peerID int64) error {
	return tc.ScopedStore.SetPeerFilterOverride(ctx, peerType, peerID, store.PeerFilterOverrideAllow)
}

func (tc *TelegramClient) setManualPeerDenied(ctx context.Context, peerType ids.PeerType, peerID int64) error {
	return tc.ScopedStore.SetPeerFilterOverride(ctx, peerType, peerID, store.PeerFilterOverrideDeny)
}

func matchesPeerFilter(ctx context.Context, peerType ids.PeerType, peerID int64, list []string) bool {
	idPlain := strconv.FormatInt(peerID, 10)
	idWithMinus100 := "-100" + idPlain
	idWithType := string(peerType) + ":" + idPlain

	for _, item := range list {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}

		if item == idPlain || item == idWithMinus100 || item == idWithType {
			return true
		}

		if strings.HasPrefix(item, "regex:") {
			pattern := strings.TrimPrefix(item, "regex:")

			if regexMatches(ctx, pattern, idPlain, item) {
				return true
			}
			if regexMatches(ctx, pattern, idWithMinus100, item) {
				return true
			}
			if regexMatches(ctx, pattern, idWithType, item) {
				return true
			}
		}
	}

	return false
}

func regexMatches(ctx context.Context, pattern, value, originalItem string) bool {
	ok, err := regexp.MatchString("^("+pattern+")$", value)
	if err != nil {
		zerolog.Ctx(ctx).Warn().
			Err(err).
			Str("pattern", originalItem).
			Msg("Invalid Telegram peer filter regex")
		return false
	}
	return ok
}
