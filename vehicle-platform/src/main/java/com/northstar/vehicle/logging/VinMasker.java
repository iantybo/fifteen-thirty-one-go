package com.northstar.vehicle.logging;

/**
 * Masks VINs for logs and metrics. A VIN plus a location or timestamp identifies a person, so logs
 * carry only the last 6 characters (enough for support to correlate with the shadow table).
 */
public final class VinMasker {
    private VinMasker() {
    }

    public static String mask(String vin) {
        if (vin == null || vin.length() <= 6) {
            return "******";
        }
        return "*".repeat(vin.length() - 6) + vin.substring(vin.length() - 6);
    }
}
