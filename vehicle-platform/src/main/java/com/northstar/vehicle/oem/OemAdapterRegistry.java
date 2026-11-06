package com.northstar.vehicle.oem;

import java.util.EnumMap;
import java.util.List;
import java.util.Map;
import org.springframework.stereotype.Component;

/** Looks up the adapter for a vehicle's OEM. Fails loudly for an OEM with no adapter. */
@Component
public class OemAdapterRegistry {
    private final Map<OemId, OemAdapter> byOem = new EnumMap<>(OemId.class);

    public OemAdapterRegistry(List<OemAdapter> adapters) {
        for (OemAdapter adapter : adapters) {
            if (byOem.putIfAbsent(adapter.oem(), adapter) != null) {
                throw new IllegalStateException("two adapters registered for " + adapter.oem());
            }
        }
    }

    public OemAdapter forOem(OemId oem) {
        OemAdapter adapter = byOem.get(oem);
        if (adapter == null) {
            throw new IllegalStateException("no adapter for " + oem);
        }
        return adapter;
    }
}
