package com.northstar.vehicle.oem.avalon;

import com.northstar.vehicle.command.CommandType;
import com.northstar.vehicle.command.RemoteCommand;
import com.northstar.vehicle.messaging.VehicleMqttGateway;
import com.northstar.vehicle.messaging.payload.CommandPayloadV2;
import com.northstar.vehicle.oem.CommandResult;
import com.northstar.vehicle.oem.OemAdapter;
import com.northstar.vehicle.oem.OemId;
import com.northstar.vehicle.vehicle.VehicleShadow;
import java.time.Duration;
import java.util.EnumSet;
import java.util.Set;
import org.springframework.stereotype.Component;

/** Avalon Motors: always-connected TCUs on MQTT, payload v2 on every generation. */
@Component
public class AvalonAdapter implements OemAdapter {
    private static final Set<CommandType> SUPPORTED = EnumSet.of(
            CommandType.DOOR_LOCK, CommandType.DOOR_UNLOCK, CommandType.REMOTE_START, CommandType.HORN_LIGHTS,
            CommandType.PRECONDITION_CLIMATE);

    private final VehicleMqttGateway mqtt;

    public AvalonAdapter(VehicleMqttGateway mqtt) {
        this.mqtt = mqtt;
    }

    @Override
    public OemId oem() {
        return OemId.AVALON;
    }

    @Override
    public Set<CommandType> supported() {
        return SUPPORTED;
    }

    @Override
    public CommandResult send(RemoteCommand command, VehicleShadow vehicle, Duration timeout) {
        if (!SUPPORTED.contains(command.type())) {
            return CommandResult.UNSUPPORTED_BY_OEM;
        }
        CommandPayloadV2 payload = new CommandPayloadV2(2, command.commandId().toString(), command.type().name(),
                command.requestedAt().toEpochMilli(), command.params());
        if (command.type() == CommandType.PRECONDITION_CLIMATE) {
            mqtt.publishAndWait(topic(vehicle), payload);
            return CommandResult.ACCEPTED;
        }
        return mqtt.publish(topic(vehicle), payload, timeout) ? CommandResult.ACCEPTED : CommandResult.VEHICLE_UNREACHABLE;
    }

    private static String topic(VehicleShadow vehicle) {
        return "avalon/v2/" + vehicle.vin() + "/cmd";
    }
}
