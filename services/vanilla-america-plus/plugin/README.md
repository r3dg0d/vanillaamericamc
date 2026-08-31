# VanillaAmericaPlus

Production Paper plugin for the Vanilla America+ server. It provides branded,
Bedrock-safe onboarding, editable guides/rules/facts, persistent player reports,
immutable staff audit rows, a cosmetic-only entitlement queue, and a bounded
Liberty Bell celebration. No client mod or resource pack is required.

## Compatibility

- Paper 26.2 build 121, Java 25
- Geyser 2.11.2 and Floodgate 2.2.5 are optional soft integrations
- LuckPerms 5.5 is optional and is used for allowlisted entitlement permissions
- BlueMap, PlaceholderAPI, and Vault are soft dependencies only; this release
  does not require or directly call them

Floodgate detection uses its documented API. When Floodgate is absent, join
messaging falls back to Java-safe text. Essential output avoids hover-only
instructions, custom fonts, inventories, and mandatory clickable components.

## Build and test

```sh
./gradlew clean test shadowJar
```

The release JAR is `build/libs/VanillaAmericaPlus-1.0.0.jar`.

## Commands and permissions

- `/va`, `/guide`, `/helpva`: `vaplus.guide`
- `/rules`: `vaplus.rules`
- `/report <player> <reason>`: `vaplus.report`
- `/reports`, `/report view`: `vaplus.staff.reports`
- `/report resolve|dismiss`: `vaplus.staff.resolve`
- `/va reload`: `vaplus.admin.reload`
- `/libertybell`: `vaplus.event.liberty`

## Persistence and portal contract

The configured SQLite file is opened in WAL mode. Reports, staff audit records,
and entitlement rows share this database with the loopback-only portal. Audit
rows are protected by database triggers that reject UPDATE and DELETE. The
plugin polls pending entitlements on one bounded worker and applies only codes
listed in `entitlements.allowed`; order IDs are unique and safe to retry.

Back up the database with the server stopped or via SQLite's online backup
mechanism. To roll back, stop Paper, restore the previous JAR and database copy,
then start Paper. Never use Bukkit's `/reload`; `/va reload` validates only
this plugin's editable settings.

