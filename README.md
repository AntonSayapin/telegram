# mautrix-telegram
![Languages](https://img.shields.io/github/languages/top/mautrix/telegram.svg)
[![License](https://img.shields.io/github/license/mautrix/telegram.svg)](LICENSE)
[![Release](https://img.shields.io/github/release/mautrix/telegram/all.svg)](https://github.com/mautrix/telegram/releases)
[![GitLab CI](https://mau.dev/mautrix/telegram/badges/main/pipeline.svg)](https://mau.dev/mautrix/telegram/container_registry)

## Custom Telegram peer filtering

This fork adds per-peer-type filtering for Telegram chats before they are bridged into Matrix.

The goal is to control which Telegram peers can create or update Matrix portal rooms. Peer types can also be marked `manual_only` to let dialog sync discover them without creating Matrix rooms until a user runs the explicit `manualbridge` command.

### Supported Telegram peer types

The bridge separates Telegram peers into three logical groups:

| Config section | Telegram peer type | Meaning |
|---|---|---|
| `users` | `user` | Direct/private Telegram chats |
| `groups` | `chat` | Legacy Telegram groups |
| `channels` | `channel` | Telegram channels and supergroups |

Important: Telegram supergroups are usually represented by Telegram API as `channel`, so most modern Telegram groups should be filtered under the `channels` section.

### New config block

A new top-level `filter` section was added to the bridge config:

```yaml
filter:
  users:
    enabled: true
    mode: blacklist
    manual_only: false
    load_media: true
    list: []

  groups:
    enabled: true
    mode: blacklist
    manual_only: true
    load_media: false
    list: []

  channels:
    enabled: true
    mode: blacklist
    manual_only: true
    load_media: false
    list: []
```

`enabled: false` disables both automatic and manual bridging for that peer type. With `manual_only: true`, peers still need to pass the blacklist/whitelist, but Matrix rooms are only created after a successful `manualbridge`; `manualunbridge` records a persistent deny state so new Telegram messages do not recreate the room.

In `mode: blacklist`, all peers of the type are allowed except entries in `list`. In `mode: whitelist`, only entries in `list` are allowed. Channel and supergroup IDs may be written in the Telegram-friendly `-100...` form.

`load_media: false` keeps text messages bridged for allowed peers, but replaces Telegram media uploads with a text placeholder instead of downloading the file from Telegram and uploading it to Matrix. If the option is omitted, it defaults to `true` for backwards compatibility.

Limitation: Telegram albums/grouped media may currently appear as separate `[media: 1]` placeholders until an album collector is implemented before message conversion.

The `!tg addressbook` command lists known Telegram peers that have been discovered by dialog sync and stored as portal records. It supports type, bridged/unbridged, manual state, page, and search filters, and does not create rooms, download media, backfill, or change manual allow/deny state.

Manual-only peers should be bridged with:

```text
!tg manualbridge -1001234567890
!tg manualbridge channel:1234567890
!tg manualbridge chat:123456789
!tg manualbridge user:123456789
```

`manualbridge` creates or opens the dedicated Matrix portal room for the Telegram peer. It does not bind the current management room and does not replace the upstream `bridge` command. If the portal already has a Matrix room, the command reports the existing room instead of creating a duplicate and saves the manual allow state only after the static filter config accepts the peer.

`manualbridge --overwrite ...` is parsed and validated, but overwriting an existing portal room is currently refused with a clear error. The bridgev2 portal API creates rooms with a deterministic room ID for each portal key and does not currently expose a safe non-destructive API to detach an existing Matrix room and create a fresh replacement without risking room reuse or deletion.

Use `!tg manualunbridge` in the portal room to run the normal unbridge flow and then persist a manual deny state after the portal binding has been removed. The upstream `bridge` and `unbridge` commands keep their original behavior.

`portal_name_prefix` / `portal_name_suffix` add a global prefix/suffix to Matrix portal room names only. They do not affect ghost/user display names or Telegram peer IDs.

Use `!tg resync-names --dry-run` to preview Matrix room name changes for already bridged Telegram portals after changing `portal_name_prefix` or `portal_name_suffix`. Use `!tg resync-names` to apply the room name updates. The command only updates existing portal room names; it does not create rooms, bridge new peers, download media, backfill messages, or change manual allow/deny state.

TODO: manual allow/deny overrides are currently stored per Telegram peer, not per forum topic.

## Deploy

This fork includes an optional Helm chart under deploy/helm/mautrix-telegram.
The chart expects an existing PVC with bridge config and registration files.
Do not commit real config.yaml, registration.yaml, tokens, generated chart packages, or environment-specific values.

## Sponsors
* [Joel Lehtonen / Zouppen](https://github.com/zouppen)

## Documentation
All setup and usage instructions are located on
[docs.mau.fi](https://docs.mau.fi/bridges/go/telegram/index.html).
Some quick links:

* [Bridge setup](https://docs.mau.fi/bridges/go/setup.html?bridge=telegram)
  (or [with Docker](https://docs.mau.fi/bridges/general/docker-setup.html?bridge=telegram))
* Basic usage: [Authentication](https://docs.mau.fi/bridges/go/telegram/authentication.html)

### Features & Roadmap
[ROADMAP.md](ROADMAP.md) contains a general overview of what is supported by the bridge.

## Discussion
Matrix room: [`#telegram:maunium.net`](https://matrix.to/#/#telegram:maunium.net)

Telegram chat: [`mautrix_telegram`](https://t.me/mautrix_telegram) (bridged to Matrix room)
