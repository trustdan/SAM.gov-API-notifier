Feature: Opportunity Deduplication
  As a monitor system
  I need to track which opportunities have been seen
  So that I only notify about new opportunities

  Background:
    Given the monitor system is running
    And I have a state file for tracking opportunities

  Scenario: First time seeing an opportunity
    Given an empty state file
    And I have an opportunity with notice ID "DARPA-001"
    When I process the opportunity
    Then it should be marked as new
    And it should be added to the state
    And the first_seen timestamp should be set
    And the last_seen timestamp should be set

  Scenario: Previously seen opportunity
    Given opportunity "DARPA-001" was seen yesterday
    And I have the same opportunity with notice ID "DARPA-001"
    When I process the opportunity again
    Then it should not be marked as new
    And the last_seen date should be updated
    And the first_seen date should remain unchanged
    And no notification should be triggered

  Scenario: Modified opportunity detection
    Given opportunity "DARPA-001" with modification date "2024-01-01"
    And the opportunity was processed previously
    When I process the same opportunity with modification date "2024-01-02"
    Then it should be marked as updated
    And the last_modified timestamp should be updated
    And a notification should be triggered for the update

  Scenario: Multiple opportunities in batch
    Given I have a batch of 5 opportunities:
      | noticeId | title        | firstTime |
      | DARPA-001| AI Research  | true      |
      | DARPA-002| ML Contract  | false     |
      | DOD-001  | Cyber Sec    | true      |
      | NSF-001  | Basic Res    | true      |
      | DARPA-003| Robotics     | false     |
    When I process the batch
    Then 3 opportunities should be marked as new
    And 2 opportunities should be marked as existing
    And the state should contain all 5 opportunities
    And only new opportunities should trigger notifications

  Scenario: Opportunity expires and returns
    Given opportunity "DARPA-001" was seen 60 days ago
    And the opportunity is no longer active
    When the same opportunity appears again as active
    Then it should be treated as a new opportunity
    And a notification should be sent
    And the state should reflect the new appearance

  Scenario: Hash-based change detection
    Given opportunity "DARPA-001" with title "AI Research Contract"
    And the opportunity has been processed with hash "abc123"
    When the same opportunity appears with title "AI Research Contract (Updated)"
    Then the hash should be different
    And it should be marked as updated
    And the new hash should be stored

  Scenario: Concurrent state access
    Given multiple queries are processing opportunities simultaneously
    When opportunity "DARPA-001" is processed by query A
    And opportunity "DARPA-002" is processed by query B at the same time
    Then both opportunities should be properly stored
    And no race conditions should occur
    And the state file should remain consistent

  Scenario: State file corruption recovery
    Given a corrupted state file
    When the monitor attempts to load the state
    Then it should create a new empty state
    And log the corruption event
    And continue processing normally
    And all opportunities should be treated as new