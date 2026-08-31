package dev.vanillaamericaplus.config;

import org.junit.jupiter.api.Test;

import java.util.Map;

import static org.assertj.core.api.Assertions.assertThat;

class EntitlementAllowlistTest {
    @Test
    void releaseCatalogContainsOnlyCosmeticPermissions() {
        Map<String, String> allowlist = Map.of(
                "supporter_badge", "vaplus.cosmetic.supporter",
                "liberty_bell", "vaplus.event.liberty"
        );
        assertThat(allowlist)
                .allSatisfy((code, permission) -> {
                    assertThat(code).matches("[a-z0-9_]+");
                    assertThat(permission).startsWith("vaplus.");
                    assertThat(permission).doesNotContain("op", "gamemode", "give");
                });
    }
}

