Feature: SAM.gov Opportunity Monitoring
  As a government contractor
  I want to monitor SAM.gov for relevant opportunities
  So that I can respond to them quickly

  Background:
    Given I have a valid SAM.gov API key
    And I have configured search queries

  Scenario: Successfully retrieve opportunities
    Given the following search parameters:
      | field            | value                      |
      | title           | artificial intelligence     |
      | organizationName| DARPA                      |
      | ptype           | s                          |
    When I execute the search
    Then I should receive a list of opportunities
    And each opportunity should have a notice ID
    And each opportunity should have a title
    And each opportunity should have a posted date

  Scenario: Handle no results gracefully
    Given a search with very specific criteria:
      | field            | value                           |
      | title           | extremelyspecificunlikelyterm   |
      | organizationName| NONEXISTENTORGANIZATION         |
    When I execute the search
    And no opportunities match
    Then I should receive an empty result set
    And no error should occur
    And the total records should be 0

  Scenario: API key validation
    Given an invalid API key
    When I attempt to execute a search
    Then I should receive an authentication error
    And the error should indicate invalid credentials

  Scenario: Date range filtering
    Given I configure a search with:
      | field       | value      |
      | postedFrom  | 01/01/2024 |
      | postedTo    | 01/31/2024 |
    When I execute the search
    Then all returned opportunities should be posted within the date range
    And no opportunities outside the range should be returned

  Scenario: Concurrent query execution
    Given I have 3 different queries configured:
      | name          | title        | organizationName |
      | AI Contracts  | AI           | DARPA           |
      | IT Services   | software     | DOD             |
      | Research      | research     | NSF             |
    When I execute all queries concurrently
    Then all queries should complete within 10 seconds
    And results should be returned for each query
    And no query should block another query

  Scenario: Handle API rate limiting
    Given the API returns a 429 rate limit error
    When I execute a search
    Then the system should retry with backoff
    And eventually succeed when rate limit is lifted
    And the retry attempts should be logged

  Scenario: Network timeout handling
    Given a network timeout occurs during API call
    When I execute a search
    Then the system should return a timeout error
    And the error should be properly formatted
    And the error should indicate the timeout cause