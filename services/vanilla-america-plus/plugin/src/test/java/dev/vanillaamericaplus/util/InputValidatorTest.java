package dev.vanillaamericaplus.util;

import org.junit.jupiter.api.Test;

import java.util.UUID;

import static org.assertj.core.api.Assertions.assertThat;

class InputValidatorTest {
    @Test
    void acceptsJavaAndFloodgateStyleNames() {
        assertThat(InputValidator.playerName("Notch")).contains("Notch");
        assertThat(InputValidator.playerName(".Bedrock User")).contains(".Bedrock User");
        assertThat(InputValidator.playerName("+ConsolePlayer")).contains("+ConsolePlayer");
    }

    @Test
    void rejectsCommandAndControlInjection() {
        assertThat(InputValidator.playerName("name; op attacker")).isEmpty();
        assertThat(InputValidator.reason("ok\u0000bad", 240)).isEmpty();
        assertThat(InputValidator.reason("   ", 240)).isEmpty();
    }

    @Test
    void normalizesAndBoundsReasons() {
        assertThat(InputValidator.reason(" griefing    at spawn ", 240)).contains("griefing at spawn");
        assertThat(InputValidator.reason("x".repeat(241), 240)).isEmpty();
    }

    @Test
    void validatesReportIdsAndStaffActions() {
        String id = UUID.randomUUID().toString();
        assertThat(InputValidator.reportId(id)).contains(id);
        assertThat(InputValidator.reportId("../database")).isEmpty();
        assertThat(InputValidator.staffAction("Resolve")).contains("resolve");
        assertThat(InputValidator.staffAction("op")).isEmpty();
    }
}

