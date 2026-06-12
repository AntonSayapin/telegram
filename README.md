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

`enabled: false` disables both automatic and manual bridging for that peer type. With `manual_only: true`, peers still need to pass the blacklist/whitelist, but Matrix rooms are only created after a successful manual `bridge`; `unbridge` records a persistent deny state so new Telegram messages do not recreate the room.

In `mode: blacklist`, all peers of the type are allowed except entries in `list`. In `mode: whitelist`, only entries in `list` are allowed. Channel and supergroup IDs may be written in the Telegram-friendly `-100...` form.

`load_media: false` keeps text messages bridged for allowed peers, but replaces Telegram media uploads with a text placeholder instead of downloading the file from Telegram and uploading it to Matrix. If the option is omitted, it defaults to `true` for backwards compatibility.

Limitation: Telegram albums/grouped media may currently appear as separate `[media: 1]` placeholders until an album collector is implemented before message conversion.

The `!tg addressbook` command lists known Telegram peers that have been discovered by dialog sync and stored as portal records. It supports type, bridged/unbridged, manual state, page, and search filters, and does not create rooms, download media, backfill, or change manual allow/deny state.

`portal_name_prefix` / `portal_name_suffix` add a global prefix/suffix to Matrix portal room names only. They do not affect ghost/user display names or Telegram peer IDs.

TODO: manual allow/deny overrides are currently stored per Telegram peer, not per forum topic.

## Production deployment

This fork includes a Docker release workflow and a Helm chart for the production Kubernetes deployment.

Release image flow:

```bash
export RELEASE_TAG=v26.05.3-antonsayapin

git tag "$RELEASE_TAG"
git push origin custom-peer-filter
git push origin "$RELEASE_TAG"
```

The release tag build pushes:

```text
ghcr.io/antonsayapin/mautrix-telegram:v26.05.3-antonsayapin
ghcr.io/antonsayapin/mautrix-telegram:latest
```

The Docker workflow runs on tags matching `v*.*.*-antonsayapin` and uses the git tag name as the Docker image tag.

The Helm chart is in `deploy/helm/mautrix-telegram`. It deploys `StatefulSet/mautrix-telegram` and `Service/mautrix-telegram` in the target namespace, mounts the existing `mautrix-telegram-data` PVC at `/data`, and expects `/data/config.yaml` and `/data/registration.yaml` to already exist.

The chart intentionally does not create a PVC. `persistence.existingClaim` is required and template rendering fails if it is empty, to avoid accidentally starting the bridge without the existing `/data` volume. The chart also enforces `replicaCount: 1` because the production PVC is RWO and the bridge must be single-instance.

Liveness and readiness probes are disabled by default because this deployment does not currently expose a confirmed health endpoint. Resources and security context are left minimal by default.

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

export RELEASE_TAG=v26.05.3-antonsayapin

git fetch --tags origin
git checkout "$RELEASE_TAG"

helm package deploy/helm/mautrix-telegram --destination ~/mautrix-telegram-install

sed -i "s/^  tag: .*/  tag: ${RELEASE_TAG}/" \
  ~/ess-config-values/mautrix-telegram/mautrix-telegram-values.yaml

helm upgrade --install mautrix-telegram \
  ~/mautrix-telegram-install/mautrix-telegram-0.1.0.tgz \
  --namespace ess \
  -f ~/ess-config-values/mautrix-telegram/mautrix-telegram-values.yaml
```

The chart package filename follows `deploy/helm/mautrix-telegram/Chart.yaml` (`mautrix-telegram-0.1.0.tgz` in the example above). The container image tag is controlled separately by `image.tag` in values.

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
