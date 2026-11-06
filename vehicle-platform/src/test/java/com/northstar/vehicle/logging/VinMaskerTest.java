package com.northstar.vehicle.logging;

import static org.assertj.core.api.Assertions.assertThat;

import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.CsvSource;
import org.junit.jupiter.params.provider.NullSource;

class VinMaskerTest {
    @ParameterizedTest
    @CsvSource({
        "1HGCM82633A004352, ***********004352",
        "ABC123, ******",
        "'', ******"
    })
    void masksAllButLastSix(String vin, String expected) {
        assertThat(VinMasker.mask(vin)).isEqualTo(expected);
    }

    @ParameterizedTest
    @NullSource
    void masksNull(String vin) {
        assertThat(VinMasker.mask(vin)).isEqualTo("******");
    }
}
