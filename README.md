# mautrix-telegram
![Languages](https://img.shields.io/github/languages/top/mautrix/telegram.svg)
[![License](https://img.shields.io/github/license/mautrix/telegram.svg)](LICENSE)
[![Release](https://img.shields.io/github/release/mautrix/telegram/all.svg)](https://github.com/mautrix/telegram/releases)
[![GitLab CI](https://mau.dev/mautrix/telegram/badges/main/pipeline.svg)](https://mau.dev/mautrix/telegram/container_registry)

## Custom Telegram peer filtering

This fork adds per-peer-type filtering for Telegram chats before they are bridged into Matrix.

The goal is to control which Telegram peers can create or update Matrix portal rooms. Peer types can also be marked `manual_only` to let dialog sync discover them without creating Matrix rooms until a user runs the `bridge` command.

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
    manual_only: false
    load_media: true
    list: []

  groups:
    enabled: false
    mode: blacklist
    manual_only: true
    load_media: false
    list: []

  channels:
    enabled: true
    mode: whitelist
    manual_only: true
    load_media: false
    list:
      - "-1001234567890"
      - "-1009876543210"
      - "regex:-100555[0-9]+"
```

`enabled: false` disables both automatic and manual bridging for that peer type. With `manual_only: true`, peers still need to pass the blacklist/whitelist, but Matrix rooms are only created after a successful manual `bridge`; `unbridge` records a persistent deny state so new Telegram messages do not recreate the room.

`load_media: false` keeps text messages bridged for allowed peers, but replaces Telegram media uploads with a text placeholder instead of downloading the file from Telegram and uploading it to Matrix. If the option is omitted, it defaults to `true` for backwards compatibility.

Limitation: Telegram albums/grouped media may currently appear as separate `[media: 1]` placeholders until an album collector is implemented before message conversion.

`portal_name_prefix` / `portal_name_suffix` add a global prefix/suffix to Matrix portal room names only. They do not affect ghost/user display names or Telegram peer IDs.

TODO: manual allow/deny overrides are currently stored per Telegram peer, not per forum topic.

## Production deployment

This fork includes a Docker release workflow and a Helm chart for the production Kubernetes deployment.

Release image flow:

```bash
git tag v0.1.0
git push origin custom-peer-filter
git push origin v0.1.0
```

The release tag build pushes:

```text
ghcr.io/antonsayapin/mautrix-telegram:v0.1.0
ghcr.io/antonsayapin/mautrix-telegram:latest
```

The Helm chart is in `deploy/helm/mautrix-telegram`. It deploys `StatefulSet/mautrix-telegram` and `Service/mautrix-telegram` in the target namespace, mounts the existing `mautrix-telegram-data` PVC at `/data`, and expects `/data/config.yaml` and `/data/registration.yaml` to already exist.

First migration warning:

- The current manual `StatefulSet` and `Service` are not Helm-managed.
- The first migration requires deleting the old `StatefulSet` and `Service` while keeping the PVC.
- Never delete `mautrix-telegram-data`.

Example first migration:

```bash
kubectl scale statefulset/mautrix-telegram -n ess --replicas=0
kubectl delete statefulset mautrix-telegram -n ess
kubectl delete service mautrix-telegram -n ess
kubectl get pvc -n ess | grep mautrix-telegram
```

Server upgrade flow:

```bash
cd ~/mautrix-telegram-install/telegram

git fetch --tags origin
git checkout v0.1.0

helm package deploy/helm/mautrix-telegram --destination ~/mautrix-telegram-install

sed -i 's/tag: v[0-9]\+\.[0-9]\+\.[0-9]\+/tag: v0.1.0/' \
  ~/ess-config-values/mautrix-telegram/mautrix-telegram-values.yaml

helm upgrade --install mautrix-telegram \
  ~/mautrix-telegram-install/mautrix-telegram-0.1.0.tgz \
  --namespace ess \
  -f ~/ess-config-values/mautrix-telegram/mautrix-telegram-values.yaml
```

Do not commit generated `.tgz` Helm packages.

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
