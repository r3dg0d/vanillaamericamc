package dev.vanillaamericaplus.message;

import dev.vanillaamericaplus.config.PluginSettings;
import net.kyori.adventure.text.Component;
import net.kyori.adventure.text.minimessage.MiniMessage;
import net.kyori.adventure.text.minimessage.tag.Tag;
import net.kyori.adventure.text.minimessage.tag.resolver.Placeholder;
import net.kyori.adventure.text.minimessage.tag.resolver.TagResolver;
import org.bukkit.configuration.file.YamlConfiguration;
import org.bukkit.plugin.java.JavaPlugin;

import java.io.File;
import java.util.Map;

public final class MessageBundle {
    private final MiniMessage miniMessage;
    private final Map<String, String> messages;
    private final TagResolver theme;

    private MessageBundle(MiniMessage miniMessage, Map<String, String> messages, TagResolver theme) {
        this.miniMessage = miniMessage;
        this.messages = messages;
        this.theme = theme;
    }

    public static MessageBundle load(JavaPlugin plugin, PluginSettings settings) {
        File file = new File(plugin.getDataFolder(), "messages.yml");
        YamlConfiguration yaml = YamlConfiguration.loadConfiguration(file);
        Map<String, String> loaded = yaml.getKeys(false).stream()
                .collect(java.util.stream.Collectors.toUnmodifiableMap(
                        key -> key,
                        key -> {
                            String value = yaml.getString(key);
                            if (value == null || value.isBlank() || value.length() > 4000) {
                                throw new IllegalArgumentException("Invalid messages.yml entry: " + key);
                            }
                            return value;
                        }
                ));
        TagResolver theme = TagResolver.builder()
                .tag("navy", Tag.styling(net.kyori.adventure.text.format.TextColor.fromHexString(settings.primary())))
                .tag("red", Tag.styling(net.kyori.adventure.text.format.TextColor.fromHexString(settings.accent())))
                .tag("ivory", Tag.styling(net.kyori.adventure.text.format.TextColor.fromHexString(settings.ivory())))
                .build();
        return new MessageBundle(MiniMessage.miniMessage(), loaded, theme);
    }

    public Component get(String key, TagResolver... placeholders) {
        String raw = messages.getOrDefault(key, "<red>Missing message: " + key + "</red>");
        TagResolver resolver = TagResolver.builder().resolver(theme).resolvers(placeholders).build();
        Component content = miniMessage.deserialize(raw, resolver);
        if ("prefix".equals(key)) {
            return content;
        }
        String prefix = messages.get("prefix");
        return prefix == null ? content : miniMessage.deserialize(prefix, theme).append(content);
    }

    public Component plain(String input) {
        return Component.text(input);
    }

    public static TagResolver value(String name, String value) {
        return Placeholder.unparsed(name, value);
    }
}

