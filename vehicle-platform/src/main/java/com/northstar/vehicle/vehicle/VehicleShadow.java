package com.northstar.vehicle.vehicle;

import com.northstar.vehicle.oem.OemId;

/**
 * The platform's record of one vehicle on the road (its "shadow"). This is the source of truth for
 * which OEM adapter, head-unit generation, and payload version a command must use.
 *
 * @param vin              vehicle identification number (treat as personal data: mask in logs)
 * @param oem              manufacturer
 * @param headUnitGen      head-unit / TCU generation, e.g. "BL-G1"; older generations cannot be updated
 * @param ownerAccountId   current owner's account id
 * @param payloadVersion   highest command payload version the vehicle accepts (1 or 2)
 */
public record VehicleShadow(String vin, OemId oem, String headUnitGen, String ownerAccountId, int payloadVersion) {
}
