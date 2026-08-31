# Plugin matrix

Reviewed 2026-08-30. Downloads in the Nix module use official URLs and fixed
SHA-256 hashes.

| Purpose | Candidate/version | Decision | Source/license | Java/Bedrock and maintenance notes | Data |
|---|---|---|---|---|---|
| Server | Paper 26.2 build 121 | Selected | PaperMC / GPL-3.0 | Current stable API; Java 25; no client mod | server root/worlds |
| Cross-play | Geyser 2.11.2 build 1233 | Selected | GeyserMC / MIT | Supports Java 26.2 and Bedrock 26.0–26.40; Paper plugin recommended | plugins/Geyser-Spigot |
| Bedrock identity | Floodgate 2.2.5 build 140 | Selected | GeyserMC / MIT | Bedrock accounts without Java ownership; optional API detection in VA+ | plugins/floodgate |
| Permissions | LuckPerms 5.5.81 | Selected | LuckPerms / MIT | One permission authority; required for safe entitlement nodes | plugins/LuckPerms |
| Map | BlueMap 5.23 | Selected | BlueMap / MIT | Paper 26.2 release; webserver disabled and files served locally by Caddy | plugins/BlueMap |
| VA+ identity/reports/shop bridge | VanillaAmericaPlus 1.0.0 | Selected | Local source / project license | Adventure messages, Floodgate fallback, reports, audited idempotent entitlements | integration DB and plugin config |
| Protocol translation | ViaVersion | Rejected now | ViaVersion / GPL-3.0 | Geyser and Paper both target 26.2; adding it would expand protocol/test surface without need | n/a |
| Rollback/audit | CoreProtect | Deferred | CoreProtect / Artistic-2.0 | Upstream claims current support, but no live world exists and runtime compatibility is unverified here; add after an EULA-gated staging load | n/a |
| Claims | GriefPrevention/HuskClaims | Rejected initially | respective upstreams | Land claims materially alter vanilla survival; evaluate only after community demand | n/a |
| Moderation suite | external bundle | Rejected | varies | VA+ owns bounded reports; portal queues only warn/mute/kick/tempban. Avoid duplicate ban databases | integration DB |
| Economy/Vault | Vault plus economy plugin | Rejected | varies | Shop grants cosmetics/permissions only; no gameplay economy is needed | n/a |
| Placeholders | PlaceholderAPI | Soft integration only | PlaceholderAPI / eCloud terms | VA+ does not require it; no reason to install yet | n/a |
| Anti-cheat | Boar/Grim | Deferred | upstream specific | Geyser compatibility needs real Bedrock calibration; false-positive risk outweighs benefit before client tests | n/a |
| Profiling | Paper spark | Selected built-in | Paper bundle | No extra plugin; use Paper's maintained profiler tooling | Paper logs/profiles |

Update procedure:

1. Read release notes and exact game/API compatibility.
2. Replace URL, versioned filename, and SHA-256 together in
   `modules/nixos/vanilla-america-plus.nix`.
3. Build the VA+ plugin against the new Paper API.
4. Run Gradle and Go tests, then the target NixOS build.
5. Back up and test on loopback before switching.
