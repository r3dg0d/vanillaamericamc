package dev.vanillaamericaplus.entitlement;

import dev.vanillaamericaplus.persistence.Database;
import net.luckperms.api.LuckPerms;
import net.luckperms.api.node.Node;
import org.bukkit.Bukkit;
import org.bukkit.plugin.RegisteredServiceProvider;
import org.bukkit.plugin.java.JavaPlugin;

import java.sql.SQLException;
import java.util.Map;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.ThreadPoolExecutor;

public final class EntitlementService implements Runnable {
    private final JavaPlugin plugin;
    private final Database database;
    private final ThreadPoolExecutor worker;
    private volatile Map<String, String> allowlist;

    public EntitlementService(
            JavaPlugin plugin,
            Database database,
            ThreadPoolExecutor worker,
            Map<String, String> allowlist
    ) {
        this.plugin = plugin;
        this.database = database;
        this.worker = worker;
        this.allowlist = Map.copyOf(allowlist);
    }

    public void replaceAllowlist(Map<String, String> replacement) {
        this.allowlist = Map.copyOf(replacement);
    }

    @Override
    public void run() {
        if (worker.getQueue().remainingCapacity() == 0) return;
        worker.execute(() -> {
            try {
                for (Database.PendingEntitlement pending : database.claimPendingEntitlements(16)) {
                    apply(pending);
                }
            } catch (SQLException exception) {
                plugin.getLogger().severe("event=entitlement_poll_failed type=" + exception.getClass().getSimpleName());
            }
        });
    }

    private void apply(Database.PendingEntitlement pending) {
        String permission = allowlist.get(pending.code());
        if (permission == null) {
            finish(pending, false, "code_not_allowlisted");
            return;
        }

        RegisteredServiceProvider<LuckPerms> registration =
                Bukkit.getServicesManager().getRegistration(LuckPerms.class);
        if (registration == null) {
            finish(pending, false, "luckperms_unavailable");
            return;
        }

        LuckPerms luckPerms = registration.getProvider();
        luckPerms.getUserManager().loadUser(pending.playerUuid())
                .thenCompose(user -> {
                    user.data().add(Node.builder(permission).value(true).build());
                    return luckPerms.getUserManager().saveUser(user);
                })
                .whenComplete((ignored, error) -> {
                    if (error != null) {
                        finish(pending, false, "permission_apply_failed");
                    } else {
                        finish(pending, true, "permission_applied");
                    }
                });
    }

    private void finish(Database.PendingEntitlement pending, boolean applied, String detail) {
        CompletableFuture.runAsync(() -> {
            try {
                database.finishEntitlement(pending, applied, detail);
                database.audit("entitlement-worker", "entitlement." + detail, pending.orderId(),
                        applied ? "success" : "rejected");
                plugin.getLogger().info("event=entitlement_finished order_id=" + pending.orderId()
                        + " code=" + pending.code() + " applied=" + applied);
            } catch (SQLException exception) {
                plugin.getLogger().severe("event=entitlement_finish_failed order_id=" + pending.orderId());
            }
        }, worker);
    }
}

