package com.northstar.vehicle.oem;

/** Delivery outcome reported by an adapter. Adapters must never report ACCEPTED for work they did not do. */
public enum CommandResult {
    ACCEPTED,
    UNSUPPORTED_BY_OEM,
    VEHICLE_UNREACHABLE,
    REJECTED_BY_VEHICLE
}
