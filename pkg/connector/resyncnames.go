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
	"errors"
	"fmt"
	"strings"
	"time"

	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/commands"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/format"
	"maunium.net/go/mautrix/id"

	"go.mau.fi/mautrix-telegram/pkg/connector/ids"
)

const resyncNamesExampleLimit = 10

var errNoResyncNamesLogin = errors.New("no Telegram login found to resolve current chat info")

type resyncNamesFilter struct {
	DryRun   bool
	PeerType *ids.PeerType
}

type resyncNamesChangeExample struct {
	PortalKey networkid.PortalKey
	OldName   string
	NewName   string
}

type resyncNamesIssueExample struct {
	PortalKey networkid.PortalKey
	RoomID    id.RoomID
	Reason    string
}

type resyncNamesResult struct {
	DryRun    bool
	Scanned   int
	Updated   int
	Unchanged int
	Skipped   int
	Failed    int

	ChangeExamples []resyncNamesChangeExample
	IssueExamples  []resyncNamesIssueExample
}

type resyncNamesHooks struct {
	checkRoomAccessible func(ce *commands.Event, portal *bridgev2.Portal) error
	resolveName         func(ce *commands.Event, portal *bridgev2.Portal) (string, bool, error)
	updateName          func(ce *commands.Event, portal *bridgev2.Portal, newName string) error
}

var cmdResyncNames = &commands.FullHandler{
	Func:    fnResyncNames,
	Name:    "resync-names",
	Aliases: []string{"resyncnames", "sync-names"},
	Help: commands.HelpMeta{
		Section:     commands.HelpSectionAdmin,
		Description: "Update existing Telegram portal room names from the current portal_name_prefix/suffix config",
		Args:        "[--dry-run] [users|groups|channels]",
	},
	RequiresAdmin: true,
}

func fnResyncNames(ce *commands.Event) {
	filter, err := parseResyncNamesArgs(ce.Args)
	if err != nil {
		ce.Reply("Invalid resync-names arguments: %v", err)
		return
	}

	result, err := runResyncNames(ce, filter)
	if err != nil {
		ce.Log.Err(err).Msg("Failed to load bridged portals for name resync")
		ce.Reply("Failed to load bridged portals: %v", err)
		return
	}
	ce.Reply("%s", formatResyncNamesReply(result))
}

func parseResyncNamesArgs(args []string) (resyncNamesFilter, error) {
	var filter resyncNamesFilter
	for _, arg := range args {
		switch strings.ToLower(arg) {
		case "--dry-run":
			filter.DryRun = true
		case "users", "groups", "channels":
			peerType := ids.PeerTypeUser
			if strings.EqualFold(arg, "groups") {
				peerType = ids.PeerTypeChat
			} else if strings.EqualFold(arg, "channels") {
				peerType = ids.PeerTypeChannel
			}
			if filter.PeerType != nil && *filter.PeerType != peerType {
				return filter, fmt.Errorf("only one peer type filter can be used")
			}
			filter.PeerType = &peerType
		default:
			return filter, fmt.Errorf("unknown argument %s. Usage: `$cmdprefix resync-names [--dry-run] [users|groups|channels]`", format.SafeMarkdownCode(arg))
		}
	}
	return filter, nil
}

func runResyncNames(ce *commands.Event, filter resyncNamesFilter) (resyncNamesResult, error) {
	result := resyncNamesResult{
		DryRun: filter.DryRun,
	}
	portals, err := ce.Bridge.GetAllPortalsWithMXID(ce.Ctx)
	if err != nil {
		return result, err
	}

	for _, portal := range portals {
		peerType, _, _, err := ids.ParsePortalID(portal.ID)
		if err != nil {
			result.Scanned++
			result.addSkipped(portal.PortalKey, "failed to parse Telegram portal ID")
			continue
		} else if filter.PeerType != nil && peerType != *filter.PeerType {
			continue
		}

		result.Scanned++
		resyncPortalNameWithHooks(ce, portal, filter, &result, defaultResyncNamesHooks())
	}
	return result, nil
}

func defaultResyncNamesHooks() resyncNamesHooks {
	return resyncNamesHooks{
		checkRoomAccessible: checkResyncNamesRoomAccessible,
		resolveName:         resolveResyncNamesPortalName,
		updateName:          updateResyncNamesPortalName,
	}
}

func resyncPortalNameWithHooks(ce *commands.Event, portal *bridgev2.Portal, filter resyncNamesFilter, result *resyncNamesResult, hooks resyncNamesHooks) {
	if portal.MXID == "" {
		result.addSkippedPortal(portal, "portal has no Matrix room ID")
		return
	}

	if err := hooks.checkRoomAccessible(ce, portal); err != nil {
		result.addSkippedPortal(portal, formatResyncNamesMatrixAccessReason(err))
		return
	}

	newName, ok, err := hooks.resolveName(ce, portal)
	if errors.Is(err, errNoResyncNamesLogin) {
		result.addSkippedPortal(portal, err.Error())
		return
	} else if err != nil {
		if ce != nil {
			ce.Log.Err(err).Stringer("portal_key", portal.PortalKey).Msg("Failed to get chat info for name resync")
		}
		result.addFailedPortal(portal, fmt.Sprintf("failed to get current Telegram chat info: %v", err))
		return
	} else if !ok {
		result.addSkippedPortal(portal, "current Telegram chat info has no room name")
		return
	}

	oldName := portal.Name
	needsUpdate := oldName != newName || !portal.NameSet
	if !needsUpdate {
		result.Unchanged++
		return
	}

	result.addChangeExample(portal.PortalKey, oldName, newName)
	if filter.DryRun {
		result.Updated++
		return
	}

	if err := hooks.updateName(ce, portal, newName); err != nil {
		result.addFailedPortal(portal, fmt.Sprintf("failed to update Matrix room name: %v", err))
		return
	}
	result.Updated++
}

func checkResyncNamesRoomAccessible(ce *commands.Event, portal *bridgev2.Portal) error {
	if ce == nil || ce.Bot == nil {
		return fmt.Errorf("Matrix room accessibility check is unavailable")
	}
	return ce.Bot.EnsureJoined(ce.Ctx, portal.MXID)
}

func resolveResyncNamesPortalName(ce *commands.Event, portal *bridgev2.Portal) (string, bool, error) {
	client, _ := telegramClientForManualOverride(ce, portal, nil)
	if client == nil {
		return "", false, errNoResyncNamesLogin
	}

	info, err := client.GetChatInfo(ce.Ctx, portal)
	if err != nil {
		return "", false, err
	}
	name, ok := client.resyncNameFromChatInfo(info)
	return name, ok, nil
}

func updateResyncNamesPortalName(ce *commands.Event, portal *bridgev2.Portal, newName string) error {
	if ce == nil || ce.Bot == nil {
		return fmt.Errorf("Matrix room update API is unavailable")
	}

	_, err := ce.Bot.SendState(ce.Ctx, portal.MXID, event.StateRoomName, "", &event.Content{
		Parsed: &event.RoomNameEventContent{Name: newName},
		Raw: map[string]any{
			"com.beeper.exclude_from_timeline": true,
		},
	}, time.Time{})
	if err != nil {
		return err
	}

	portal.Name = newName
	portal.NameSet = true
	portal.NameIsCustom = true
	if err = portal.Save(ce.Ctx); err != nil {
		return fmt.Errorf("sent room name event, but failed to save portal metadata: %w", err)
	}
	return nil
}

func formatResyncNamesMatrixAccessReason(err error) string {
	msg := strings.TrimSpace(err.Error())
	if msg == "" {
		return "Matrix room inaccessible or bot not joined"
	}
	return "Matrix room inaccessible or bot not joined: " + msg
}

func (tc *TelegramClient) resyncNameFromChatInfo(info *bridgev2.ChatInfo) (string, bool) {
	if info == nil || info.Name == nil || *info.Name == "" {
		return "", false
	}
	// GetChatInfo already applies portal_name_prefix/suffix. Calling formatPortalName here
	// is intentionally idempotent and keeps this helper safe if a future info path returns a raw name.
	return tc.formatPortalName(*info.Name), true
}

func (result *resyncNamesResult) addChangeExample(portalKey networkid.PortalKey, oldName, newName string) {
	if len(result.ChangeExamples) < resyncNamesExampleLimit {
		result.ChangeExamples = append(result.ChangeExamples, resyncNamesChangeExample{
			PortalKey: portalKey,
			OldName:   oldName,
			NewName:   newName,
		})
	}
}

func (result *resyncNamesResult) addSkipped(portalKey networkid.PortalKey, reason string) {
	result.Skipped++
	result.addIssueExample(portalKey, "", reason)
}

func (result *resyncNamesResult) addFailed(portalKey networkid.PortalKey, reason string) {
	result.Failed++
	result.addIssueExample(portalKey, "", reason)
}

func (result *resyncNamesResult) addSkippedPortal(portal *bridgev2.Portal, reason string) {
	result.Skipped++
	result.addIssueExample(portal.PortalKey, portal.MXID, reason)
}

func (result *resyncNamesResult) addFailedPortal(portal *bridgev2.Portal, reason string) {
	result.Failed++
	result.addIssueExample(portal.PortalKey, portal.MXID, reason)
}

func (result *resyncNamesResult) addIssueExample(portalKey networkid.PortalKey, roomID id.RoomID, reason string) {
	if len(result.IssueExamples) < resyncNamesExampleLimit {
		result.IssueExamples = append(result.IssueExamples, resyncNamesIssueExample{
			PortalKey: portalKey,
			RoomID:    roomID,
			Reason:    reason,
		})
	}
}

func formatResyncNamesReply(result resyncNamesResult) string {
	var builder strings.Builder
	builder.WriteString("Resynced Telegram portal room names:\n\n")
	if result.DryRun {
		builder.WriteString("Dry run: no Matrix rooms were updated.\n\n")
	}
	updatedLabel := "updated"
	if result.DryRun {
		updatedLabel = "would update"
	}
	fmt.Fprintf(&builder, "* scanned: %d\n", result.Scanned)
	fmt.Fprintf(&builder, "* %s: %d\n", updatedLabel, result.Updated)
	fmt.Fprintf(&builder, "* unchanged: %d\n", result.Unchanged)
	fmt.Fprintf(&builder, "* skipped: %d\n", result.Skipped)
	fmt.Fprintf(&builder, "* failed: %d", result.Failed)

	if result.DryRun && len(result.ChangeExamples) > 0 {
		builder.WriteString("\n\nPlanned changes:\n")
		for _, example := range result.ChangeExamples {
			fmt.Fprintf(
				&builder,
				"* %s: %s -> %s\n",
				format.SafeMarkdownCode(example.PortalKey.String()),
				format.SafeMarkdownCode(example.OldName),
				format.SafeMarkdownCode(example.NewName),
			)
		}
		if more := result.Updated - len(result.ChangeExamples); more > 0 {
			fmt.Fprintf(&builder, "* ...and %d more\n", more)
		}
	}

	if len(result.IssueExamples) > 0 {
		builder.WriteString("\n\nSkipped/failed examples:\n")
		for _, example := range result.IssueExamples {
			identifier := example.PortalKey.String()
			if example.RoomID != "" {
				identifier += " / " + string(example.RoomID)
			}
			fmt.Fprintf(
				&builder,
				"* %s: %s\n",
				format.SafeMarkdownCode(identifier),
				example.Reason,
			)
		}
		if more := result.Skipped + result.Failed - len(result.IssueExamples); more > 0 {
			fmt.Fprintf(&builder, "* ...and %d more\n", more)
		}
	}

	return strings.TrimSpace(builder.String())
}
