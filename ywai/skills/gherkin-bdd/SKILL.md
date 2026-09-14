---
name: gherkin-bdd
description: "Write BDD scenarios in Gherkin. Trigger: acceptance criteria, Given/When/Then, feature files, test scenarios."
---

# Gherkin BDD

A Gherkin scenario is a contract between someone who wants a behaviour and someone who verifies it. It is written in the language of the domain, not of the UI or the code, so it survives a redesign of both.

## Structure

```gherkin
Feature: <the capability, from the user's point of view>

  Background:
    Given <state every scenario below depends on>

  Scenario: <the behaviour being verified>
    Given <the starting state>
    When <the single action>
    Then <the observable outcome>
    And <a further assertion on that same outcome>
```

One `When` per scenario. A second `When` means a second scenario.

Use `Scenario Outline` when the same behaviour varies only by data:

```gherkin
  Scenario Outline: Rejected amounts are refused with a reason
    Given an account with balance <balance>
    When the customer withdraws <amount>
    Then the withdrawal is refused with "<reason>"

    Examples:
      | balance | amount | reason              |
      | 100     | 150    | insufficient funds  |
      | 100     | 0      | amount must be > 0  |
      | 100     | -50    | amount must be > 0  |
```

Examples rows that assert different behaviours do not belong in one Outline — split them.

## Rules

1. **Declarative, not imperative.** Describe intent, not keystrokes. `When the customer submits the order` — never `When I click "#submit-btn"`. Imperative steps break on every UI change and hide what is actually being tested.
2. **Every `Then` is observable.** A tester or an assertion must be able to see it. `Then it works correctly` is a hope, not an outcome.
3. **One behaviour per scenario.** A scenario asserting three things reports one failure and hides two.
4. **Scenarios are independent.** No scenario may depend on another having run. Shared setup goes in `Background`; shared *state* is a bug.
5. **`Given` is state, not action.** `Given the customer has an active subscription` — not `Given the customer signs up and pays`. If the setup reads like a scenario, it is one.
6. **Name the scenario after the behaviour, not the mechanism.** `Scenario: Expired card is declined at checkout`, never `Scenario: Test payment 2`.
7. **Cover the unhappy paths.** Empty, expired, unauthorized, concurrent, and boundary values earn scenarios. A feature with only happy paths is untested.

## Anti-patterns

| Smell | Why it hurts | Fix |
|---|---|---|
| Click-by-click steps | Rewritten on every UI change | Raise to the domain action |
| `Then the API returns 200` | Asserts plumbing, not behaviour | Assert what the user gets |
| Scenario 2 depends on scenario 1 | Random order breaks it; one failure cascades | Independent `Given` per scenario |
| Outline with 20 rows | Nobody reads it; failures are unattributable | Keep the boundary cases, drop the rest |
| Conjunctions in one step (`When A and B`) | Two behaviours, one failure signal | Split the scenario |

## Coverage

Work from the list of use cases and edge cases, not from intuition: **every case gets at least one scenario**, and coverage is checked against that list. A case with no scenario is a gap to report, not a case to silently drop.

**Done when** every case maps to at least one scenario, each scenario has exactly one `When` and an observable `Then`, and no scenario depends on another.
