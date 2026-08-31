{ config, lib, pkgs, ... }:

let
  cfg = config.services.vanillaAmericaPlus;
  stateDir = "/var/lib/vanilla-america-plus";
  serverDir = "${stateDir}/server";
  integrationDir = "${stateDir}/integration";
  credentialsDir = "${stateDir}/credentials";
  backupDir = "${stateDir}/backups";
  gameBind = if cfg.networkScope == "lan" then "0.0.0.0" else "127.0.0.1";

  paper = pkgs.fetchurl {
    url = "https://fill-data.papermc.io/v1/objects/0de30efb024bc8b83c9c7d507d11802897ad8056b6110ec09fe1a91d126ccb54/paper-26.2-121.jar";
    hash = "sha256-DeMO+wJLyLg8nH1QfRGAKJetgFa2EQ7An+GpHRJsy1Q=";
  };
  geyser = pkgs.fetchurl {
    url = "https://download.geysermc.org/v2/projects/geyser/versions/2.11.2/builds/1233/downloads/spigot";
    hash = "sha256-qFGt6yMuRWRFJs4WJj6BnOtCepjzkZ5al+YzSxZcL4M=";
  };
  floodgate = pkgs.fetchurl {
    url = "https://download.geysermc.org/v2/projects/floodgate/versions/2.2.5/builds/140/downloads/spigot";
    hash = "sha256-n0NsQv/YsQkaQ316Thb4IYG51TFPixcy36nVpP/7Gf4=";
  };
  blueMap = pkgs.fetchurl {
    url = "https://github.com/BlueMap-Minecraft/BlueMap/releases/download/v5.23/bluemap-5.23-paper.jar";
    hash = "sha256-M5VU11ztqzVON2Z3z8cwjEmZUpFYSejimUcY5KFT1k4=";
  };
  luckPerms = pkgs.fetchurl {
    url = "https://download.luckperms.net/1668/bukkit/loader/LuckPerms-Bukkit-5.5.81.jar";
    hash = "sha256-J+ADARO60O/AnvdYGOc1c/a87Cscxy9kAAvEIWARORg=";
  };
  vaPlugin = ../../services/vanilla-america-plus/plugin/artifacts/VanillaAmericaPlus-1.0.0.jar;

  portal = pkgs.buildGoModule {
    pname = "vanilla-america-plus-portal";
    version = "1.0.0";
    src = ../../services/vanilla-america-plus/portal;
    vendorHash = "sha256-SFCy7e9nMdlmXsjjVWXPehbfW4ORzvdLey/r13oUluU=";
    ldflags = [ "-s" "-w" ];
  };

  setupScript = pkgs.writeShellScript "va-plus-setup" ''
        set -eu
        umask 0007
        install -d -m 0770 -o va-plus -g va-plus-data \
          ${serverDir} ${serverDir}/plugins ${serverDir}/plugins/BlueMap \
          ${serverDir}/plugins/VanillaAmericaPlus ${integrationDir}
        install -d -m 0750 -o va-plus -g va-plus-data ${backupDir}
        if [ ! -e ${serverDir}/eula.txt ]; then
          printf '%s\n' '# Set eula=true only after reading and accepting https://aka.ms/MinecraftEULA' \
            'eula=false' > ${serverDir}/eula.txt
          chown va-plus:va-plus-data ${serverDir}/eula.txt
          chmod 0660 ${serverDir}/eula.txt
        fi
        install -m 0640 -o va-plus -g va-plus-data ${paper} ${serverDir}/paper-26.2-121.jar
        install -m 0640 -o va-plus -g va-plus-data ${geyser} ${serverDir}/plugins/Geyser-Spigot-2.11.2.jar
        install -m 0640 -o va-plus -g va-plus-data ${floodgate} ${serverDir}/plugins/floodgate-spigot-2.2.5.jar
        install -m 0640 -o va-plus -g va-plus-data ${blueMap} ${serverDir}/plugins/bluemap-5.23-paper.jar
        install -m 0640 -o va-plus -g va-plus-data ${luckPerms} ${serverDir}/plugins/LuckPerms-Bukkit-5.5.81.jar
        install -m 0640 -o va-plus -g va-plus-data ${vaPlugin} ${serverDir}/plugins/VanillaAmericaPlus-1.0.0.jar
        cat > ${serverDir}/plugins/BlueMap/webserver.conf <<'BLUE'
    enabled: false
    webroot: "bluemap/web"
    port: 8100
    sse-enabled: false
    BLUE
        chown va-plus:va-plus-data ${serverDir}/plugins/BlueMap/webserver.conf
        chmod 0640 ${serverDir}/plugins/BlueMap/webserver.conf
        cat > ${serverDir}/plugins/VanillaAmericaPlus/config.yml <<'VAPLUS'
    theme:
      primary: "#0B1F3A"
      accent: "#B22234"
      ivory: "#F5F0E6"
    connection:
      java: "localhost:25565"
      bedrock: "localhost:19132"
    reports:
      cooldown-seconds: 300
      max-reason-length: 240
      page-size: 8
    database:
      path: "${integrationDir}/va-plus.db"
    entitlements:
      poll-seconds: 10
      allowed:
        supporter_badge: "vanillaamericaplus.cosmetic.supporter"
        liberty_bell: "vanillaamericaplus.libertybell"
    celebration:
      enabled: true
      cooldown-seconds: 900
      radius: 32
    facts:
      interval-minutes: 20
      entries:
        - "The Appalachian Trail crosses fourteen states."
        - "Great Basin National Park protects some of the world's oldest known trees."
        - "The Great Lakes hold roughly one fifth of the world's surface fresh water."
    VAPLUS
        chown va-plus:va-plus-data ${serverDir}/plugins/VanillaAmericaPlus/config.yml
        chmod 0640 ${serverDir}/plugins/VanillaAmericaPlus/config.yml
  '';

  credentialsScript = pkgs.writeShellScript "va-plus-credentials" ''
    set -eu
    umask 0027
    install -d -m 0750 -o root -g va-plus-secrets ${credentialsDir}
    for name in portal-admin-password rcon-password; do
      if [ ! -s "${credentialsDir}/$name" ]; then
        ${pkgs.openssl}/bin/openssl rand -base64 36 > "${credentialsDir}/$name"
      fi
      chown root:va-plus-secrets "${credentialsDir}/$name"
      chmod 0640 "${credentialsDir}/$name"
    done
  '';

  serverProperties = pkgs.writeShellScript "va-plus-server-properties" ''
    set -eu
    umask 0007
    rcon_password=$(tr -d '\r\n' < ${credentialsDir}/rcon-password)
    {
      printf '%s\n' \
        'motd=Vanilla America+ — Cross-Platform Survival' \
        'gamemode=survival' 'difficulty=normal' 'hardcore=false' \
        'online-mode=true' 'enforce-secure-profile=true' \
        'server-ip=${gameBind}' 'server-port=25565' \
        'max-players=30' 'view-distance=10' 'simulation-distance=8' \
        'spawn-protection=16' 'allow-flight=false' 'enable-command-block=false' \
        'enable-rcon=true' 'rcon.port=25575' 'broadcast-rcon-to-ops=false' \
        "rcon.password=$rcon_password"
    } > ${serverDir}/server.properties
    chmod 0600 ${serverDir}/server.properties
  '';

  stopScript = pkgs.writeShellScript "va-plus-stop" ''
    set +e
    password=$(tr -d '\r\n' < ${credentialsDir}/rcon-password)
    ${pkgs.mcrcon}/bin/mcrcon -H 127.0.0.1 -P 25575 -p "$password" \
      "save-all flush" "stop" >/dev/null 2>&1
  '';

  backupScript = pkgs.writeShellScript "va-plus-backup" ''
    set -eu
    timestamp=$(${pkgs.coreutils}/bin/date -u +%Y%m%dT%H%M%SZ)
    destination="${backupDir}/va-plus-$timestamp.tar.zst"
    password=$(tr -d '\r\n' < ${credentialsDir}/rcon-password)
    online=false
    if ${pkgs.mcrcon}/bin/mcrcon -H 127.0.0.1 -P 25575 -p "$password" "save-off" "save-all flush" >/dev/null 2>&1; then
      online=true
    fi
    cleanup() {
      if [ "$online" = true ]; then
        ${pkgs.mcrcon}/bin/mcrcon -H 127.0.0.1 -P 25575 -p "$password" "save-on" >/dev/null 2>&1 || true
      fi
    }
    trap cleanup EXIT
    ${pkgs.gnutar}/bin/tar --ignore-failed-read --zstd -C ${stateDir} -cf "$destination" \
      server/world server/world_nether server/world_the_end server/plugins \
      server/server.properties server/eula.txt integration
    chmod 0640 "$destination"
    find ${backupDir} -type f -name 'va-plus-*.tar.zst' -mtime +${toString cfg.backupRetentionDays} -delete
  '';
in
{
  options.services.vanillaAmericaPlus = {
    enable = lib.mkEnableOption "Vanilla America+ cross-platform server and local operations portal";
    networkScope = lib.mkOption {
      type = lib.types.enum [ "loopback" "lan" ];
      default = "loopback";
      description = "Bind game listeners to loopback, or explicitly expose only game ports to the LAN.";
    };
    memory = lib.mkOption {
      type = lib.types.str;
      default = "8G";
      description = "Maximum Paper JVM heap.";
    };
    backupRetentionDays = lib.mkOption {
      type = lib.types.ints.between 1 365;
      default = 14;
    };
  };

  config = lib.mkIf cfg.enable {
    users.groups.va-plus-data = { };
    users.groups.va-plus-secrets = { };
    users.users.va-plus = {
      isSystemUser = true;
      group = "va-plus-data";
      extraGroups = [ "va-plus-secrets" ];
      home = serverDir;
    };
    users.users.va-plus-portal = {
      isSystemUser = true;
      group = "va-plus-data";
      extraGroups = [ "va-plus-secrets" ];
    };
    users.users.caddy.extraGroups = [ "va-plus-data" ];

    systemd.services.va-plus-credentials = {
      description = "Create VA+ local credentials if absent";
      wantedBy = [ "multi-user.target" ];
      before = [ "vanilla-america-plus.service" "va-plus-portal.service" ];
      serviceConfig.Type = "oneshot";
      serviceConfig.RemainAfterExit = true;
      serviceConfig.ExecStart = credentialsScript;
    };

    systemd.services.va-plus-setup = {
      description = "Install pinned VA+ server artifacts and managed configuration";
      wantedBy = [ "multi-user.target" ];
      before = [ "vanilla-america-plus.service" "va-plus-portal.service" ];
      serviceConfig.Type = "oneshot";
      serviceConfig.RemainAfterExit = true;
      serviceConfig.ExecStart = setupScript;
    };

    systemd.services.vanilla-america-plus = {
      description = "Vanilla America+ Paper 26.2 server";
      wantedBy = [ "multi-user.target" ];
      after = [ "network-online.target" "va-plus-setup.service" "va-plus-credentials.service" ];
      wants = [ "network-online.target" ];
      requires = [ "va-plus-setup.service" "va-plus-credentials.service" ];
      environment = {
        LD_LIBRARY_PATH = lib.makeLibraryPath [ pkgs.stdenv.cc.cc.lib ];
      };
      serviceConfig = {
        Type = "simple";
        User = "va-plus";
        Group = "va-plus-data";
        WorkingDirectory = serverDir;
        UMask = "0007";
        ExecCondition = "${pkgs.gnugrep}/bin/grep -q ^eula=true$ ${serverDir}/eula.txt";
        ExecStartPre = serverProperties;
        ExecStart = "${pkgs.jdk25}/bin/java -Xms2G -Xmx${cfg.memory} -DgeyserUdpAddress=${gameBind} -DgeyserUdpPort=19132 -XX:+UseG1GC -jar ${serverDir}/paper-26.2-121.jar --nogui";
        ExecStop = stopScript;
        TimeoutStopSec = "90s";
        Restart = "on-failure";
        RestartSec = "15s";
        StartLimitIntervalSec = "5min";
        StartLimitBurst = 4;
        LimitNOFILE = 65536;
        MemoryHigh = "10G";
        MemoryMax = "12G";
        NoNewPrivileges = true;
        PrivateTmp = true;
        PrivateDevices = true;
        ProtectSystem = "strict";
        ProtectHome = true;
        ReadWritePaths = [ stateDir ];
        RestrictSUIDSGID = true;
        LockPersonality = true;
        RestrictRealtime = true;
      };
    };

    systemd.services.va-plus-portal = {
      description = "Vanilla America+ portal and least-privilege operations desk";
      wantedBy = [ "multi-user.target" ];
      after = [ "network.target" "va-plus-setup.service" "va-plus-credentials.service" ];
      requires = [ "va-plus-setup.service" "va-plus-credentials.service" ];
      environment = {
        VA_BIND = "127.0.0.1:18080";
        VA_SERVER_ADDRESS = "127.0.0.1:25565";
        VA_DATABASE = "${integrationDir}/va-plus.db";
        VA_SERVER_LOG = "${serverDir}/logs/latest.log";
        VA_BOOTSTRAP_PASSWORD_FILE = "${credentialsDir}/portal-admin-password";
        VA_RCON_PASSWORD_FILE = "${credentialsDir}/rcon-password";
      };
      serviceConfig = {
        User = "va-plus-portal";
        Group = "va-plus-data";
        UMask = "0007";
        ExecStart = "${portal}/bin/portal";
        Restart = "on-failure";
        RestartSec = "5s";
        NoNewPrivileges = true;
        PrivateTmp = true;
        PrivateDevices = true;
        ProtectSystem = "strict";
        ProtectHome = true;
        ReadWritePaths = [ integrationDir ];
        ReadOnlyPaths = [ serverDir credentialsDir ];
        RestrictSUIDSGID = true;
        LockPersonality = true;
        RestrictRealtime = true;
      };
    };

    security.sudo.extraRules = [{
      users = [ "va-plus-portal" ];
      commands = map
        (verb: {
          command = "/run/current-system/sw/bin/systemctl ${verb} vanilla-america-plus.service";
          options = [ "NOPASSWD" ];
        }) [ "start" "stop" "restart" ];
    }];

    services.caddy = {
      enable = true;
      virtualHosts."https://localhost:8444".extraConfig = ''
        bind 127.0.0.1 ::1
        tls internal
        handle_path /map/* {
          root * ${serverDir}/plugins/BlueMap/web
          file_server
        }
        reverse_proxy 127.0.0.1:18080
      '';
    };

    systemd.services.va-plus-backup = {
      description = "Consistent Vanilla America+ backup";
      serviceConfig = {
        Type = "oneshot";
        User = "va-plus";
        Group = "va-plus-data";
        ExecStart = backupScript;
        UMask = "0027";
        NoNewPrivileges = true;
        PrivateTmp = true;
        ProtectSystem = "strict";
        ProtectHome = true;
        ReadWritePaths = [ stateDir ];
      };
    };
    systemd.timers.va-plus-backup = {
      wantedBy = [ "timers.target" ];
      timerConfig = {
        OnCalendar = "daily";
        RandomizedDelaySec = "30min";
        Persistent = true;
      };
    };

    environment.systemPackages = [
      (pkgs.writeShellScriptBin "va-plus-accept-eula" ''
        set -eu
        printf '%s\n' 'Review https://aka.ms/MinecraftEULA before continuing.'
        printf 'Type I-ACCEPT to record acceptance: '
        read -r answer
        [ "$answer" = I-ACCEPT ] || { echo 'EULA not accepted.' >&2; exit 1; }
        printf '%s\n' 'eula=true' | ${pkgs.coreutils}/bin/install -m 0660 -o va-plus -g va-plus-data /dev/stdin ${serverDir}/eula.txt
        /run/current-system/sw/bin/systemctl start vanilla-america-plus.service
      '')
      (pkgs.writeShellScriptBin "va-plus-reset-admin" ''
        exec /run/wrappers/bin/sudo -u va-plus-portal ${portal}/bin/portal reset-admin "$@"
      '')
    ];

    networking.firewall.allowedTCPPorts = lib.mkIf (cfg.networkScope == "lan") [ 25565 ];
    networking.firewall.allowedUDPPorts = lib.mkIf (cfg.networkScope == "lan") [ 19132 ];
  };
}
