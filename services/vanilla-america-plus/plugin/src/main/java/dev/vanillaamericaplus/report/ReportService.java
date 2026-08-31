package dev.vanillaamericaplus.report;

import dev.vanillaamericaplus.persistence.Database;
import org.bukkit.plugin.java.JavaPlugin;

import java.sql.SQLException;
import java.util.List;
import java.util.Optional;
import java.util.UUID;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.RejectedExecutionException;
import java.util.concurrent.ThreadPoolExecutor;

public final class ReportService {
    private final JavaPlugin plugin;
    private final Database database;
    private final ThreadPoolExecutor worker;

    public ReportService(JavaPlugin plugin, Database database, ThreadPoolExecutor worker) {
        this.plugin = plugin;
        this.database = database;
        this.worker = worker;
    }

    public CompletableFuture<ReportRecord> create(
            UUID reporterUuid, String reporterName, String targetName, String reason
    ) {
        return submit(() -> {
            ReportRecord report = database.createReport(reporterUuid, reporterName, targetName, reason);
            plugin.getLogger().info("event=report_created id=" + report.id() + " reporter_uuid=" + reporterUuid);
            return report;
        });
    }

    public CompletableFuture<List<ReportRecord>> listOpen(int page, int pageSize) {
        int safePage = Math.max(1, page);
        return submit(() -> database.listOpen(pageSize, (safePage - 1) * pageSize));
    }

    public CompletableFuture<Optional<ReportRecord>> find(UUID id) {
        return submit(() -> database.findReport(id));
    }

    public CompletableFuture<Boolean> close(UUID id, String status, String actor, String note) {
        return submit(() -> {
            boolean changed = database.closeReport(id, status, actor, note);
            plugin.getLogger().info("event=report_" + status + " id=" + id + " changed=" + changed);
            return changed;
        });
    }

    private <T> CompletableFuture<T> submit(SqlSupplier<T> operation) {
        CompletableFuture<T> future = new CompletableFuture<>();
        try {
            worker.execute(() -> {
                try {
                    future.complete(operation.get());
                } catch (Exception exception) {
                    plugin.getLogger().severe("event=database_operation_failed type=" + exception.getClass().getSimpleName());
                    future.completeExceptionally(exception);
                }
            });
        } catch (RejectedExecutionException exception) {
            future.completeExceptionally(exception);
        }
        return future;
    }

    @FunctionalInterface
    private interface SqlSupplier<T> {
        T get() throws SQLException;
    }
}

