package dev.vanillaamericaplus.config;

import org.bukkit.configuration.file.FileConfiguration;

import java.nio.file.Path;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

public record PluginSettings(
        String primary,
        String accent,
        String ivory,
        String javaAddress,
        String bedrockAddress,
        int reportCooldownSeconds,
        int maxReasonLength,
        int reportPageSize,
        Path databasePath,
        int entitlementPollSeconds,
        Map<String, String> entitlements,
        boolean celebrationEnabled,
        int celebrationCooldownSeconds,
        int celebrationRadius,
        int factIntervalMinutes,
        List<String> facts
) {
    public static PluginSettings load(FileConfiguration config, Path dataDirectory) {
        int cooldown = bounded(config.getInt("reports.cooldown-seconds", 300), 10, 86400, "reports.cooldown-seconds");
        int reasonLength = bounded(config.getInt("reports.max-reason-length", 240), 32, 1000, "reports.max-reason-length");
        int pageSize = bounded(config.getInt("reports.page-size", 8), 1, 20, "reports.page-size");
        int poll = bounded(config.getInt("entitlements.poll-seconds", 10), 5, 3600, "entitlements.poll-seconds");
        int celebrationCooldown = bounded(config.getInt("celebration.cooldown-seconds", 900), 30, 86400, "celebration.cooldown-seconds");
        int radius = bounded(config.getInt("celebration.radius", 32), 4, 96, "celebration.radius");
        int factInterval = bounded(config.getInt("facts.interval-minutes", 20), 5, 1440, "facts.interval-minutes");

        String configuredPath = required(config.getString("database.path", "integration.db"), "database.path");
        Path database = Path.of(configuredPath);
        if (!database.isAbsolute()) {
            database = dataDirectory.resolve(database).normalize();
        }

        Map<String, String> allowed = new LinkedHashMap<>();
        var section = config.getConfigurationSection("entitlements.allowed");
        if (section != null) {
            for (String code : section.getKeys(false)) {
                if (!code.matches("[a-z0-9_]{1,48}")) {
                    throw new IllegalArgumentException("Invalid entitlement code: " + code);
                }
                String permission = required(section.getString(code), "entitlements.allowed." + code);
                if (!permission.matches("[a-z0-9_.-]{3,100}")) {
                    throw new IllegalArgumentException("Invalid permission for entitlement " + code);
                }
                allowed.put(code, permission);
            }
        }
        if (allowed.isEmpty()) {
            throw new IllegalArgumentException("At least one entitlements.allowed mapping is required");
        }

        List<String> facts = config.getStringList("facts.entries").stream()
                .map(String::trim)
                .filter(value -> !value.isEmpty() && value.length() <= 280)
                .toList();

        return new PluginSettings(
                color(config, "theme.primary", "#0B1F3A"),
                color(config, "theme.accent", "#B22234"),
                color(config, "theme.ivory", "#F5F0E6"),
                required(config.getString("connection.java"), "connection.java"),
                required(config.getString("connection.bedrock"), "connection.bedrock"),
                cooldown,
                reasonLength,
                pageSize,
                database,
                poll,
                Map.copyOf(allowed),
                config.getBoolean("celebration.enabled", true),
                celebrationCooldown,
                radius,
                factInterval,
                facts
        );
    }

    private static String color(FileConfiguration config, String path, String fallback) {
        String value = config.getString(path, fallback);
        if (value == null || !value.matches("#[0-9A-Fa-f]{6}")) {
            throw new IllegalArgumentException(path + " must be a six-digit hex color");
        }
        return value;
    }

    private static String required(String value, String path) {
        if (value == null || value.isBlank() || value.length() > 256) {
            throw new IllegalArgumentException(path + " must be non-empty and at most 256 characters");
        }
        return value.trim();
    }

    private static int bounded(int value, int minimum, int maximum, String path) {
        if (value < minimum || value > maximum) {
            throw new IllegalArgumentException(path + " must be between " + minimum + " and " + maximum);
        }
        return value;
    }
}

