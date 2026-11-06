package com.northstar.vehicle.vehicle;

import org.springframework.stereotype.Service;

/** Single place where "may this caller command this vehicle?" is decided. */
@Service
public class VehicleOwnershipService {
    private final VehicleShadowRepository vehicles;

    public VehicleOwnershipService(VehicleShadowRepository vehicles) {
        this.vehicles = vehicles;
    }

    /**
     * Returns the vehicle if accountId owns it.
     *
     * @throws VehicleNotFoundException if the vehicle is unknown or owned by someone else (same error for both)
     */
    public VehicleShadow requireOwned(String accountId, String vin) {
        return vehicles.findByVin(vin)
                .filter(v -> v.ownerAccountId().equals(accountId))
                .orElseThrow(VehicleNotFoundException::new);
    }
}
