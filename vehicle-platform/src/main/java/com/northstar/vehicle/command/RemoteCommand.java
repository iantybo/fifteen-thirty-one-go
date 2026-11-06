package com.northstar.vehicle.command;

import java.time.Instant;
import java.util.Map;
import java.util.Objects;
import java.util.UUID;

/**
 * A command accepted for one vehicle.
 *
 * @param commandId   idempotency key; the same id is never dispatched to a vehicle twice
 * @param vin         target vehicle
 * @param type        command type
 * @param accountId   verified owner who requested it (from the JWT subject, never from the request body)
 * @param params      optional, command-specific parameters (e.g. climate target); empty for most commands
 * @param requestedAt when the platform accepted the command
 */
public record RemoteCommand(
        UUID commandId,
        String vin,
        CommandType type,
        String accountId,
        Map<String, String> params,
        Instant requestedAt) {

    public RemoteCommand {
        Objects.requireNonNull(commandId, "commandId");
        Objects.requireNonNull(vin, "vin");
        Objects.requireNonNull(type, "type");
        Objects.requireNonNull(accountId, "accountId");
        params = params == null ? Map.of() : Map.copyOf(params);
        Objects.requireNonNull(requestedAt, "requestedAt");
    }
}
