package com.northstar.vehicle.oem.brightline;

import com.northstar.vehicle.command.CommandType;
import com.northstar.vehicle.command.RemoteCommand;
import com.northstar.vehicle.messaging.SmsWakeupClient;
import com.northstar.vehicle.messaging.VehicleMqttGateway;
import com.northstar.vehicle.messaging.payload.CommandPayloadV1;
import com.northstar.vehicle.messaging.payload.CommandPayloadV2;
import com.northstar.vehicle.oem.CommandResult;
import com.northstar.vehicle.oem.OemAdapter;
import com.northstar.vehicle.oem.OemId;
import com.northstar.vehicle.vehicle.VehicleShadow;
import java.time.Duration;
import java.util.EnumSet;
import java.util.Set;
import org.springframework.stereotype.Component;

/**
 * Brightline Auto. BL-G1 TCUs sleep to save the 12V battery and must be woken by SMS before every
 * command; they only understand payload v1. BL-G2 is always connected and speaks v2.
 */
@Component
public class BrightlineAdapter implements OemAdapter {
    private static final Set<CommandType> SUPPORTED = EnumSet.of(
            CommandType.DOOR_LOCK, CommandType.DOOR_UNLOCK, CommandType.REMOTE_START,
            CommandType.PRECONDITION_CLIMATE);

    private final VehicleMqttGateway mqtt;
    private final SmsWakeupClient sms;

    public BrightlineAdapter(VehicleMqttGateway mqtt, SmsWakeupClient sms) {
        this.mqtt = mqtt;
        this.sms = sms;
    }

    @Override
    public OemId oem() {
        return OemId.BRIGHTLINE;
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
        if (command.type() == CommandType.PRECONDITION_CLIMATE) {
            Integer target = Integer.valueOf(command.params().get("targetTempC"));
            CommandPayloadV1 payload = new CommandPayloadV1(command.commandId().toString(), command.type().name(),
                    command.requestedAt().toEpochMilli(), target);
            return mqtt.publish(topic(vehicle), payload, timeout) ? CommandResult.ACCEPTED : CommandResult.VEHICLE_UNREACHABLE;
        }
        if (isFirstGen(vehicle) && !sms.wake(vehicle.vin(), timeout)) {
            return CommandResult.VEHICLE_UNREACHABLE;
        }
        Object payload = vehicle.payloadVersion() >= 2
                ? new CommandPayloadV2(2, command.commandId().toString(), command.type().name(),
                        command.requestedAt().toEpochMilli(), command.params())
                : new CommandPayloadV1(command.commandId().toString(), command.type().name(),
                        command.requestedAt().toEpochMilli(), null);
        return mqtt.publish(topic(vehicle), payload, timeout) ? CommandResult.ACCEPTED : CommandResult.VEHICLE_UNREACHABLE;
    }

    private static boolean isFirstGen(VehicleShadow vehicle) {
        return "BL-G1".equals(vehicle.headUnitGen());
    }

    private static String topic(VehicleShadow vehicle) {
        return "brightline/" + vehicle.vin() + "/command";
    }
}
