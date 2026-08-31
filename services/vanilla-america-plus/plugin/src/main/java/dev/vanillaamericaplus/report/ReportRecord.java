package dev.vanillaamericaplus.report;

import java.time.Instant;
import java.util.UUID;

public record ReportRecord(
        UUID id,
        UUID reporterUuid,
        String reporterName,
        String targetName,
        String reason,
        String status,
        Instant createdAt,
        String resolvedBy,
        String resolutionNote
) {
}

