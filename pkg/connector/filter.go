package connector

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"github.com/rs/zerolog"

	"go.mau.fi/mautrix-telegram/pkg/connector/ids"
	"go.mau.fi/mautrix-telegram/pkg/gotd/tg"
)

func (tc *TelegramClient) allowPeer(ctx context.Context, peer tg.PeerClass) bool {
	peerType, peerID := peerTypeAndID(peer)
	if peerType == "" || peerID == 0 {
		zerolog.Ctx(ctx).Warn().
			Str("peer_type", string(peerType)).
			Int64("peer_id", peerID).
			Msg("Rejecting unknown Telegram peer")
		return false
	}

	filter := tc.filterForPeerType(peerType)

	if filter.Enabled != nil && !*filter.Enabled {
		zerolog.Ctx(ctx).Debug().
			Str("peer_type", string(peerType)).
			Int64("peer_id", peerID).
			Msg("Rejecting Telegram peer because peer type is disabled")
		return false
	}

	switch filter.Mode {
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
			Msg("Unknown Telegram peer filter mode, allowing peer")
		return true
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

func peerTypeAndID(peer tg.PeerClass) (ids.PeerType, int64) {
	switch p := peer.(type) {
	case *tg.PeerUser:
		return ids.PeerTypeUser, p.UserID
	case *tg.PeerChat:
		return ids.PeerTypeChat, p.ChatID
	case *tg.PeerChannel:
		return ids.PeerTypeChannel, p.ChannelID
	default:
		return "", 0
	}
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
