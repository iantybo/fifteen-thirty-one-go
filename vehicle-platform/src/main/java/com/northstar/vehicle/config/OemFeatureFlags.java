package com.northstar.vehicle.config;

import com.northstar.vehicle.command.CommandType;
import com.northstar.vehicle.oem.OemId;
import java.util.EnumMap;
import java.util.Map;
import java.util.Set;
import org.springframework.boot.context.properties.ConfigurationProperties;

/**
 * Per-OEM rollout flags ({@code northstar.flags.<OEM>: [COMMANDS]}). A command reaches an OEM's
 * vehicles only after that OEM passes certification for it, so new behaviour ships dark and is
 * enabled one OEM at a time in a later monthly release.
 */
@ConfigurationProperties("northstar")
public record OemFeatureFlags(Map<OemId, Set<CommandType>> flags) {
    public OemFeatureFlags {
        flags = flags == null ? Map.of() : new EnumMap<>(flags);
    }

    public boolean isEnabled(OemId oem, CommandType type) {
        return flags.getOrDefault(oem, Set.of()).contains(type);
    }
}
