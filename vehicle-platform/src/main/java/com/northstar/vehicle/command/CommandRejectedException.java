package com.northstar.vehicle.command;

import org.springframework.http.HttpStatus;
import org.springframework.web.bind.annotation.ResponseStatus;

/** The command is valid but not available for this vehicle (OEM does not support it, or it is not rolled out). */
@ResponseStatus(HttpStatus.UNPROCESSABLE_ENTITY)
public class CommandRejectedException extends RuntimeException {
    public CommandRejectedException(String message) {
        super(message);
    }
}
