package com.northstar.vehicle.climate;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.northstar.vehicle.command.RemoteCommand;
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

/** Delivers climate pre-conditioning commands to vehicles. */
@Component
public class ClimateQueueListener {
    private static final Logger log = LoggerFactory.getLogger(ClimateQueueListener.class);

    private final ObjectMapper json;
    private final VehicleShadowRepository vehicles;
    private final OemAdapterRegistry adapters;
    private final Duration timeout;

    public ClimateQueueListener(ObjectMapper json, VehicleShadowRepository vehicles, OemAdapterRegistry adapters,
            @Value("${northstar.vehicle.command-timeout}") Duration timeout) {
        this.json = json;
        this.vehicles = vehicles;
        this.adapters = adapters;
        this.timeout = timeout;
    }

    @SqsListener("${northstar.sqs.climate-queue:climate-commands}")
    public void onMessage(String body) throws JsonProcessingException {
        RemoteCommand command = json.readValue(body, RemoteCommand.class);
        VehicleShadow vehicle = vehicles.findByVin(command.vin()).orElseThrow();
        CommandResult result = adapters.forOem(vehicle.oem()).send(command, vehicle, timeout);
        log.info("climate command {} vin={} result={}", command.commandId(), command.vin(), result);
    }
}
