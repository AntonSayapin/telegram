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
	"sort"
	"strconv"
	"strings"

	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/commands"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/format"

	"go.mau.fi/mautrix-telegram/pkg/connector/ids"
	"go.mau.fi/mautrix-telegram/pkg/connector/store"
)

const addressBookDefaultLimit = 20

type AddressBookFilter struct {
	PeerType    *ids.PeerType
	Bridged     *bool
	ManualState string
	ManualOnly  *bool
	Search      string
	Page        int
	Limit       int
}

type addressBookEntry struct {
	PeerType      ids.PeerType
	PeerID        int64
	PortalID      networkid.PortalID
	FriendlyID    string
	Name          string
	Bridged       bool
	Enabled       bool
	ManualOnly    bool
	ConfigAllowed bool
	ManualState   string
}

var cmdAddressBook = &commands.FullHandler{
	Func: fnAddressBook,
	Name: "addressbook",
	Help: commands.HelpMeta{
		Section:     commands.HelpSectionChats,
		Description: "List known Telegram peers discovered by dialog sync.",
		Args:        "[users|groups|channels|bridged|unbridged|allowed|denied|manual] [page N] [search query]",
	},
	RequiresLogin: true,
}

func fnAddressBook(ce *commands.Event) {
	filter, err := parseAddressBookArgs(ce.Args)
	if err != nil {
		ce.Reply("Invalid addressbook arguments: %v", err)
		return
	}
	login := ce.User.GetDefaultLogin()
	if login == nil {
		ce.Reply("Could not find a Telegram login for reading the address book.")
		return
	}
	client, ok := login.Client.(*TelegramClient)
	if !ok {
		ce.Reply("Could not find a Telegram login for reading the address book.")
		return
	}

	entries, err := client.listAddressBookEntries(ce.Ctx, filter)
	if err != nil {
		ce.Log.Err(err).Msg("Failed to load Telegram address book")
		ce.Reply("Failed to load Telegram address book: %v", err)
		return
	}
	ce.Reply("%s", formatAddressBookReply(filter, entries, ce.Bridge.Config.CommandPrefix))
}

func parseAddressBookArgs(args []string) (AddressBookFilter, error) {
	filter := AddressBookFilter{
		Page:  1,
		Limit: addressBookDefaultLimit,
	}
	for i := 0; i < len(args); i++ {
		arg := strings.ToLower(args[i])
		switch arg {
		case "users", "groups", "channels":
			peerType := ids.PeerTypeUser
			if arg == "groups" {
				peerType = ids.PeerTypeChat
			} else if arg == "channels" {
				peerType = ids.PeerTypeChannel
			}
			if filter.PeerType != nil && *filter.PeerType != peerType {
				return filter, fmt.Errorf("only one peer type filter can be used")
			}
			filter.PeerType = &peerType
		case "bridged", "unbridged":
			bridged := arg == "bridged"
			if filter.Bridged != nil && *filter.Bridged != bridged {
				return filter, fmt.Errorf("bridged and unbridged filters cannot be combined")
			}
			filter.Bridged = &bridged
		case "allowed", "denied":
			state := store.PeerFilterOverrideAllow
			if arg == "denied" {
				state = store.PeerFilterOverrideDeny
			}
			if filter.ManualState != "" && filter.ManualState != state {
				return filter, fmt.Errorf("allowed and denied filters cannot be combined")
			}
			filter.ManualState = state
		case "manual":
			manualOnly := true
			filter.ManualOnly = &manualOnly
		case "page":
			if i+1 >= len(args) {
				return filter, fmt.Errorf("page requires a number")
			}
			page, err := strconv.Atoi(args[i+1])
			if err != nil || page < 1 {
				return filter, fmt.Errorf("invalid page value %q", args[i+1])
			}
			filter.Page = page
			i++
		case "search":
			if i+1 >= len(args) {
				return filter, fmt.Errorf("search requires a query")
			}
			var query []string
			i++
			for i < len(args) {
				if strings.EqualFold(args[i], "page") {
					i--
					break
				}
				query = append(query, args[i])
				i++
			}
			if len(query) == 0 {
				return filter, fmt.Errorf("search requires a query")
			}
			filter.Search = strings.Join(query, " ")
		default:
			return filter, fmt.Errorf("unknown filter %q", args[i])
		}
	}
	return filter, nil
}

func (tc *TelegramClient) listAddressBookEntries(ctx context.Context, filter AddressBookFilter) ([]addressBookEntry, error) {
	portals, err := tc.main.Bridge.GetAllPortals(ctx)
	if err != nil {
		return nil, err
	}
	entries := make([]addressBookEntry, 0, len(portals))
	for _, portal := range portals {
		peerType, peerID, topicID, ok := parseAddressBookPortalID(portal.ID)
		if !ok || topicID != 0 {
			// Addressbook entries are per Telegram peer. Forum topic portals are intentionally omitted
			// because manual allow/deny overrides are currently stored per peer, not per topic.
			continue
		}
		if !addressBookPortalBelongsToLogin(portal.PortalKey, peerType, tc.loginID) {
			continue
		}
		entry, err := tc.makeAddressBookEntry(ctx, portal, peerType, peerID)
		if err != nil {
			return nil, err
		}
		if addressBookEntryMatchesFilter(entry, filter) {
			entries = append(entries, entry)
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].PeerType != entries[j].PeerType {
			return entries[i].PeerType < entries[j].PeerType
		}
		nameI := strings.ToLower(entries[i].Name)
		nameJ := strings.ToLower(entries[j].Name)
		if nameI != nameJ {
			return nameI < nameJ
		}
		return entries[i].PeerID < entries[j].PeerID
	})
	return entries, nil
}

func parseAddressBookPortalID(portalID networkid.PortalID) (peerType ids.PeerType, peerID int64, topicID int, ok bool) {
	parts := strings.Split(string(portalID), ":")
	if len(parts) != 2 && len(parts) != 3 {
		return "", 0, 0, false
	}
	peerType = ids.PeerType(parts[0])
	switch peerType {
	case ids.PeerTypeUser, ids.PeerTypeChat, ids.PeerTypeChannel:
	default:
		return "", 0, 0, false
	}
	var err error
	peerID, err = strconv.ParseInt(parts[1], 10, 64)
	if err != nil || peerID <= 0 {
		return "", 0, 0, false
	}
	if len(parts) == 3 {
		if peerType != ids.PeerTypeChannel {
			return "", 0, 0, false
		}
		topicID, err = strconv.Atoi(parts[2])
		if err != nil {
			return "", 0, 0, false
		}
	}
	return peerType, peerID, topicID, true
}

func addressBookPortalBelongsToLogin(portalKey networkid.PortalKey, peerType ids.PeerType, loginID networkid.UserLoginID) bool {
	switch peerType {
	case ids.PeerTypeUser, ids.PeerTypeChat:
		return portalKey.Receiver == loginID
	case ids.PeerTypeChannel:
		return portalKey.Receiver == "" || portalKey.Receiver == loginID
	default:
		return false
	}
}

func (tc *TelegramClient) makeAddressBookEntry(
	ctx context.Context,
	portal *bridgev2.Portal,
	peerType ids.PeerType,
	peerID int64,
) (addressBookEntry, error) {
	filter := tc.filterForPeerType(peerType)
	enabled := filter.Enabled == nil || *filter.Enabled
	state, ok, err := tc.ScopedStore.GetPeerFilterOverride(ctx, peerType, peerID)
	if err != nil {
		return addressBookEntry{}, err
	}
	if !ok {
		state = "none"
	}
	name := portal.Name
	if name == "" {
		name = string(portal.ID)
	}
	return addressBookEntry{
		PeerType:      peerType,
		PeerID:        peerID,
		PortalID:      portal.ID,
		FriendlyID:    formatAddressBookFriendlyID(peerType, peerID),
		Name:          name,
		Bridged:       portal.MXID != "",
		Enabled:       enabled,
		ManualOnly:    filter.ManualOnly,
		ConfigAllowed: tc.allowPeerByConfig(ctx, peerType, peerID),
		ManualState:   state,
	}, nil
}

func addressBookEntryMatchesFilter(entry addressBookEntry, filter AddressBookFilter) bool {
	if filter.PeerType != nil && entry.PeerType != *filter.PeerType {
		return false
	}
	if filter.Bridged != nil && entry.Bridged != *filter.Bridged {
		return false
	}
	if filter.ManualOnly != nil && entry.ManualOnly != *filter.ManualOnly {
		return false
	}
	if filter.ManualState != "" && entry.ManualState != filter.ManualState {
		return false
	}
	if filter.Search != "" {
		query := strings.ToLower(filter.Search)
		haystack := strings.ToLower(strings.Join([]string{
			string(entry.PortalID),
			entry.FriendlyID,
			entry.Name,
		}, "\n"))
		if !strings.Contains(haystack, query) {
			return false
		}
	}
	return true
}

func formatAddressBookReply(filter AddressBookFilter, entries []addressBookEntry, commandPrefix string) string {
	var out strings.Builder
	out.WriteString("Telegram address book")
	if filter.Search != "" {
		out.WriteString("\nSearch: ")
		out.WriteString(format.EscapeMarkdown(filter.Search))
	}
	out.WriteString("\n\n")

	total := len(entries)
	start, end := addressBookPageBounds(filter, total)
	if total == 0 || start >= end {
		out.WriteString("No known Telegram peers found for the selected filters. Run ")
		out.WriteString(format.SafeMarkdownCode(commandPrefix + " sync"))
		out.WriteString(" or wait for dialog sync, then try again.")
		return out.String()
	}

	for i, entry := range entries[start:end] {
		if i > 0 {
			out.WriteString("\n\n")
		}
		out.WriteString(formatAddressBookEntry(start+i+1, entry, commandPrefix))
	}
	out.WriteString("\n\n")
	out.WriteString(fmt.Sprintf("Showing %d-%d of %d.", start+1, end, total))
	if end < total {
		out.WriteString("\nNext: ")
		out.WriteString(format.SafeMarkdownCode(formatAddressBookNextCommand(filter, commandPrefix)))
	}
	return out.String()
}

func addressBookPageBounds(filter AddressBookFilter, total int) (start, end int) {
	limit := filter.Limit
	if limit <= 0 {
		limit = addressBookDefaultLimit
	}
	page := filter.Page
	if page <= 0 {
		page = 1
	}
	start = (page - 1) * limit
	if start > total {
		start = total
	}
	end = min(start+limit, total)
	return start, end
}

func formatAddressBookEntry(index int, entry addressBookEntry, commandPrefix string) string {
	var out strings.Builder
	out.WriteString(fmt.Sprintf("%d. %s", index, format.EscapeMarkdown(entry.Name)))
	out.WriteString("\n   ID: ")
	out.WriteString(format.EscapeMarkdown(formatAddressBookPortalID(entry.PeerType, entry.PeerID)))
	out.WriteString("\n   Status: ")
	out.WriteString(format.EscapeMarkdown(formatAddressBookStatus(entry)))
	out.WriteString("\n   Manual state: ")
	out.WriteString(format.EscapeMarkdown(entry.ManualState))
	out.WriteString("\n   Media: not implemented in this command")
	if !entry.ConfigAllowed {
		out.WriteString("\n   Filter: rejected by current config")
	}
	out.WriteString("\n   Command: ")
	out.WriteString(format.SafeMarkdownCode(commandPrefix + " bridge " + entry.FriendlyID))
	return out.String()
}

func formatAddressBookPortalID(peerType ids.PeerType, peerID int64) string {
	portalID := ids.MakePortalID(peerType, peerID)
	friendlyID := formatAddressBookFriendlyID(peerType, peerID)
	if string(portalID) == friendlyID {
		return friendlyID
	}
	return fmt.Sprintf("%s / %s", portalID, friendlyID)
}

func formatAddressBookFriendlyID(peerType ids.PeerType, peerID int64) string {
	return formatUserFriendlyTelegramID(peerType, peerID)
}

func formatAddressBookStatus(entry addressBookEntry) string {
	var status []string
	if !entry.Enabled {
		status = append(status, "disabled")
	} else if entry.ManualOnly {
		status = append(status, "manual_only")
	} else {
		status = append(status, "auto")
	}
	if entry.Bridged {
		status = append(status, "bridged")
	} else {
		status = append(status, "not bridged")
	}
	return strings.Join(status, ", ")
}

func formatAddressBookNextCommand(filter AddressBookFilter, commandPrefix string) string {
	var args []string
	if filter.PeerType != nil {
		switch *filter.PeerType {
		case ids.PeerTypeUser:
			args = append(args, "users")
		case ids.PeerTypeChat:
			args = append(args, "groups")
		case ids.PeerTypeChannel:
			args = append(args, "channels")
		}
	}
	if filter.Bridged != nil {
		if *filter.Bridged {
			args = append(args, "bridged")
		} else {
			args = append(args, "unbridged")
		}
	}
	switch filter.ManualState {
	case store.PeerFilterOverrideAllow:
		args = append(args, "allowed")
	case store.PeerFilterOverrideDeny:
		args = append(args, "denied")
	}
	if filter.ManualOnly != nil && *filter.ManualOnly {
		args = append(args, "manual")
	}
	if filter.Search != "" {
		args = append(args, "search", filter.Search)
	}
	args = append(args, "page", strconv.Itoa(filter.Page+1))
	return commandPrefix + " addressbook " + strings.Join(args, " ")
}
