// mautrix-telegram - A Matrix-Telegram puppeting bridge.
// Copyright (C) 2026 Tulir Asokan
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package connector

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/rs/zerolog"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/commands"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/format"

	"go.mau.fi/mautrix-telegram/pkg/connector/ids"
)

const manualBridgeUsage = "Usage: `$cmdprefix manualbridge [login ID] [--overwrite] <telegram peer ID>`"

type manualBridgeArgs struct {
	LoginID   networkid.UserLoginID
	PeerType  ids.PeerType
	PeerID    int64
	PortalID  networkid.PortalID
	Overwrite bool
}

var cmdManualBridge = &commands.FullHandler{
	Func: fnManualBridge,
	Name: "manualbridge",
	Help: commands.HelpMeta{
		Section:     commands.HelpSectionChats,
		Description: "Create or open a Telegram portal room for manual-only peer filtering",
		Args:        "[login ID] [--overwrite] <telegram peer ID>",
	},
}

var cmdManualUnbridge = &commands.FullHandler{
	Func: fnManualUnbridge,
	Name: "manualunbridge",
	Help: commands.HelpMeta{
		Section:     commands.HelpSectionChats,
		Description: "Unbridge the current portal room and deny automatic recreation for manual-only peer filtering",
	},
	RequiresPortal: true,
}

func fnManualBridge(ce *commands.Event) {
	parsed, nonFlagArgs, err := parseManualBridgeArgs(ce.Args)
	if err != nil {
		ce.Reply("%s", err)
		return
	}

	clientForConfig := telegramClientForManualCommand(ce, nonFlagArgs)
	if clientForConfig == nil {
		ce.Reply("Could not find a Telegram login for checking manual bridge permissions.")
		return
	} else if !clientForConfig.allowPeerForManualBridge(ce.Ctx, parsed.PeerType, parsed.PeerID) {
		ce.Reply("%s", clientForConfig.describeManualBridgeRejection(ce.Ctx, parsed.PeerType, parsed.PeerID))
		return
	}

	portalKey := clientForConfig.makePortalKeyFromID(parsed.PeerType, parsed.PeerID, 0)
	portal, err := ce.Bridge.GetPortalByKey(ce.Ctx, portalKey)
	if err != nil {
		ce.Log.Err(err).Stringer("portal_key", portalKey).Msg("Failed to get portal for manual bridge")
		ce.Reply("Failed to get portal record: %v", err)
		return
	}

	if portal.MXID != "" {
		if parsed.Overwrite {
			ce.Reply("Manual bridge overwrite is not safely supported yet. The bridgev2 portal API creates rooms with a deterministic room ID for each portal key, and there is no non-destructive API to detach the existing Matrix room and create a fresh replacement without risking room reuse or deletion.")
			return
		}
		if err = clientForConfig.setManualPeerAllowed(ce.Ctx, parsed.PeerType, parsed.PeerID); err != nil {
			ce.Log.Err(err).Stringer("portal_key", portalKey).Msg("Failed to save manual allow state for existing portal")
			ce.Reply("Portal already exists at [%s](%s), but failed to save manual allow state: %v", portal.MXID, portal.MXID.URI().MatrixToURL(), err)
			return
		}
		ce.Reply("Portal already exists at [%s](%s). Manual allow state is saved.", portal.MXID, portal.MXID.URI().MatrixToURL())
		return
	}

	info, err := clientForConfig.GetChatInfo(ce.Ctx, portal)
	if err != nil {
		ce.Log.Err(err).Stringer("portal_key", portalKey).Msg("Failed to get chat info for manual bridge")
		ce.Reply("Failed to get chat info: %v", err)
		return
	} else if info == nil {
		ce.Reply("Chat info not found")
		return
	}
	if err = portal.CreateMatrixRoom(ce.Ctx, clientForConfig.userLogin, info); err != nil {
		ce.Log.Err(err).Stringer("portal_key", portalKey).Msg("Failed to create portal room for manual bridge")
		ce.Reply("Failed to create portal room: %v", err)
		return
	} else if portal.MXID == "" {
		ce.Log.Error().Stringer("portal_key", portalKey).Msg("Manual bridge portal creation returned without Matrix room ID")
		ce.Reply("Failed to create portal room: no Matrix room ID was saved.")
		return
	}

	log := ce.Log.With().
		Str("peer_type", string(parsed.PeerType)).
		Int64("peer_id", parsed.PeerID).
		Str("portal_id", string(parsed.PortalID)).
		Str("room_id", string(portal.MXID)).
		Logger()
	log.UpdateContext(func(c zerolog.Context) zerolog.Context {
		return c.Str("login_id", string(clientForConfig.userLogin.ID))
	})
	if err = clientForConfig.setManualPeerAllowed(ce.Ctx, parsed.PeerType, parsed.PeerID); err != nil {
		log.Err(err).Msg("Manual bridge succeeded, but saving allow override failed")
		ce.Reply("Portal room was created at [%s](%s), but failed to save manual allow state: %v", portal.MXID, portal.MXID.URI().MatrixToURL(), err)
		return
	}
	log.Info().Msg("Manual bridge succeeded, saved allow override")
	ce.Reply("Successfully created portal room [%s](%s) and saved manual allow state.", portal.MXID, portal.MXID.URI().MatrixToURL())
}

func fnManualUnbridge(ce *commands.Event) {
	portalID := ce.Portal.ID
	roomID := ce.Portal.MXID
	peerType, peerID, _, err := ids.ParsePortalID(portalID)
	if err != nil {
		ce.Log.Err(err).Str("portal_id", string(portalID)).Msg("Failed to parse portal ID for filtered unbridge")
		ce.Reply("Failed to parse portal ID")
		return
	}

	client, login := telegramClientForManualOverride(ce, ce.Portal, nil)
	commands.CommandUnbridge.Run(ce)

	portal, verifyErr := ce.Bridge.GetExistingPortalByKey(ce.Ctx, ce.Portal.PortalKey)
	if verifyErr != nil {
		ce.Log.Err(verifyErr).Msg("Failed to verify manual unbridge result")
		return
	} else if portal != nil && portal.MXID != "" {
		return
	}

	log := ce.Log.With().
		Str("peer_type", string(peerType)).
		Int64("peer_id", peerID).
		Str("portal_id", string(portalID)).
		Str("room_id", string(roomID)).
		Logger()
	if login != nil {
		log.UpdateContext(func(c zerolog.Context) zerolog.Context {
			return c.Str("login_id", string(login.ID))
		})
	}
	if client == nil {
		log.Warn().Msg("Manual unbridge succeeded, but no Telegram login was found for saving deny override")
		ce.Reply("Room was unbridged, but failed to save manual deny state because no Telegram login was found.")
		return
	}
	if err = client.setManualPeerDenied(ce.Ctx, peerType, peerID); err != nil {
		log.Err(err).Msg("Manual unbridge succeeded, but saving deny override failed")
		ce.Reply("Room was unbridged, but failed to save manual deny state: %v", err)
		return
	}
	log.Info().Msg("Manual unbridge succeeded, saved deny override")
}

func parseManualBridgeArgs(args []string) (manualBridgeArgs, []string, error) {
	var parsed manualBridgeArgs
	nonFlagArgs := make([]string, 0, len(args))
	for _, arg := range args {
		if strings.EqualFold(arg, "--overwrite") {
			parsed.Overwrite = true
			continue
		} else if strings.HasPrefix(arg, "--") {
			return parsed, nil, fmt.Errorf("Unknown manualbridge flag %s. %s", format.SafeMarkdownCode(arg), manualBridgeUsage)
		}
		nonFlagArgs = append(nonFlagArgs, arg)
	}
	if len(nonFlagArgs) == 0 || len(nonFlagArgs) > 2 {
		return parsed, nil, fmt.Errorf(manualBridgeUsage)
	}
	if len(nonFlagArgs) == 2 {
		parsed.LoginID = networkid.UserLoginID(nonFlagArgs[0])
	}
	peerType, peerID, portalID, _, err := parseManualBridgeIdentifier(nonFlagArgs[len(nonFlagArgs)-1])
	if err != nil {
		return parsed, nil, fmt.Errorf("Invalid Telegram peer ID: %v", err)
	}
	parsed.PeerType = peerType
	parsed.PeerID = peerID
	parsed.PortalID = portalID
	return parsed, nonFlagArgs, nil
}

func telegramClientForManualCommand(ce *commands.Event, nonFlagArgs []string) *TelegramClient {
	login := loginForManualBridgeArgs(ce, nonFlagArgs)
	if login == nil {
		return nil
	}
	client, _ := login.Client.(*TelegramClient)
	return client
}

func loginForManualBridgeArgs(ce *commands.Event, nonFlagArgs []string) *bridgev2.UserLogin {
	if len(nonFlagArgs) == 2 {
		return ce.Bridge.GetCachedUserLoginByID(networkid.UserLoginID(nonFlagArgs[0]))
	}
	if ce.Bridge.Config.Relay.PreferDefault && len(ce.Bridge.Config.Relay.DefaultRelays) > 0 {
		for _, relayID := range ce.Bridge.Config.Relay.DefaultRelays {
			if login := ce.Bridge.GetCachedUserLoginByID(relayID); login != nil {
				return login
			}
		}
	}
	if login := ce.User.GetDefaultLogin(); login != nil {
		return login
	}
	if ce.Bridge.Config.Relay.AllowBridge {
		for _, relayID := range ce.Bridge.Config.Relay.DefaultRelays {
			if login := ce.Bridge.GetCachedUserLoginByID(relayID); login != nil {
				return login
			}
		}
	}
	return nil
}

func telegramClientForManualOverride(ce *commands.Event, portal *bridgev2.Portal, nonFlagArgs []string) (*TelegramClient, *bridgev2.UserLogin) {
	var login *bridgev2.UserLogin
	if len(nonFlagArgs) == 2 {
		login = ce.Bridge.GetCachedUserLoginByID(networkid.UserLoginID(nonFlagArgs[0]))
	}
	if login == nil && portal.Receiver != "" {
		login = ce.Bridge.GetCachedUserLoginByID(portal.Receiver)
	}
	if login == nil {
		login = loginForManualBridgeArgs(ce, nonFlagArgs)
	}
	if login == nil {
		var err error
		login, _, err = portal.FindPreferredLogin(ce.Ctx, ce.User, ce.Bridge.Config.Relay.AllowBridge)
		if err != nil {
			ce.Log.Debug().Err(err).Msg("Failed to find preferred login for manual override")
		}
	}
	if login == nil {
		login = portal.Relay
	}
	if login == nil {
		return nil, nil
	}
	client, _ := login.Client.(*TelegramClient)
	return client, login
}

func (tc *TelegramClient) describeManualBridgeRejection(ctx context.Context, peerType ids.PeerType, peerID int64) string {
	filter := tc.filterForPeerType(peerType)
	prettyID := format.SafeMarkdownCode(formatUserFriendlyTelegramID(peerType, peerID))
	if filter.Enabled != nil && !*filter.Enabled {
		return fmt.Sprintf("Manual bridge rejected: Telegram %s bridging is disabled by config for %s.", peerType, prettyID)
	}
	switch strings.ToLower(filter.Mode) {
	case "whitelist":
		if !matchesPeerFilter(ctx, peerType, peerID, filter.List) {
			return fmt.Sprintf("Manual bridge rejected: %s is not in the Telegram %s whitelist.", prettyID, peerType)
		}
	case "blacklist":
		if matchesPeerFilter(ctx, peerType, peerID, filter.List) {
			return fmt.Sprintf("Manual bridge rejected: %s is in the Telegram %s blacklist.", prettyID, peerType)
		}
	case "", "none":
		return "Manual bridge rejected by Telegram peer filter."
	default:
		return fmt.Sprintf("Manual bridge rejected: unknown Telegram %s filter mode %q.", peerType, filter.Mode)
	}
	return "Manual bridge rejected by Telegram peer filter."
}

func parseManualBridgeIdentifier(identifier string) (
	peerType ids.PeerType,
	peerID int64,
	portalID networkid.PortalID,
	ok bool,
	err error,
) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return "", 0, "", false, fmt.Errorf("empty identifier")
	}

	if strings.HasPrefix(identifier, "-") {
		if !strings.HasPrefix(identifier, "-100") || len(identifier) <= len("-100") {
			return "", 0, "", false, fmt.Errorf("negative IDs must use Telegram channel form -100<id>")
		}
		peerID, err = strconv.ParseInt(strings.TrimPrefix(identifier, "-100"), 10, 64)
		if err != nil || peerID <= 0 {
			return "", 0, "", false, fmt.Errorf("invalid Telegram channel ID %q", identifier)
		}
		peerType = ids.PeerTypeChannel
		return peerType, peerID, ids.MakePortalID(peerType, peerID), true, nil
	}

	if strings.Contains(identifier, ":") {
		parts := strings.Split(identifier, ":")
		if len(parts) != 2 {
			return "", 0, "", false, fmt.Errorf("expected <user|chat|channel>:<id>")
		}
		peerType = ids.PeerType(parts[0])
		switch peerType {
		case ids.PeerTypeUser, ids.PeerTypeChat, ids.PeerTypeChannel:
		default:
			return "", 0, "", false, fmt.Errorf("unknown Telegram peer type %q", parts[0])
		}
		peerID, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil || peerID <= 0 {
			return "", 0, "", false, fmt.Errorf("invalid Telegram %s ID %q", peerType, parts[1])
		}
		return peerType, peerID, ids.MakePortalID(peerType, peerID), true, nil
	}

	peerID, err = strconv.ParseInt(identifier, 10, 64)
	if err != nil || peerID <= 0 {
		return "", 0, "", false, fmt.Errorf("expected a positive user ID, -100 channel ID, or typed ID like channel:123")
	}
	peerType = ids.PeerTypeUser
	return peerType, peerID, ids.MakePortalID(peerType, peerID), true, nil
}

func formatUserFriendlyTelegramID(peerType ids.PeerType, peerID int64) string {
	if peerType == ids.PeerTypeChannel {
		return "-100" + strconv.FormatInt(peerID, 10)
	}
	return fmt.Sprintf("%s:%d", peerType, peerID)
}
