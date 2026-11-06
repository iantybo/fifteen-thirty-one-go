package com.northstar.vehicle.messaging.payload;

/**
 * Legacy command payload (v1) understood by first-generation head units (e.g. Brightline BL-G1).
 *
 * <p><b>Frozen.</b> v1 firmware validates the JSON strictly and rejects any unknown or extra field, and
 * those head units cannot be updated over the air. Never add, remove, or rename fields; put new data
 * in {@link CommandPayloadV2#params()} and send v2 to vehicles whose shadow reports payloadVersion 2.
 */
public record CommandPayloadV1(String cmdId, String cmd, long ts, Integer targetTempC) {
}
