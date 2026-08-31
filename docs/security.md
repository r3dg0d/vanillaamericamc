# Security model

## Network and process boundaries

- Default game listeners: 127.0.0.1:25565/TCP and 127.0.0.1:19132/UDP.
- Public origin: Caddy at https://localhost:8444 only.
- Portal upstream: 127.0.0.1:18080 only.
- RCON: 127.0.0.1:25575, never opened in the firewall or sent to a browser.
- SQLite has no network listener. BlueMap's built-in listener is disabled.
- The `lan` option opens only Java TCP and Bedrock UDP game ports. It does not
  expose web administration.

Paper and portal have distinct unprivileged users. Shared state is limited to
the `va-plus-data` group. Systemd applies no-new-privileges, private temporary
and device namespaces, strict system protection, home protection, and explicit
write paths. Paper retains outbound networking and state writes needed for game
and plugin operation.

## Authentication and authorization

Passwords use Argon2id (64 MiB, three iterations, parallelism four) with random
salts. Sessions are random, server-stored, twelve-hour records. Cookies are
Secure, HttpOnly, SameSite=Strict; state changes require a constant-time checked
CSRF cookie/header pair. Login attempts are limited to five per address per
fifteen minutes.

Every privileged handler enforces role server-side. Moderator authority is
limited to reports and queued warn/mute/kick/tempban operations. Administrator
authority adds staff accounts, order fulfillment, lifecycle, allowlisted
console, and audit review. Moderators cannot reach systemd, RCON, filesystem,
secrets, OP assignment, role management, or shop fulfillment.

RCON input is mapped through an allowlist; untrusted catalog values are never
interpolated into commands. Shop fulfillment maps fixed catalog codes to fixed
LuckPerms permission nodes and uses `(order_id, entitlement_code)` uniqueness
for retry safety. Checkout is explicitly mock mode and performs no payment.

## Secrets

The activation creates random values only when absent:

- `credentials/portal-admin-password`
- `credentials/rcon-password`

They are mode 0640, root-owned, and group-readable only by service identities.
They are not in the Nix store, Git, process arguments, or logs. The first file
is a bootstrap input; changing it later does not silently replace an existing
database password. Use the documented reset command.

## Audit and privacy

Staff actions record actor, action, target, outcome, and UTC time.
SQLite triggers reject audit update/delete operations. Player reports are not
served to ordinary users. SearXNG uses POST, removes query titles, disables
uWSGI request logs, and retains only error logging. Health monitoring sends one
bounded synthetic query per hour.

## Shop policy review

The Minecraft EULA and Usage Guidelines were reviewed on 2026-08-30:

- https://www.minecraft.net/en-us/eula
- https://www.minecraft.net/en-us/usage-guidelines

The test catalog contains only noncompetitive cosmetic/community permission
entitlements. It has no loot boxes, gambling, scarcity claims, competitive
advantages, cash-out currency, or live payment path. The site includes a
non-affiliation disclaimer. This is an engineering policy review, not legal
advice.
