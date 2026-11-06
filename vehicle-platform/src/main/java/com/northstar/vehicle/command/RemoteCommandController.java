package com.northstar.vehicle.command;

import com.northstar.vehicle.vehicle.VehicleOwnershipService;
import com.northstar.vehicle.vehicle.VehicleShadow;
import java.util.Map;
import java.util.UUID;
import org.springframework.http.ResponseEntity;
import org.springframework.security.core.annotation.AuthenticationPrincipal;
import org.springframework.security.oauth2.jwt.Jwt;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestHeader;
import org.springframework.web.bind.annotation.RestController;

/**
 * Remote command API used by the owner app and partner integrations.
 *
 * <p>Ownership is checked before anything else, and a vehicle the caller does not own is reported
 * exactly like a vehicle that does not exist (404), so the API is not a VIN-validity oracle.
 */
@RestController
public class RemoteCommandController {
    private final VehicleOwnershipService ownership;
    private final RemoteCommandService commands;

    public RemoteCommandController(VehicleOwnershipService ownership, RemoteCommandService commands) {
        this.ownership = ownership;
        this.commands = commands;
    }

    /**
     * Accepts a remote command for asynchronous delivery.
     *
     * @param vin            target vehicle
     * @param type           command type
     * @param idempotencyKey client-generated UUID; retries with the same key are accepted once
     * @param jwt            verified caller
     * @return 202 with the command id
     */
    @PostMapping("/v1/vehicles/{vin}/commands/{type}")
    public ResponseEntity<Map<String, String>> submit(
            @PathVariable String vin,
            @PathVariable CommandType type,
            @RequestHeader("Idempotency-Key") UUID idempotencyKey,
            @AuthenticationPrincipal Jwt jwt) {
        VehicleShadow vehicle = ownership.requireOwned(jwt.getSubject(), vin);
        UUID commandId = commands.submit(vehicle, type, jwt.getSubject(), Map.of(), idempotencyKey);
        return ResponseEntity.accepted().body(Map.of("commandId", commandId.toString()));
    }
}
