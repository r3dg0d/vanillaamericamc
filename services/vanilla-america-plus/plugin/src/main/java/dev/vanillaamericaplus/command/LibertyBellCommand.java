package dev.vanillaamericaplus.command;

import dev.vanillaamericaplus.VanillaAmericaPlusPlugin;
import dev.vanillaamericaplus.message.MessageBundle;
import org.bukkit.Bukkit;
import org.bukkit.Particle;
import org.bukkit.Sound;
import org.bukkit.command.Command;
import org.bukkit.command.CommandExecutor;
import org.bukkit.command.CommandSender;
import org.bukkit.entity.Player;
import org.jetbrains.annotations.NotNull;

import java.time.Duration;
import java.time.Instant;

public final class LibertyBellCommand implements CommandExecutor {
    private final VanillaAmericaPlusPlugin plugin;
    private Instant nextAllowed = Instant.EPOCH;

    public LibertyBellCommand(VanillaAmericaPlusPlugin plugin) {
        this.plugin = plugin;
    }

    @Override
    public boolean onCommand(
            @NotNull CommandSender sender,
            @NotNull Command command,
            @NotNull String label,
            @NotNull String[] args
    ) {
        if (!plugin.settings().celebrationEnabled()) {
            sender.sendMessage(plugin.messages().plain("The Liberty Bell celebration is disabled."));
            return true;
        }
        Instant now = Instant.now();
        if (nextAllowed.isAfter(now)) {
            sender.sendMessage(plugin.messages().get("celebration-cooldown",
                    MessageBundle.value("seconds",
                            Long.toString(Math.max(1, Duration.between(now, nextAllowed).toSeconds())))));
            return true;
        }
        nextAllowed = now.plusSeconds(plugin.settings().celebrationCooldownSeconds());
        Bukkit.broadcast(plugin.messages().get("celebration"));

        int radiusSquared = plugin.settings().celebrationRadius() * plugin.settings().celebrationRadius();
        for (Player player : Bukkit.getOnlinePlayers()) {
            player.playSound(player.getLocation(), Sound.BLOCK_BELL_USE, 0.8f, 1.0f);
            player.getWorld().spawnParticle(
                    Particle.FIREWORK,
                    player.getLocation().add(0, 1.4, 0),
                    12,
                    0.8, 0.5, 0.8,
                    0.02
            );
        }
        plugin.getLogger().info("event=liberty_bell actor=" + sender.getName()
                + " audience=" + Bukkit.getOnlinePlayers().size()
                + " radius_squared=" + radiusSquared);
        return true;
    }
}

