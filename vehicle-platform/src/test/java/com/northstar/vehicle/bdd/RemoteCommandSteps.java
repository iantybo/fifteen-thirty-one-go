package com.northstar.vehicle.bdd;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

import com.northstar.vehicle.command.CommandType;
import com.northstar.vehicle.command.RemoteCommand;
import com.northstar.vehicle.messaging.SmsWakeupClient;
import com.northstar.vehicle.messaging.VehicleMqttGateway;
import com.northstar.vehicle.oem.CommandResult;
import com.northstar.vehicle.oem.OemAdapter;
import com.northstar.vehicle.oem.OemId;
import com.northstar.vehicle.oem.avalon.AvalonAdapter;
import com.northstar.vehicle.oem.brightline.BrightlineAdapter;
import com.northstar.vehicle.oem.cascade.CascadeAdapter;
import com.northstar.vehicle.oem.cascade.CascadeCloudClient;
import com.northstar.vehicle.vehicle.VehicleShadow;
import io.cucumber.java.en.Given;
import io.cucumber.java.en.Then;
import io.cucumber.java.en.When;
import java.time.Duration;
import java.time.Instant;
import java.util.Map;
import java.util.UUID;

/** Step definitions shared by every OEM's remote-command scenarios. */
public class RemoteCommandSteps {
    private final VehicleMqttGateway mqtt = mock(VehicleMqttGateway.class);
    private final SmsWakeupClient sms = mock(SmsWakeupClient.class);
    private final CascadeCloudClient cascadeCloud = mock(CascadeCloudClient.class);
    private VehicleShadow vehicle;
    private CommandResult result;

    @Given("a {word} vehicle with head unit {word} on payload v{int}")
    public void aVehicle(String oem, String headUnit, int payloadVersion) {
        vehicle = new VehicleShadow("TESTVIN0000000001", OemId.valueOf(oem), headUnit, "acct-1", payloadVersion);
        when(mqtt.publish(anyString(), any(), any())).thenReturn(true);
        when(sms.wake(anyString(), any())).thenReturn(true);
        when(cascadeCloud.postCommand(anyString(), anyString(), anyString(), any())).thenReturn(202);
    }

    @When("the owner sends {word}")
    public void theOwnerSends(String type) {
        RemoteCommand command = new RemoteCommand(UUID.randomUUID(), vehicle.vin(), CommandType.valueOf(type),
                "acct-1", Map.of(), Instant.now());
        result = adapter().send(command, vehicle, Duration.ofSeconds(8));
    }

    @Then("the result is {word}")
    public void theResultIs(String expected) {
        assertThat(result).isEqualTo(CommandResult.valueOf(expected));
    }

    private OemAdapter adapter() {
        return switch (vehicle.oem()) {
            case AVALON -> new AvalonAdapter(mqtt);
            case BRIGHTLINE -> new BrightlineAdapter(mqtt, sms);
            case CASCADE -> new CascadeAdapter(cascadeCloud);
        };
    }
}
