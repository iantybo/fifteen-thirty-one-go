package com.northstar.vehicle.messaging;

import java.time.Duration;

/**
 * Sends the carrier "shoulder-tap" SMS that wakes a sleeping first-generation TCU so it reconnects to MQTT.
 * Required before every command to Brightline BL-G1 vehicles (see oem-capability-matrix.md).
 */
public interface SmsWakeupClient {
    /** @return true if the TCU reconnected within {@code timeout} */
    boolean wake(String vin, Duration timeout);
}
