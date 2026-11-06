package com.northstar.vehicle.command;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.northstar.vehicle.config.OemFeatureFlags;
import com.northstar.vehicle.logging.VinMasker;
import com.northstar.vehicle.oem.OemAdapterRegistry;
import com.northstar.vehicle.vehicle.VehicleShadow;
import io.awspring.cloud.sqs.operations.SqsTemplate;
import java.time.Clock;
import java.util.Map;
import java.util.UUID;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;

/**
 * Validates a command against OEM capability and rollout flags, records it in the idempotency ledger,
 * and enqueues it for delivery. Delivery itself happens in {@link CommandQueueListener}.
 */
@Service
public class RemoteCommandService {
    private static final Logger log = LoggerFactory.getLogger(RemoteCommandService.class);

    private final OemFeatureFlags flags;
    private final OemAdapterRegistry adapters;
    private final CommandLedger ledger;
    private final SqsTemplate sqs;
    private final ObjectMapper json;
    private final Clock clock;
    private final String queue;

    public RemoteCommandService(
            OemFeatureFlags flags,
            OemAdapterRegistry adapters,
            CommandLedger ledger,
            SqsTemplate sqs,
            ObjectMapper json,
            Clock clock,
            @Value("${northstar.sqs.command-queue}") String queue) {
        this.flags = flags;
        this.adapters = adapters;
        this.ledger = ledger;
        this.sqs = sqs;
        this.json = json;
        this.clock = clock;
        this.queue = queue;
    }

    /**
     * Accepts a command for delivery.
     *
     * @return the command id (equal to the idempotency key)
     * @throws CommandRejectedException if the OEM does not support the command or it is not rolled out to that OEM
     */
    public UUID submit(VehicleShadow vehicle, CommandType type, String accountId, Map<String, String> params,
            UUID idempotencyKey) {
        if (!adapters.forOem(vehicle.oem()).supported().contains(type)) {
            throw new CommandRejectedException(type + " is not supported for this vehicle");
        }
        if (!flags.isEnabled(vehicle.oem(), type)) {
            throw new CommandRejectedException(type + " is not available for this vehicle yet");
        }
        RemoteCommand command = new RemoteCommand(idempotencyKey, vehicle.vin(), type, accountId, params,
                clock.instant());
        if (!ledger.recordAccepted(command)) {
            log.info("duplicate submit for command {} on vehicle {}", command.commandId(), VinMasker.mask(vehicle.vin()));
            return command.commandId();
        }
        try {
            sqs.send(to -> to.queue(queue).payload(json.writeValueAsString(command)));
        } catch (JsonProcessingException e) {
            throw new IllegalStateException("could not serialize command " + command.commandId(), e);
        }
        return command.commandId();
    }
}
