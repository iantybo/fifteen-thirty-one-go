package com.northstar.vehicle.climate;

import jakarta.validation.constraints.NotNull;

/**
 * Remote climate pre-conditioning request from the owner app.
 *
 * @param targetTemp desired cabin temperature in {@code unit}
 * @param unit       "C" or "F" (US apps send F)
 */
public record ClimateRequest(@NotNull Double targetTemp, @NotNull String unit) {
}
