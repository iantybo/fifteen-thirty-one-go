package com.northstar.vehicle.vehicle;

import org.springframework.http.HttpStatus;
import org.springframework.web.bind.annotation.ResponseStatus;

/** Thrown for unknown vehicles and for vehicles the caller does not own; both map to the same 404. */
@ResponseStatus(HttpStatus.NOT_FOUND)
public class VehicleNotFoundException extends RuntimeException {
    public VehicleNotFoundException() {
        super("vehicle not found");
    }
}
