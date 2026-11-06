package com.northstar.vehicle.command;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.northstar.vehicle.logging.VinMasker;
import com.northstar.vehicle.oem.CommandResult;
import com.northstar.vehicle.oem.OemAdapterRegistry;
import com.northstar.vehicle.vehicle.VehicleShadow;
import com.northstar.vehicle.vehicle.VehicleShadowRepository;
import io.awspring.cloud.sqs.annotation.SqsListener;
import java.time.Duration;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Component;

/** Delivers queued commands to vehicles exactly once per command id. */
@Component
public class CommandQueueListener {
    private static final Logger log = LoggerFactory.getLogger(CommandQueueListener.class);

    private final ObjectMapper json;
    private final CommandLedger ledger;
    private final VehicleShadowRepository vehicles;
    private final OemAdapterRegistry adapters;
    private final Duration timeout;

    public CommandQueueListener(ObjectMapper json, CommandLedger ledger, VehicleShadowRepository vehicles,
            OemAdapterRegistry adapters, @Value("${northstar.vehicle.command-timeout}") Duration timeout) {
        this.json = json;
        this.ledger = ledger;
        this.vehicles = vehicles;
        this.adapters = adapters;
        this.timeout = timeout;
    }

    @SqsListener("${northstar.sqs.command-queue}")
    public void onMessage(String body) throws JsonProcessingException {
        RemoteCommand command = json.readValue(body, RemoteCommand.class);
        if (!ledger.claimForDispatch(command)) {
            log.info("command {} already dispatched; dropping redelivery", command.commandId());
            return;
        }
        VehicleShadow vehicle = vehicles.findByVin(command.vin())
                .orElseThrow(() -> new IllegalStateException("vehicle vanished for command " + command.commandId()));
        CommandResult result = adapters.forOem(vehicle.oem()).send(command, vehicle, timeout);
        ledger.recordOutcome(command, result.name());
        log.info("command {} type={} oem={} vehicle={} result={}", command.commandId(), command.type(),
                vehicle.oem(), VinMasker.mask(command.vin()), result);
    }
}
