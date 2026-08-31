package dev.vanillaamericaplus.command;

import dev.vanillaamericaplus.VanillaAmericaPlusPlugin;
import dev.vanillaamericaplus.message.MessageBundle;
import dev.vanillaamericaplus.report.ReportRecord;
import dev.vanillaamericaplus.util.InputValidator;
import net.kyori.adventure.text.Component;
import org.bukkit.Bukkit;
import org.bukkit.command.Command;
import org.bukkit.command.CommandExecutor;
import org.bukkit.command.CommandSender;
import org.bukkit.entity.Player;
import org.jetbrains.annotations.NotNull;

import java.time.Duration;
import java.time.Instant;
import java.util.Arrays;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.ConcurrentHashMap;

public final class ReportCommand implements CommandExecutor {
    private final VanillaAmericaPlusPlugin plugin;
    private final Map<UUID, Instant> cooldowns = new ConcurrentHashMap<>();

    public ReportCommand(VanillaAmericaPlusPlugin plugin) {
        this.plugin = plugin;
    }

    @Override
    public boolean onCommand(
            @NotNull CommandSender sender,
            @NotNull Command command,
            @NotNull String label,
            @NotNull String[] args
    ) {
        if (args.length >= 2 && InputValidator.staffAction(args[0]).filter(action ->
                action.equals("resolve") || action.equals("dismiss")).isPresent()) {
            return closeReport(sender, args);
        }
        if (args.length >= 2 && "view".equalsIgnoreCase(args[0])) {
            return viewReport(sender, args[1]);
        }
        if (!plugin.reportsReady()) {
            sender.sendMessage(plugin.messages().get("report-busy"));
            return true;
        }
        if (!(sender instanceof Player player)) {
            sender.sendMessage(plugin.messages().get("report-invalid"));
            return true;
        }
        return createReport(player, args);
    }

    private boolean createReport(Player player, String[] args) {
        if (args.length < 2) {
            player.sendMessage(plugin.messages().get("report-invalid"));
            return true;
        }
        var target = InputValidator.playerName(args[0]);
        var reason = InputValidator.reason(
                String.join(" ", Arrays.copyOfRange(args, 1, args.length)),
                plugin.settings().maxReasonLength()
        );
        if (target.isEmpty() || reason.isEmpty() || target.get().equalsIgnoreCase(player.getName())) {
            player.sendMessage(plugin.messages().get("report-invalid"));
            return true;
        }

        Instant now = Instant.now();
        Instant next = cooldowns.getOrDefault(player.getUniqueId(), Instant.EPOCH)
                .plusSeconds(plugin.settings().reportCooldownSeconds());
        if (next.isAfter(now)) {
            long remaining = Math.max(1, Duration.between(now, next).toSeconds());
            player.sendMessage(plugin.messages().get("report-cooldown",
                    MessageBundle.value("seconds", Long.toString(remaining))));
            return true;
        }

        cooldowns.put(player.getUniqueId(), now);
        plugin.reports().create(player.getUniqueId(), player.getName(), target.get(), reason.get())
                .whenComplete((report, error) -> Bukkit.getScheduler().runTask(plugin, () -> {
                    if (error != null) {
                        cooldowns.remove(player.getUniqueId(), now);
                        player.sendMessage(plugin.messages().get("report-busy"));
                        return;
                    }
                    player.sendMessage(plugin.messages().get("report-created",
                            MessageBundle.value("id", report.id().toString())));
                    Component notice = Component.text("New VA+ report " + report.id()
                            + " involving " + report.targetName() + ". Use /report view " + report.id());
                    Bukkit.getOnlinePlayers().stream()
                            .filter(staff -> staff.hasPermission("vaplus.staff.reports"))
                            .forEach(staff -> staff.sendMessage(notice));
                }));
        return true;
    }

    private boolean viewReport(CommandSender sender, String rawId) {
        if (!plugin.reportsReady()) {
            sender.sendMessage(plugin.messages().get("report-busy"));
            return true;
        }
        if (!sender.hasPermission("vaplus.staff.reports")) {
            sender.sendMessage(plugin.messages().get("no-permission"));
            return true;
        }
        var id = InputValidator.reportId(rawId);
        if (id.isEmpty()) {
            sender.sendMessage(plugin.messages().get("report-not-found"));
            return true;
        }
        plugin.reports().find(UUID.fromString(id.get()))
                .whenComplete((report, error) -> Bukkit.getScheduler().runTask(plugin, () -> {
                    if (error != null || report.isEmpty()) {
                        sender.sendMessage(plugin.messages().get("report-not-found"));
                        return;
                    }
                    ReportRecord value = report.get();
                    sender.sendMessage(Component.text("Report " + value.id()
                            + " [" + value.status() + "] "
                            + value.reporterName() + " -> " + value.targetName()
                            + ": " + value.reason()));
                }));
        return true;
    }

    private boolean closeReport(CommandSender sender, String[] args) {
        if (!plugin.reportsReady()) {
            sender.sendMessage(plugin.messages().get("report-busy"));
            return true;
        }
        if (!sender.hasPermission("vaplus.staff.resolve")) {
            sender.sendMessage(plugin.messages().get("no-permission"));
            return true;
        }
        var action = InputValidator.staffAction(args[0]);
        var id = InputValidator.reportId(args[1]);
        String noteRaw = args.length > 2 ? String.join(" ", Arrays.copyOfRange(args, 2, args.length)) : "";
        var note = InputValidator.note(noteRaw, plugin.settings().maxReasonLength());
        if (action.isEmpty() || id.isEmpty() || note.isEmpty()) {
            sender.sendMessage(plugin.messages().get("report-invalid"));
            return true;
        }
        plugin.reports().close(UUID.fromString(id.get()), action.get(), sender.getName(), note.get())
                .whenComplete((changed, error) -> Bukkit.getScheduler().runTask(plugin, () -> {
                    if (error != null || !changed) {
                        sender.sendMessage(plugin.messages().get("report-not-found"));
                    } else {
                        sender.sendMessage(plugin.messages().get("report-updated",
                                MessageBundle.value("id", id.get()),
                                MessageBundle.value("status", action.get())));
                    }
                }));
        return true;
    }
}

