# OEM capability matrix

Vehicles stay on the road for 10 to 15 years, and most head units are never updated after they ship.
This matrix is the source of truth for what each OEM and generation can do. Code that disagrees
with it is a bug, even when every test passes.

| OEM | Generation | Transport | Payload | Wake-up | Lock/Unlock | Remote start | Horn/lights | Climate pre-conditioning |
|-----|------------|-----------|---------|---------|-------------|--------------|-------------|--------------------------|
| Avalon Motors | AV-4, AV-5 | MQTT (always connected) | v2 | none | yes | yes | yes | yes (params.targetTempC) |
| Brightline Auto | BL-G1 | MQTT after SMS shoulder-tap | **v1 only (strict schema)** | **SMS before every command** | yes | yes | no | **no** |
| Brightline Auto | BL-G2 | MQTT (always connected) | v2 | none | yes | yes | no | yes (params.targetTempC) |
| Cascade Vehicles | CV-2, CV-3 | Cascade OEM cloud (REST) | n/a | Cascade cloud handles it | yes | no | no | **no**; certification is planned for 2027.Q2 |

## Rules that follow from the matrix

1. **Payload v1 is frozen.** BL-G1 firmware rejects any JSON field it does not know, and the whole
   command fails. `CommandPayloadV1` must never gain, lose, or rename a field. New parameters go in
   `CommandPayloadV2.params` and are only sent to vehicles whose shadow reports `payloadVersion >= 2`.
2. **Wake before every BL-G1 command.** That includes new command types. A BL-G1 that was not woken
   drops the MQTT message silently.
3. **Unsupported means UNSUPPORTED_BY_OEM.** An adapter must return `UNSUPPORTED_BY_OEM` for anything
   the matrix marks "no", and the API must reject it before queueing. Returning `ACCEPTED` for work
   that never happened tells the owner their car is warming up when it is not.
4. **Capability is per generation, not per OEM.** Check `VehicleShadow.headUnitGen` and
   `payloadVersion`; never assume a whole OEM supports a feature.
