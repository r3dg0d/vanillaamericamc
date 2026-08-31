package dev.vanillaamericaplus;

import dev.vanillaamericaplus.command.GuideCommand;
import dev.vanillaamericaplus.command.LibertyBellCommand;
import dev.vanillaamericaplus.command.ReportCommand;
import dev.vanillaamericaplus.command.ReportsCommand;
import dev.vanillaamericaplus.config.PluginSettings;
import dev.vanillaamericaplus.entitlement.EntitlementService;
import dev.vanillaamericaplus.listener.PlayerListener;
import dev.vanillaamericaplus.message.MessageBundle;
import dev.vanillaamericaplus.persistence.Database;
import dev.vanillaamericaplus.report.ReportService;
import org.bukkit.Bukkit;
import org.bukkit.command.PluginCommand;
import org.bukkit.plugin.java.JavaPlugin;
import org.bukkit.scheduler.BukkitTask;

import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import java.util.Objects;
import java.util.concurrent.ArrayBlockingQueue;
import java.util.concurrent.ThreadFactory;
import java.util.concurrent.ThreadPoolExecutor;
import java.util.concurrent.TimeUnit;

public final class VanillaAmericaPlusPlugin extends JavaPlugin {
    private PluginSettings settings;
    private MessageBundle messages;
    private ThreadPoolExecutor worker;
    private volatile Database database;
    private volatile ReportService reports;
    private volatile EntitlementService entitlements;
    private final List<BukkitTask> tasks = new ArrayList<>();

    @Override
    public void onEnable() {
        saveDefaultConfig();
        saveResource("messages.yml", false);
        this.settings = PluginSettings.load(getConfig(), getDataFolder().toPath());
        this.messages = MessageBundle.load(this, settings);

        ThreadFactory factory = runnable -> {
            Thread thread = new Thread(runnable, "vaplus-io");
            thread.setDaemon(true);
            thread.setUncaughtExceptionHandler((ignored, error) ->
                    getLogger().severe("event=worker_uncaught type=" + error.getClass().getSimpleName()));
            return thread;
        };
        this.worker = new ThreadPoolExecutor(
                1, 1, 0L, TimeUnit.MILLISECONDS,
                new ArrayBlockingQueue<>(256), factory, new ThreadPoolExecutor.AbortPolicy()
        );

        registerCommands();
        Bukkit.getPluginManager().registerEvents(new PlayerListener(this), this);

        // Local schema work and driver initialization stay off the server thread.
        worker.execute(() -> {
            try {
                Database opened = new Database(this, settings.databasePath());
                ReportService reportService = new ReportService(this, opened, worker);
                EntitlementService entitlementService =
                        new EntitlementService(this, opened, worker, settings.entitlements());
                this.database = opened;
                this.reports = reportService;
                this.entitlements = entitlementService;
                Bukkit.getScheduler().runTask(this, this::startScheduledTasks);
                getLogger().info("event=plugin_ready version=" + getPluginMeta().getVersion());
            } catch (Exception exception) {
                getLogger().severe("event=database_initialization_failed type="
                        + exception.getClass().getSimpleName());
                Bukkit.getScheduler().runTask(this, () ->
                        Bukkit.getPluginManager().disablePlugin(this));
            }
        });
    }

    @Override
    public void onDisable() {
        tasks.forEach(BukkitTask::cancel);
        tasks.clear();
        if (worker != null) {
            worker.shutdown();
            try {
                if (!worker.awaitTermination(Duration.ofSeconds(5).toMillis(), TimeUnit.MILLISECONDS)) {
                    worker.shutdownNow();
                }
            } catch (InterruptedException exception) {
                Thread.currentThread().interrupt();
                worker.shutdownNow();
            }
        }
        if (database != null) database.close();
        getLogger().info("event=plugin_stopped");
    }

    public void reloadValidatedConfiguration() {
        reloadConfig();
        PluginSettings replacement = PluginSettings.load(getConfig(), getDataFolder().toPath());
        if (!replacement.databasePath().equals(settings.databasePath())) {
            throw new IllegalArgumentException("database.path requires a full server restart");
        }
        MessageBundle replacementMessages = MessageBundle.load(this, replacement);
        this.settings = replacement;
        this.messages = replacementMessages;
        if (entitlements != null) entitlements.replaceAllowlist(replacement.entitlements());
    }

    private void registerCommands() {
        executor("va", new GuideCommand(this));
        executor("rules", new GuideCommand(this));
        executor("report", new ReportCommand(this));
        executor("reports", new ReportsCommand(this));
        executor("libertybell", new LibertyBellCommand(this));
    }

    private void executor(String commandName, org.bukkit.command.CommandExecutor executor) {
        PluginCommand command = Objects.requireNonNull(getCommand(commandName), "Missing command " + commandName);
        command.setExecutor(executor);
    }

    private void startScheduledTasks() {
        if (entitlements != null) {
            long ticks = settings.entitlementPollSeconds() * 20L;
            tasks.add(Bukkit.getScheduler().runTaskTimer(this, entitlements, ticks, ticks));
        }
        if (!settings.facts().isEmpty()) {
            long ticks = settings.factIntervalMinutes() * 60L * 20L;
            tasks.add(Bukkit.getScheduler().runTaskTimer(this, new Runnable() {
                private int index;

                @Override
                public void run() {
                    String fact = settings.facts().get(index++ % settings.facts().size());
                    Bukkit.broadcast(messages.plain("VA+ Field Note: " + fact));
                }
            }, ticks, ticks));
        }
    }

    public PluginSettings settings() {
        return settings;
    }

    public MessageBundle messages() {
        return messages;
    }

    public boolean reportsReady() {
        return reports != null;
    }

    public ReportService reports() {
        ReportService current = reports;
        if (current == null) throw new IllegalStateException("Report service is not ready");
        return current;
    }
}

