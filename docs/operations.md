# Operations

## Addresses and services

- Portal/panel/shop: https://localhost:8444/
- Map after first BlueMap render: https://localhost:8444/map/
- SearXNG: http://localhost:8888/
- Java: localhost:25565/TCP
- Bedrock: localhost:19132/UDP

`systemctl status va-plus-portal caddy vanilla-america-plus` shows state.
Paper is expected to be inactive with a skipped EULA condition until acceptance.

## First start and credentials

Read https://aka.ms/MinecraftEULA. If you accept it, run:

```console
sudo va-plus-accept-eula
```

The script requires typing `I-ACCEPT`; no acceptance is inferred by a rebuild.
Retrieve the initial local administrator password without copying it into shell
history:

```console
sudo sed -n '1p' /var/lib/vanilla-america-plus/credentials/portal-admin-password
```

Log in as `admin`, then create separate moderator/admin accounts. Reset an
account using a root-readable temporary file:

```console
sudo va-plus-reset-admin admin /path/to/new-password-file
```

The password must be 14–256 characters. Delete the temporary file afterward.

## Lifecycle and logs

```console
sudo systemctl start vanilla-america-plus
sudo systemctl stop vanilla-america-plus
sudo systemctl restart vanilla-america-plus
journalctl -u vanilla-america-plus -f
journalctl -u va-plus-portal -f
journalctl -u caddy -f
```

The administrator desk exposes these same three fixed lifecycle verbs and a
small RCON command allowlist. It is not a shell or raw console.

## Backups and restore

Run an immediate consistent backup:

```console
sudo systemctl start va-plus-backup
sudo journalctl -u va-plus-backup --since today
sudo ls -lh /var/lib/vanilla-america-plus/backups
```

The job uses `save-off` plus `save-all flush` while online, always restores
`save-on`, creates timestamped `.tar.zst` archives, and removes archives
older than fourteen days.

Restore requires an intentional maintenance window:

1. Stop Paper: `sudo systemctl stop vanilla-america-plus`.
2. Move the current `server` and `integration` directories aside; do not
   delete them.
3. Inspect the archive with `tar --zstd -tf BACKUP`.
4. Extract from `/var/lib/vanilla-america-plus` using
   `sudo tar --zstd -xf BACKUP`.
5. Restore ownership to `va-plus:va-plus-data`, start Paper, and inspect logs.
6. Keep the moved-aside state until Java and Bedrock checks pass.

## SearXNG engine recovery

Status and logs:

```console
systemctl status uwsgi redis-searx searx-healthcheck.timer
journalctl -u uwsgi -u searx-healthcheck --since today
curl -fsS http://127.0.0.1:8888/stats
```

Brave, DuckDuckGo, and Startpage were removed from defaults because their
upstreams challenged this shared VPN egress. Do not treat an expired suspension
as recovery. To retest, add one engine temporarily to `keep_only`, rebuild,
then issue two ordinary queries several minutes apart. A CAPTCHA, 429, or
immediate suspension means remove it again. Do not solve challenges, rotate
identities, or increase retry/concurrency.

Rollback SearXNG and VA+ together by selecting the previous NixOS generation:

```console
sudo nixos-rebuild switch --rollback
```

World/database state remains under `/var/lib` across configuration rollback.

## Updates

All server/plugin URLs and hashes are in
`modules/nixos/vanilla-america-plus.nix`. Never replace them with latest tags.
Update one layer at a time, rebuild plugin/tests, build the host, back up, switch,
and inspect startup. The portal Go dependency hash must change only when
`go.mod` or `go.sum` changes.

## LAN exposure

Change `networkScope = "loopback"` to `"lan"` only after deciding that LAN
players should connect. Rebuild opens exactly 25565/TCP and 19132/UDP. Router,
DNS, CDN, public firewall, and live payments remain separate authorization
boundaries and are not configured here.
