package dev.vanillaamericaplus.persistence;

import dev.vanillaamericaplus.report.ReportRecord;
import org.bukkit.plugin.java.JavaPlugin;
import org.sqlite.SQLiteConfig;
import org.sqlite.SQLiteDataSource;

import java.nio.file.Files;
import java.nio.file.Path;
import java.sql.Connection;
import java.sql.PreparedStatement;
import java.sql.ResultSet;
import java.sql.SQLException;
import java.sql.Statement;
import java.time.Instant;
import java.util.ArrayList;
import java.util.List;
import java.util.Optional;
import java.util.UUID;

public final class Database implements AutoCloseable {
    private final SQLiteDataSource dataSource;

    public Database(JavaPlugin plugin, Path databasePath) throws Exception {
        Path parent = databasePath.toAbsolutePath().getParent();
        if (parent != null) Files.createDirectories(parent);
        SQLiteConfig config = new SQLiteConfig();
        config.setBusyTimeout(5000);
        config.setJournalMode(SQLiteConfig.JournalMode.WAL);
        config.enforceForeignKeys(true);
        this.dataSource = new SQLiteDataSource(config);
        this.dataSource.setUrl("jdbc:sqlite:" + databasePath.toAbsolutePath());
        migrate();
        plugin.getLogger().info("event=database_ready path=" + databasePath.toAbsolutePath());
    }

    private void migrate() throws SQLException {
        try (Connection connection = dataSource.getConnection(); Statement statement = connection.createStatement()) {
            statement.executeUpdate("""
                    CREATE TABLE IF NOT EXISTS reports (
                        id TEXT PRIMARY KEY,
                        reporter_uuid TEXT NOT NULL,
                        reporter_name TEXT NOT NULL,
                        target_name TEXT NOT NULL,
                        reason TEXT NOT NULL,
                        status TEXT NOT NULL CHECK(status IN ('open','resolved','dismissed')),
                        created_at TEXT NOT NULL,
                        resolved_by TEXT,
                        resolution_note TEXT
                    )
                    """);
            statement.executeUpdate("""
                    CREATE TABLE IF NOT EXISTS staff_audit (
                        id INTEGER PRIMARY KEY AUTOINCREMENT,
                        actor TEXT NOT NULL,
                        action TEXT NOT NULL,
                        target TEXT NOT NULL,
                        outcome TEXT NOT NULL,
                        created_at TEXT NOT NULL
                    )
                    """);
            statement.executeUpdate("""
                    CREATE TRIGGER IF NOT EXISTS staff_audit_no_update
                    BEFORE UPDATE ON staff_audit BEGIN
                        SELECT RAISE(ABORT, 'staff_audit is immutable');
                    END
                    """);
            statement.executeUpdate("""
                    CREATE TRIGGER IF NOT EXISTS staff_audit_no_delete
                    BEFORE DELETE ON staff_audit BEGIN
                        SELECT RAISE(ABORT, 'staff_audit is immutable');
                    END
                    """);
            statement.executeUpdate("""
                    CREATE TABLE IF NOT EXISTS entitlements (
                        order_id TEXT NOT NULL,
                        player_uuid TEXT NOT NULL,
                        entitlement_code TEXT NOT NULL,
                        state TEXT NOT NULL CHECK(state IN ('pending','processing','applied','rejected')),
                        detail TEXT,
                        created_at TEXT NOT NULL,
                        applied_at TEXT,
                        PRIMARY KEY(order_id, entitlement_code)
                    )
                    """);
            statement.executeUpdate("CREATE INDEX IF NOT EXISTS reports_status_created ON reports(status, created_at)");
            statement.executeUpdate("CREATE INDEX IF NOT EXISTS entitlements_state_created ON entitlements(state, created_at)");
        }
    }

    public ReportRecord createReport(UUID reporterUuid, String reporterName, String targetName, String reason) throws SQLException {
        ReportRecord record = new ReportRecord(
                UUID.randomUUID(), reporterUuid, reporterName, targetName, reason,
                "open", Instant.now(), null, null
        );
        try (Connection connection = dataSource.getConnection();
             PreparedStatement statement = connection.prepareStatement("""
                     INSERT INTO reports(id, reporter_uuid, reporter_name, target_name, reason, status, created_at)
                     VALUES (?, ?, ?, ?, ?, ?, ?)
                     """)) {
            statement.setString(1, record.id().toString());
            statement.setString(2, reporterUuid.toString());
            statement.setString(3, reporterName);
            statement.setString(4, targetName);
            statement.setString(5, reason);
            statement.setString(6, record.status());
            statement.setString(7, record.createdAt().toString());
            statement.executeUpdate();
        }
        return record;
    }

    public List<ReportRecord> listOpen(int limit, int offset) throws SQLException {
        List<ReportRecord> records = new ArrayList<>();
        try (Connection connection = dataSource.getConnection();
             PreparedStatement statement = connection.prepareStatement("""
                     SELECT * FROM reports WHERE status = 'open'
                     ORDER BY created_at ASC LIMIT ? OFFSET ?
                     """)) {
            statement.setInt(1, limit);
            statement.setInt(2, offset);
            try (ResultSet result = statement.executeQuery()) {
                while (result.next()) records.add(readReport(result));
            }
        }
        return records;
    }

    public Optional<ReportRecord> findReport(UUID id) throws SQLException {
        try (Connection connection = dataSource.getConnection();
             PreparedStatement statement = connection.prepareStatement("SELECT * FROM reports WHERE id = ?")) {
            statement.setString(1, id.toString());
            try (ResultSet result = statement.executeQuery()) {
                return result.next() ? Optional.of(readReport(result)) : Optional.empty();
            }
        }
    }

    public boolean closeReport(UUID id, String status, String actor, String note) throws SQLException {
        if (!status.equals("resolved") && !status.equals("dismissed")) {
            throw new IllegalArgumentException("Unsupported report status");
        }
        try (Connection connection = dataSource.getConnection()) {
            connection.setAutoCommit(false);
            try (PreparedStatement update = connection.prepareStatement("""
                    UPDATE reports SET status = ?, resolved_by = ?, resolution_note = ?
                    WHERE id = ? AND status = 'open'
                    """);
                 PreparedStatement audit = connection.prepareStatement("""
                    INSERT INTO staff_audit(actor, action, target, outcome, created_at)
                    VALUES (?, ?, ?, ?, ?)
                    """)) {
                update.setString(1, status);
                update.setString(2, actor);
                update.setString(3, note);
                update.setString(4, id.toString());
                int changed = update.executeUpdate();

                audit.setString(1, actor);
                audit.setString(2, "report." + status);
                audit.setString(3, id.toString());
                audit.setString(4, changed == 1 ? "success" : "not_open");
                audit.setString(5, Instant.now().toString());
                audit.executeUpdate();
                connection.commit();
                return changed == 1;
            } catch (SQLException exception) {
                connection.rollback();
                throw exception;
            }
        }
    }

    public void audit(String actor, String action, String target, String outcome) throws SQLException {
        try (Connection connection = dataSource.getConnection();
             PreparedStatement statement = connection.prepareStatement("""
                     INSERT INTO staff_audit(actor, action, target, outcome, created_at)
                     VALUES (?, ?, ?, ?, ?)
                     """)) {
            statement.setString(1, actor);
            statement.setString(2, action);
            statement.setString(3, target);
            statement.setString(4, outcome);
            statement.setString(5, Instant.now().toString());
            statement.executeUpdate();
        }
    }

    public List<PendingEntitlement> claimPendingEntitlements(int limit) throws SQLException {
        List<PendingEntitlement> claimed = new ArrayList<>();
        try (Connection connection = dataSource.getConnection()) {
            connection.setAutoCommit(false);
            try (PreparedStatement select = connection.prepareStatement("""
                    SELECT order_id, player_uuid, entitlement_code FROM entitlements
                    WHERE state = 'pending' ORDER BY created_at ASC LIMIT ?
                    """)) {
                select.setInt(1, limit);
                try (ResultSet result = select.executeQuery()) {
                    while (result.next()) {
                        claimed.add(new PendingEntitlement(
                                result.getString("order_id"),
                                UUID.fromString(result.getString("player_uuid")),
                                result.getString("entitlement_code")
                        ));
                    }
                }
            }
            try (PreparedStatement update = connection.prepareStatement("""
                    UPDATE entitlements SET state = 'processing'
                    WHERE order_id = ? AND entitlement_code = ? AND state = 'pending'
                    """)) {
                claimed.removeIf(item -> {
                    try {
                        update.setString(1, item.orderId());
                        update.setString(2, item.code());
                        return update.executeUpdate() != 1;
                    } catch (SQLException exception) {
                        throw new DatabaseRuntimeException(exception);
                    }
                });
            } catch (DatabaseRuntimeException exception) {
                connection.rollback();
                throw exception.cause;
            }
            connection.commit();
        }
        return claimed;
    }

    public void finishEntitlement(PendingEntitlement item, boolean applied, String detail) throws SQLException {
        String safeDetail = detail == null ? "" : detail.substring(0, Math.min(detail.length(), 240));
        try (Connection connection = dataSource.getConnection();
             PreparedStatement statement = connection.prepareStatement("""
                     UPDATE entitlements SET state = ?, detail = ?, applied_at = ?
                     WHERE order_id = ? AND entitlement_code = ? AND state = 'processing'
                     """)) {
            statement.setString(1, applied ? "applied" : "rejected");
            statement.setString(2, safeDetail);
            statement.setString(3, Instant.now().toString());
            statement.setString(4, item.orderId());
            statement.setString(5, item.code());
            statement.executeUpdate();
        }
    }

    private ReportRecord readReport(ResultSet result) throws SQLException {
        return new ReportRecord(
                UUID.fromString(result.getString("id")),
                UUID.fromString(result.getString("reporter_uuid")),
                result.getString("reporter_name"),
                result.getString("target_name"),
                result.getString("reason"),
                result.getString("status"),
                Instant.parse(result.getString("created_at")),
                result.getString("resolved_by"),
                result.getString("resolution_note")
        );
    }

    @Override
    public void close() {
        // SQLiteDataSource owns no persistent connection; worker shutdown happens in the plugin.
    }

    public record PendingEntitlement(String orderId, UUID playerUuid, String code) {
    }

    private static final class DatabaseRuntimeException extends RuntimeException {
        private final SQLException cause;

        private DatabaseRuntimeException(SQLException cause) {
            super(cause);
            this.cause = cause;
        }
    }
}

