package com.northstar.vehicle.oem;

import com.northstar.vehicle.command.CommandType;
import com.northstar.vehicle.command.RemoteCommand;
import com.northstar.vehicle.vehicle.VehicleShadow;
import java.time.Duration;
import java.util.Set;

/** Delivers commands to one manufacturer's vehicles, hiding that OEM's protocol quirks. */
public interface OemAdapter {
    OemId oem();

    /** Commands this OEM's vehicles can execute. */
    Set<CommandType> supported();

    /**
     * Delivers a command and waits at most {@code timeout} for the vehicle or OEM cloud to acknowledge.
     * Must return {@link CommandResult#UNSUPPORTED_BY_OEM} (never ACCEPTED) for commands outside {@link #supported()}.
     */
    CommandResult send(RemoteCommand command, VehicleShadow vehicle, Duration timeout);
}
