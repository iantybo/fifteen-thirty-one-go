package com.northstar.vehicle.command;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import com.northstar.vehicle.config.OemFeatureFlags;
import com.northstar.vehicle.oem.OemAdapter;
import com.northstar.vehicle.oem.OemAdapterRegistry;
import com.northstar.vehicle.oem.OemId;
import com.northstar.vehicle.vehicle.VehicleShadow;
import io.awspring.cloud.sqs.operations.SqsTemplate;
import java.time.Clock;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.EnumSet;
import java.util.Map;
import java.util.Set;
import java.util.UUID;
import java.util.function.Consumer;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

class RemoteCommandServiceTest {
    private static final VehicleShadow CASCADE_CAR = new VehicleShadow("3CZRE38579G700001", OemId.CASCADE, "CV-3", "acct-1", 2);

    private final OemAdapter cascade = mock(OemAdapter.class);
    private final OemAdapterRegistry adapters = mock(OemAdapterRegistry.class);
    private final CommandLedger ledger = mock(CommandLedger.class);
    private final SqsTemplate sqs = mock(SqsTemplate.class);
    private RemoteCommandService service;

    @BeforeEach
    void setUp() {
        when(adapters.forOem(OemId.CASCADE)).thenReturn(cascade);
        when(cascade.supported()).thenReturn(EnumSet.of(CommandType.DOOR_LOCK, CommandType.DOOR_UNLOCK));
        OemFeatureFlags flags = new OemFeatureFlags(Map.of(OemId.CASCADE, Set.of(CommandType.DOOR_LOCK)));
        service = new RemoteCommandService(flags, adapters, ledger, sqs,
                new ObjectMapper().registerModule(new JavaTimeModule()),
                Clock.fixed(Instant.parse("2026-11-06T12:00:00Z"), ZoneOffset.UTC), "remote-commands");
    }

    @Test
    void rejectsCommandTheOemDoesNotSupport() {
        assertThatThrownBy(() -> service.submit(CASCADE_CAR, CommandType.REMOTE_START, "acct-1", Map.of(), UUID.randomUUID()))
                .isInstanceOf(CommandRejectedException.class);
        verify(sqs, never()).send(any(Consumer.class));
    }

    @Test
    void rejectsSupportedCommandNotYetRolledOutToThatOem() {
        assertThatThrownBy(() -> service.submit(CASCADE_CAR, CommandType.DOOR_UNLOCK, "acct-1", Map.of(), UUID.randomUUID()))
                .isInstanceOf(CommandRejectedException.class)
                .hasMessageContaining("not available");
        verify(sqs, never()).send(any(Consumer.class));
    }

    @Test
    void duplicateIdempotencyKeyIsAcceptedButNotEnqueuedTwice() {
        UUID key = UUID.randomUUID();
        when(ledger.recordAccepted(any())).thenReturn(false);

        UUID id = service.submit(CASCADE_CAR, CommandType.DOOR_LOCK, "acct-1", Map.of(), key);

        assertThat(id).isEqualTo(key);
        verify(sqs, never()).send(any(Consumer.class));
    }
}
