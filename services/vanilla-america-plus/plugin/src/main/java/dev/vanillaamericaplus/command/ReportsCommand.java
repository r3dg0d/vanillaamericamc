package dev.vanillaamericaplus.command;

import dev.vanillaamericaplus.VanillaAmericaPlusPlugin;
import net.kyori.adventure.text.Component;
import org.bukkit.Bukkit;
import org.bukkit.command.Command;
import org.bukkit.command.CommandExecutor;
import org.bukkit.command.CommandSender;
import org.jetbrains.annotations.NotNull;

public final class ReportsCommand implements CommandExecutor {
    private final VanillaAmericaPlusPlugin plugin;

    public ReportsCommand(VanillaAmericaPlusPlugin plugin) {
        this.plugin = plugin;
    }

    @Override
    public boolean onCommand(
            @NotNull CommandSender sender,
            @NotNull Command command,
            @NotNull String label,
            @NotNull String[] args
    ) {
        if (!plugin.reportsReady()) {
            sender.sendMessage(plugin.messages().get("report-busy"));
            return true;
        }
        if (!sender.hasPermission("vaplus.staff.reports")) {
            sender.sendMessage(plugin.messages().get("no-permission"));
            return true;
        }
        int page = 1;
        if (args.length == 1) {
            try {
                page = Math.max(1, Integer.parseInt(args[0]));
            } catch (NumberFormatException ignored) {
                sender.sendMessage(Component.text("Page must be a positive number."));
                return true;
            }
        }
        int selectedPage = page;
        plugin.reports().listOpen(page, plugin.settings().reportPageSize())
                .whenComplete((reports, error) -> Bukkit.getScheduler().runTask(plugin, () -> {
                    if (error != null) {
                        sender.sendMessage(plugin.messages().get("report-busy"));
                        return;
                    }
                    sender.sendMessage(Component.text("Open reports — page " + selectedPage));
                    if (reports.isEmpty()) {
                        sender.sendMessage(Component.text("No open reports on this page."));
                    } else {
                        reports.forEach(report -> sender.sendMessage(Component.text(
                                report.id() + " | " + report.targetName() + " | " + report.createdAt()
                        )));
                    }
                }));
        return true;
    }
}

