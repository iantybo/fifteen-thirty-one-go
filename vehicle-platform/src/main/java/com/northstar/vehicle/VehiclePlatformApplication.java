package com.northstar.vehicle;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.boot.context.properties.ConfigurationPropertiesScan;

/**
 * Northstar new platform services: the cloud-side proxy for every connected vehicle on the road.
 * Accepts remote commands from apps and partners, and delivers them through per-OEM adapters.
 */
@SpringBootApplication
@ConfigurationPropertiesScan
public class VehiclePlatformApplication {
    public static void main(String[] args) {
        SpringApplication.run(VehiclePlatformApplication.class, args);
    }
}
