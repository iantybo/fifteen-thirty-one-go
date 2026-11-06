package com.northstar.vehicle.command;

/** Remote commands the platform can deliver. Each OEM supports a subset; see OemAdapter#supported. */
public enum CommandType {
    DOOR_LOCK,
    DOOR_UNLOCK,
    REMOTE_START,
    HORN_LIGHTS,
    PRECONDITION_CLIMATE
}
