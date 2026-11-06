package com.northstar.vehicle.oem.cascade;

import java.time.Duration;

/** Cascade Vehicles routes commands through its own OEM cloud (REST) instead of direct MQTT. */
public interface CascadeCloudClient {
    /** @return HTTP status from the Cascade cloud; 202 means the vehicle accepted the command */
    int postCommand(String vin, String cascadeCommand, String commandId, Duration timeout);
}
