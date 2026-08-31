package dev.vanillaamericaplus.listener;

import dev.vanillaamericaplus.VanillaAmericaPlusPlugin;
import dev.vanillaamericaplus.message.MessageBundle;
import org.bukkit.Bukkit;
import org.bukkit.event.EventHandler;
import org.bukkit.event.Listener;
import org.bukkit.event.player.PlayerJoinEvent;
import org.geysermc.floodgate.api.FloodgateApi;

public final class PlayerListener implements Listener {
    private final VanillaAmericaPlusPlugin plugin;

    public PlayerListener(VanillaAmericaPlusPlugin plugin) {
        this.plugin = plugin;
    }

    @EventHandler
    public void onJoin(PlayerJoinEvent event) {
        boolean bedrock = false;
        if (Bukkit.getPluginManager().isPluginEnabled("floodgate")) {
            try {
                bedrock = FloodgateApi.getInstance().isFloodgatePlayer(event.getPlayer().getUniqueId());
            } catch (RuntimeException | LinkageError error) {
                plugin.getLogger().warning("event=floodgate_detection_failed type="
                        + error.getClass().getSimpleName());
            }
        }
        event.getPlayer().sendMessage(plugin.messages().get(
                bedrock ? "welcome-bedrock" : "welcome-java",
                MessageBundle.value("player", event.getPlayer().getName())
        ));
    }
}

