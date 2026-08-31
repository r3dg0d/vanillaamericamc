package dev.vanillaamericaplus.util;

import java.util.Locale;
import java.util.Optional;
import java.util.Set;
import java.util.regex.Pattern;

public final class InputValidator {
    private static final Pattern PLAYER = Pattern.compile("[.+_A-Za-z0-9 -]{1,32}");
    private static final Pattern UUID_TEXT = Pattern.compile("[0-9a-fA-F-]{32,36}");
    private static final Set<String> STAFF_ACTIONS = Set.of("resolve", "dismiss", "warn", "mute", "kick", "tempban");

    private InputValidator() {
    }

    public static Optional<String> playerName(String input) {
        if (input == null) return Optional.empty();
        String normalized = input.trim();
        return PLAYER.matcher(normalized).matches() ? Optional.of(normalized) : Optional.empty();
    }

    public static Optional<String> reason(String input, int maximumLength) {
        if (input == null) return Optional.empty();
        String normalized = input.replaceAll("\\s+", " ").trim();
        if (normalized.length() < 4 || normalized.length() > maximumLength || containsControl(normalized)) {
            return Optional.empty();
        }
        return Optional.of(normalized);
    }

    public static Optional<String> note(String input, int maximumLength) {
        if (input == null || input.isBlank()) return Optional.of("");
        return reason(input, maximumLength);
    }

    public static Optional<String> reportId(String input) {
        if (input == null || !UUID_TEXT.matcher(input).matches()) return Optional.empty();
        try {
            return Optional.of(java.util.UUID.fromString(input).toString());
        } catch (IllegalArgumentException ignored) {
            return Optional.empty();
        }
    }

    public static Optional<String> staffAction(String input) {
        if (input == null) return Optional.empty();
        String action = input.toLowerCase(Locale.ROOT);
        return STAFF_ACTIONS.contains(action) ? Optional.of(action) : Optional.empty();
    }

    private static boolean containsControl(String value) {
        return value.codePoints().anyMatch(codePoint ->
                Character.isISOControl(codePoint) && codePoint != '\n' && codePoint != '\t'
        );
    }
}

