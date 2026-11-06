package com.northstar.vehicle.climate;

import static org.assertj.core.api.Assertions.assertThat;

import org.junit.jupiter.api.Test;

class ClimateControllerTest {
    @Test
    void celsiusPassesThrough() {
        assertThat(ClimateController.toCelsius(new ClimateRequest(22.0, "C"))).isEqualTo(22);
    }
}
