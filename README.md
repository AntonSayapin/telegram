# mautrix-telegram
![Languages](https://img.shields.io/github/languages/top/mautrix/telegram.svg)
[![License](https://img.shields.io/github/license/mautrix/telegram.svg)](LICENSE)
[![Release](https://img.shields.io/github/release/mautrix/telegram/all.svg)](https://github.com/mautrix/telegram/releases)
[![GitLab CI](https://mau.dev/mautrix/telegram/badges/main/pipeline.svg)](https://mau.dev/mautrix/telegram/container_registry)

## Custom Telegram peer filtering

This fork adds per-peer-type filtering for Telegram chats before they are bridged into Matrix.

The goal is to control which Telegram peers can create or update Matrix portal rooms.

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
    enabled: false
    mode: blacklist
    list: []

  groups:
    enabled: false
    mode: blacklist
    list: []

  channels:
    enabled: true
    mode: whitelist
    list:
      - "-1001234567890"
      - "-1009876543210"
      - "regex:-100555[0-9]+"
```

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
