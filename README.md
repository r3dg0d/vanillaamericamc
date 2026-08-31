# Vanilla America+

Vanilla America+ (VA+) is a client-mod-free Paper server project for Java and
Bedrock cross-play, with a local operations portal, BlueMap, audited reports,
and a mock-only shop.

This repository contains the custom Paper plugin, portal source, NixOS module,
release pins, and operator documentation. The deployment defaults to
loopback-only listeners. It does not accept the Minecraft EULA, expose public
ports, create external accounts, or enable live payments automatically.

## Local project

- Portal source: `services/vanilla-america-plus/portal`
- Paper plugin: `services/vanilla-america-plus/plugin`
- NixOS module: `modules/nixos/vanilla-america-plus.nix`
- Operations docs: `docs/`

The public project page is generated from the portal's static frontend by the
GitHub Pages workflow. GitHub Pages cannot host the Paper process, RCON,
database, map backend, or Bedrock UDP listener.

## License and trademarks

The VA+ source is provided for this project; third-party components retain
their own licenses. Vanilla America+ is an independent community project and
is not affiliated with Mojang or Microsoft.
