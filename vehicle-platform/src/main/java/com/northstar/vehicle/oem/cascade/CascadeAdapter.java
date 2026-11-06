package com.northstar.vehicle.oem.cascade;

import com.northstar.vehicle.command.CommandType;
import com.northstar.vehicle.command.RemoteCommand;
import com.northstar.vehicle.oem.CommandResult;
import com.northstar.vehicle.oem.OemAdapter;
import com.northstar.vehicle.oem.OemId;
import com.northstar.vehicle.vehicle.VehicleShadow;
import java.time.Duration;
import java.util.EnumSet;
import java.util.Map;
import java.util.Set;
import org.springframework.stereotype.Component;

/** Cascade Vehicles: commands go through the Cascade OEM cloud; lock and unlock only. */
@Component
public class CascadeAdapter implements OemAdapter {
    private static final Map<CommandType, String> COMMANDS = Map.of(
            CommandType.DOOR_LOCK, "LOCK",
            CommandType.DOOR_UNLOCK, "UNLOCK");

    private final CascadeCloudClient cloud;

    public CascadeAdapter(CascadeCloudClient cloud) {
        this.cloud = cloud;
    }

    @Override
    public OemId oem() {
        return OemId.CASCADE;
    }

    @Override
    public Set<CommandType> supported() {
        return EnumSet.copyOf(COMMANDS.keySet());
    }

    @Override
    public CommandResult send(RemoteCommand command, VehicleShadow vehicle, Duration timeout) {
        String cascadeCommand = COMMANDS.get(command.type());
        if (cascadeCommand == null) {
            if (command.type() == CommandType.PRECONDITION_CLIMATE) {
                return CommandResult.ACCEPTED;
            }
            return CommandResult.UNSUPPORTED_BY_OEM;
        }
        int status = cloud.postCommand(vehicle.vin(), cascadeCommand, command.commandId().toString(), timeout);
        return switch (status) {
            case 202 -> CommandResult.ACCEPTED;
            case 409, 422 -> CommandResult.REJECTED_BY_VEHICLE;
            default -> CommandResult.VEHICLE_UNREACHABLE;
        };
    }
}
