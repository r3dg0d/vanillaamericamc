package dev.vanillaamericaplus.command;

import dev.vanillaamericaplus.VanillaAmericaPlusPlugin;
import dev.vanillaamericaplus.message.MessageBundle;
import org.bukkit.command.Command;
import org.bukkit.command.CommandExecutor;
import org.bukkit.command.CommandSender;
import org.jetbrains.annotations.NotNull;

public final class GuideCommand implements CommandExecutor {
    private final VanillaAmericaPlusPlugin plugin;

    public GuideCommand(VanillaAmericaPlusPlugin plugin) {
        this.plugin = plugin;
    }

    @Override
    public boolean onCommand(
            @NotNull CommandSender sender,
            @NotNull Command command,
            @NotNull String label,
            @NotNull String[] args
    ) {
        if ("rules".equalsIgnoreCase(command.getName())) {
            sender.sendMessage(plugin.messages().get("rules"));
            return true;
        }

        if (args.length == 1 && "reload".equalsIgnoreCase(args[0])) {
            if (!sender.hasPermission("vaplus.admin.reload")) {
                sender.sendMessage(plugin.messages().get("no-permission"));
                return true;
            }
            try {
                plugin.reloadValidatedConfiguration();
                sender.sendMessage(plugin.messages().get("reloaded"));
            } catch (IllegalArgumentException exception) {
                sender.sendMessage(plugin.messages().plain("VA+ reload rejected: " + exception.getMessage()));
            }
            return true;
        }

        sender.sendMessage(plugin.messages().get("guide",
                MessageBundle.value("java", plugin.settings().javaAddress()),
                MessageBundle.value("bedrock", plugin.settings().bedrockAddress())));
        return true;
    }
}

