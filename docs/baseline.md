# Vanilla America+ host baseline

Recorded 2026-08-30 before implementation. Secrets and complete public network
identifiers are intentionally omitted.

## Host

- NixOS 26.05.20260827.d57af92, Linux 7.2.0, x86_64.
- Intel Core i9-14900K, 31 GiB RAM, 931 GiB available storage.
- Java 25.0.4, Docker 29.7.2, Docker Compose 5.4.0.
- Active host: `nixos-btw`; source: `/home/r3dg0d/nixos-dotfiles#nixos-btw`.
- `system.stateVersion = "26.05"`; deployment uses `nixos-rebuild switch --flake`.
- The repository had unrelated modified and untracked files before this work.
  Those files were preserved.
- Docker runs an unrelated MoneroPay stack. No container or network was changed.
- No reverse proxy or NixOS-managed SQL service was present. Existing private
  credentials use root-owned files/systemd credentials; sops-nix and agenix
  were not configured.

## Existing network surface

SearXNG listened on 127.0.0.1:8888. Candidate VA+ ports 25565/TCP,
19132/UDP, 8444/TCP, 18080/TCP, and 8100/TCP were unused. Existing unrelated
listeners included MPD 6600, SIVA 8787, ReClip 8900, MoneroPay 5000, and
Jellyfin 8096. The implementation does not alter those services.

The host's egress traversed a connected Mullvad VPN relay. This is a shared VPN
egress category and can have upstream reputation/rate-limit effects. The relay
address and full public IP are not recorded here.

## SearXNG before repair

- NixOS `services.searx`, package `0-unstable-2026-05-16`.
- Built-in server on 127.0.0.1:8888; settings generated from
  `modules/nixos/services.nix`; secret from root-owned `/etc/searxng.env`.
- No reverse proxy, limiter datastore, or local inbound rate limiting.
- Logs showed Brave `SearxEngineTooManyRequestsException` with 180-second
  suspension, DuckDuckGo `SearxEngineCaptchaException`, and Startpage
  redirects to `/sp/captcha` with 3600-second suspension.
- These errors occurred on low-volume direct searches and were upstream
  responses, not local reverse-proxy or limiter failures.
- Other tested engines: Bing, SearchMySite, and Wikipedia returned useful
  results. Qwant and Yep denied access; Yahoo/AOL returned protocol errors;
  Mwmbl and Presearch timed out during bounded tests.

## Minecraft before implementation

No Minecraft server process, world, panel, map, relevant listener, or current
server directory was present. There is therefore no world migration. No
Minecraft EULA acceptance was found or inferred.

The conservative initial budget is a 2 GiB initial and 8 GiB maximum Paper
heap, with systemd memory pressure at 10 GiB and a hard limit at 12 GiB. This
leaves ample memory for the desktop, map rendering, and local services.

## Baseline evidence commands

```console
nixos-version
uname -a
lscpu
free -h
df -h /
java -version
docker version
git status --short --branch
ss -lntup
systemctl status searx
journalctl -u searx
mullvad status
```
