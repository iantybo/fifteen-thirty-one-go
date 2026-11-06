package com.northstar.vehicle.oem.brightline;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.inOrder;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import com.northstar.vehicle.command.CommandType;
import com.northstar.vehicle.command.RemoteCommand;
import com.northstar.vehicle.messaging.SmsWakeupClient;
import com.northstar.vehicle.messaging.VehicleMqttGateway;
import com.northstar.vehicle.messaging.payload.CommandPayloadV1;
import com.northstar.vehicle.oem.CommandResult;
import com.northstar.vehicle.oem.OemId;
import com.northstar.vehicle.vehicle.VehicleShadow;
import java.time.Duration;
import java.time.Instant;
import java.util.Map;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import org.mockito.InOrder;

class BrightlineAdapterTest {
    private static final Duration TIMEOUT = Duration.ofSeconds(8);
    private static final VehicleShadow G1 = new VehicleShadow("5BLAU00001G100001", OemId.BRIGHTLINE, "BL-G1", "acct-1", 1);
    private static final VehicleShadow G2 = new VehicleShadow("5BLAU00002G200002", OemId.BRIGHTLINE, "BL-G2", "acct-1", 2);

    private final VehicleMqttGateway mqtt = mock(VehicleMqttGateway.class);
    private final SmsWakeupClient sms = mock(SmsWakeupClient.class);
    private final BrightlineAdapter adapter = new BrightlineAdapter(mqtt, sms);

    private static RemoteCommand unlock(VehicleShadow vehicle) {
        return new RemoteCommand(UUID.randomUUID(), vehicle.vin(), CommandType.DOOR_UNLOCK, "acct-1", Map.of(), Instant.now());
    }

    @Test
    void firstGenIsWokenBeforeTheV1CommandIsPublished() {
        when(sms.wake(G1.vin(), TIMEOUT)).thenReturn(true);
        when(mqtt.publish(anyString(), any(), eq(TIMEOUT))).thenReturn(true);

        assertThat(adapter.send(unlock(G1), G1, TIMEOUT)).isEqualTo(CommandResult.ACCEPTED);

        InOrder order = inOrder(sms, mqtt);
        order.verify(sms).wake(G1.vin(), TIMEOUT);
        order.verify(mqtt).publish(eq("brightline/" + G1.vin() + "/command"), any(CommandPayloadV1.class), eq(TIMEOUT));
    }

    @Test
    void firstGenThatDoesNotWakeIsUnreachableAndNothingIsPublished() {
        when(sms.wake(G1.vin(), TIMEOUT)).thenReturn(false);

        assertThat(adapter.send(unlock(G1), G1, TIMEOUT)).isEqualTo(CommandResult.VEHICLE_UNREACHABLE);
        verify(mqtt, never()).publish(anyString(), any(), any());
    }

    @Test
    void secondGenSkipsWakeUp() {
        when(mqtt.publish(anyString(), any(), eq(TIMEOUT))).thenReturn(true);

        assertThat(adapter.send(unlock(G2), G2, TIMEOUT)).isEqualTo(CommandResult.ACCEPTED);
        verify(sms, never()).wake(anyString(), any());
    }

    @Test
    void unsupportedCommandIsReportedNotSent() {
        RemoteCommand horn = new RemoteCommand(UUID.randomUUID(), G2.vin(), CommandType.HORN_LIGHTS, "acct-1", Map.of(), Instant.now());
        assertThat(adapter.send(horn, G2, TIMEOUT)).isEqualTo(CommandResult.UNSUPPORTED_BY_OEM);
    }
}
