Feature: Remote commands behave correctly for every OEM and head-unit generation
  Every OEM-facing change adds or updates a scenario for each OEM and generation it affects.

  Scenario Outline: <command> on <oem> <headUnit>
    Given a <oem> vehicle with head unit <headUnit> on payload v<payload>
    When the owner sends <command>
    Then the result is <result>

    Examples:
      | oem        | headUnit | payload | command      | result             |
      | AVALON     | AV-5     | 2       | DOOR_UNLOCK  | ACCEPTED           |
      | AVALON     | AV-5     | 2       | REMOTE_START | ACCEPTED           |
      | BRIGHTLINE | BL-G1    | 1       | DOOR_UNLOCK  | ACCEPTED           |
      | BRIGHTLINE | BL-G2    | 2       | REMOTE_START | ACCEPTED           |
      | BRIGHTLINE | BL-G2    | 2       | HORN_LIGHTS  | UNSUPPORTED_BY_OEM |
      | CASCADE    | CV-3     | 2       | DOOR_LOCK    | ACCEPTED           |
      | CASCADE    | CV-3     | 2       | REMOTE_START | UNSUPPORTED_BY_OEM |
