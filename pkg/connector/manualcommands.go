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
	"slices"
	"strconv"
	"strings"

	"github.com/rs/zerolog"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/commands"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/format"

	"go.mau.fi/mautrix-telegram/pkg/connector/ids"
)

var cmdBridgeWithFilter = &commands.FullHandler{
	Func:                    fnBridgeWithFilter,
	Name:                    commands.CommandBridge.Name,
	Help:                    commands.CommandBridge.Help,
	RequiresEventLevel:      commands.CommandBridge.RequiresEventLevel,
	RequiresAdmin:           commands.CommandBridge.RequiresAdmin,
	RequiresPortal:          commands.CommandBridge.RequiresPortal,
	RequiresLogin:           commands.CommandBridge.RequiresLogin,
	RequiresLoginPermission: commands.CommandBridge.RequiresLoginPermission,
	NetworkAPI:              commands.CommandBridge.NetworkAPI,
	NetworkConnector:        commands.CommandBridge.NetworkConnector,
}

var cmdUnbridgeWithFilter = &commands.FullHandler{
	Func:                    fnUnbridgeWithFilter,
	Name:                    commands.CommandUnbridge.Name,
	Help:                    commands.CommandUnbridge.Help,
	RequiresEventLevel:      commands.CommandUnbridge.RequiresEventLevel,
	RequiresAdmin:           commands.CommandUnbridge.RequiresAdmin,
	RequiresPortal:          commands.CommandUnbridge.RequiresPortal,
	RequiresLogin:           commands.CommandUnbridge.RequiresLogin,
	RequiresLoginPermission: commands.CommandUnbridge.RequiresLoginPermission,
	NetworkAPI:              commands.CommandUnbridge.NetworkAPI,
	NetworkConnector:        commands.CommandUnbridge.NetworkConnector,
}

func fnBridgeWithFilter(ce *commands.Event) {
	originalArgs := slices.Clone(ce.Args)
	nonFlagArgs := bridgeCommandNonFlagArgs(originalArgs)
	if ce.Portal != nil {
		commands.CommandBridge.Run(ce)
		return
	} else if len(nonFlagArgs) == 0 || len(nonFlagArgs) > 2 {
		ce.Reply("Usage: `$cmdprefix bridge [login ID] <chat ID>`")
		return
	}

	peerType, peerID, portalID, _, err := parseManualBridgeIdentifier(nonFlagArgs[len(nonFlagArgs)-1])
	if err != nil {
		ce.Reply("Invalid Telegram chat ID: %v", err)
		return
	}

	clientForConfig := telegramClientForManualCommand(ce, nonFlagArgs)
	if clientForConfig == nil {
		ce.Reply("Could not find a Telegram login for checking manual bridge permissions.")
		return
	} else if !clientForConfig.allowPeerForManualBridge(ce.Ctx, peerType, peerID) {
		ce.Reply("%s", clientForConfig.describeManualBridgeRejection(ce.Ctx, peerType, peerID))
		return
	}

	ce.Args = replaceBridgeIdentifierArg(originalArgs, string(portalID))
	commands.CommandBridge.Run(ce)

	portal, err := ce.Bridge.GetPortalByMXID(ce.Ctx, ce.RoomID)
	if err != nil {
		ce.Log.Err(err).Msg("Failed to verify manual bridge result")
		return
	} else if portal == nil || portal.ID != portalID {
		return
	}

	client, login := telegramClientForManualOverride(ce, portal, nonFlagArgs)
	log := ce.Log.With().
		Str("peer_type", string(peerType)).
		Int64("peer_id", peerID).
		Str("portal_id", string(portalID)).
		Str("room_id", string(ce.RoomID)).
		Logger()
	if login != nil {
		log.UpdateContext(func(c zerolog.Context) zerolog.Context {
			return c.Str("login_id", string(login.ID))
		})
	}
	if client == nil {
		log.Warn().Msg("Manual bridge succeeded, but no Telegram login was found for saving allow override")
		ce.Reply("Room was bridged, but failed to save manual allow state because no Telegram login was found.")
		return
	}
	if err = client.setManualPeerAllowed(ce.Ctx, peerType, peerID); err != nil {
		log.Err(err).Msg("Manual bridge succeeded, but saving allow override failed")
		ce.Reply("Room was bridged, but failed to save manual allow state: %v", err)
		return
	}
	log.Info().Msg("Manual bridge succeeded, saved allow override")
}

func fnUnbridgeWithFilter(ce *commands.Event) {
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

	portal, verifyErr := ce.Bridge.GetPortalByMXID(ce.Ctx, roomID)
	if verifyErr != nil {
		ce.Log.Err(verifyErr).Msg("Failed to verify manual unbridge result")
		return
	} else if portal != nil && portal.ID == portalID {
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

func bridgeCommandNonFlagArgs(args []string) []string {
	nonFlagArgs := make([]string, 0, len(args))
	for _, arg := range args {
		if isStandardBridgeFlag(arg) {
			continue
		}
		nonFlagArgs = append(nonFlagArgs, arg)
	}
	return nonFlagArgs
}

func replaceBridgeIdentifierArg(args []string, normalized string) []string {
	out := slices.Clone(args)
	for i := len(out) - 1; i >= 0; i-- {
		if isStandardBridgeFlag(out[i]) {
			continue
		}
		out[i] = normalized
		break
	}
	return out
}

func isStandardBridgeFlag(arg string) bool {
	switch strings.ToLower(arg) {
	case "--overwrite", "--ignore-permissions":
		return true
	default:
		return false
	}
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
