Feature: Remote climate pre-conditioning

  Scenario: Avalon vehicle pre-conditions the cabin
    Given a AVALON vehicle with head unit AV-5 on payload v2
    When the owner sends PRECONDITION_CLIMATE
    Then the result is ACCEPTED
