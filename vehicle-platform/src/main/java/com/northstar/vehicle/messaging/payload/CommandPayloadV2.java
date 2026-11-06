package com.northstar.vehicle.messaging.payload;

import java.util.Map;

/**
 * Current command envelope (v2). Vehicles ignore unknown keys in {@code params}, so new command
 * parameters go there without a firmware change.
 */
public record CommandPayloadV2(int v, String cmdId, String cmd, long ts, Map<String, String> params) {
    public CommandPayloadV2 {
        params = params == null ? Map.of() : Map.copyOf(params);
    }
}
