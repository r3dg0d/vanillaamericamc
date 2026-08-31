# Validation report

Date: 2026-08-30. Target: `nixos-btw`.

| Check | Result | Evidence/limit |
|---|---|---|
| Host flake evaluation | Pass | `nix eval --show-trace .#nixosConfigurations.nixos-btw.config.system.build.toplevel.drvPath` |
| Actual host build | Pass | `nix build --no-link .#nixosConfigurations.nixos-btw.config.system.build.toplevel` produced the target closure |
| Portal unit/vet/build | Pass | `go test ./...`, `go vet ./...`, production Go build |
| Portal auth/RBAC tests | Pass | Argon2id login/session, moderator admin denial, immutable audit |
| Mock order tests | Pass | catalog allowlist and idempotent fulfillment/entitlement row |
| Plugin clean test/JAR | Pass | `./gradlew --no-daemon clean test shadowJar`; release JAR staged |
| Plugin logic tests | Pass | report input/cooldown logic and entitlement allowlist validation |
| SearXNG diagnosis | Pass | bounded engine tests and journal evidence identify upstream CAPTCHA/429 |
| SearXNG default-pool config | Pass build | blocked engines absent; Bing/SearchMySite/Wikipedia plus specialized engines |
| Local limiter | Pass build | socket-only Redis-compatible service and documented limiter schema |
| Secret scan/design | Pass | no credential value in source/store; generated root-owned files |
| Port design | Pass build | web/RCON/DB loopback; game ports loopback by default; firewall conditional |
| Real Paper plugin load | Not run | Minecraft EULA has not been accepted |
| Java client join | Not run | requires EULA acceptance and a real client |
| Bedrock client join | Not run | requires EULA acceptance and a real Bedrock device |
| BlueMap first render | Not run | requires first Paper world start |

After activation, service-level checks are appended before handoff:

```console
systemctl is-active uwsgi redis-searx va-plus-portal caddy
systemctl status vanilla-america-plus
curl -kfsS https://localhost:8444/api/status
curl -fsS http://localhost:8888/
ss -lntup
```

Manual post-EULA matrix:

1. Join Java 26.2 at localhost:25565; run `/va`, `/rules`, `/report`.
2. Join a supported Bedrock 26.0–26.40 client at localhost:19132 and verify
   Floodgate identification/welcome fallback.
3. Confirm Geyser/Floodgate/LuckPerms/BlueMap/VanillaAmericaPlus have no severe
   startup exceptions.
4. Trigger one BlueMap render and open `/map/`.
5. Create a mock order, fulfill it as admin twice, and confirm one entitlement.
6. Log in as moderator and confirm reports work while console, lifecycle,
   users, fulfillment, and audit administration return 403.
7. Create a backup, inspect its member list, and rehearse restore with copied
   disposable state.
