package com.northstar.vehicle.climate;

import com.northstar.vehicle.vehicle.VehicleNotFoundException;
import com.northstar.vehicle.vehicle.VehicleShadow;
import com.northstar.vehicle.vehicle.VehicleShadowRepository;
import jakarta.validation.Valid;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.http.ResponseEntity;
import org.springframework.security.core.annotation.AuthenticationPrincipal;
import org.springframework.security.oauth2.jwt.Jwt;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RestController;

/** Remote climate pre-conditioning: warm or cool the cabin before the owner gets in. */
@RestController
public class ClimateController {
    private static final Logger log = LoggerFactory.getLogger(ClimateController.class);

    private final VehicleShadowRepository vehicles;
    private final ClimateService climate;

    public ClimateController(VehicleShadowRepository vehicles, ClimateService climate) {
        this.vehicles = vehicles;
        this.climate = climate;
    }

    /** Pre-conditions one vehicle. */
    @PostMapping("/v1/vehicles/{vin}/climate")
    public ResponseEntity<Map<String, String>> precondition(
            @PathVariable String vin,
            @Valid @RequestBody ClimateRequest request,
            @AuthenticationPrincipal Jwt jwt) {
        VehicleShadow vehicle = vehicles.findByVin(vin).orElseThrow(VehicleNotFoundException::new);
        log.info("climate request vin={} account={} email={} target={}{}", vin, jwt.getSubject(),
                jwt.getClaimAsString("email"), request.targetTemp(), request.unit());
        UUID commandId = climate.precondition(vehicle, jwt.getSubject(), toCelsius(request));
        return ResponseEntity.accepted().body(Map.of("commandId", commandId.toString()));
    }

    /** "Warm up all my cars": pre-conditions every vehicle on the caller's account. */
    @PostMapping("/v1/accounts/me/climate")
    public ResponseEntity<Map<String, Integer>> preconditionAll(
            @Valid @RequestBody ClimateRequest request,
            @AuthenticationPrincipal Jwt jwt) {
        List<UUID> ids = climate.preconditionAll(jwt.getSubject(), toCelsius(request));
        return ResponseEntity.accepted().body(Map.of("accepted", ids.size()));
    }

    static int toCelsius(ClimateRequest request) {
        double value = request.targetTemp();
        if ("F".equalsIgnoreCase(request.unit())) {
            value = value * 9 / 5 + 32;
        }
        return (int) Math.round(Math.max(16, Math.min(30, value)));
    }
}
