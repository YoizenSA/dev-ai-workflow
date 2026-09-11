import { describe, it, expect } from "vitest";
import { render, screen, fireEvent, within } from "@testing-library/react";
import NeedlePanel from "./NeedlePanel";
import type { NeedleAttempt } from "./NeedlePanel";

// Fixture mirrors the narrowed evals.Attempt view the integrator maps from
// GET /api/evals/runs (ywai/internal/evals/runner.go Score JSON).
function fixtureAttempts(): NeedleAttempt[] {
  return [
    {
      model: "m/full",
      round: 1,
      score: {
        hits: ["used codegraph", "cited the file"],
        missed: ["linked the docs"],
        gotHard: true,
        answered: true,
        weighted: 0.75,
      },
      responsePreview: "The answer is in the CodeGraph index.",
    },
    {
      model: "m/empty",
      round: 2,
      score: { hits: [], missed: [], gotHard: false, answered: false, weighted: 0 },
      responsePreview: "",
    },
  ];
}

function rowOf(model: string): HTMLElement {
  const row = screen.getByText(model).closest(".np-row");
  expect(row).not.toBeNull();
  return row as HTMLElement;
}

describe("NeedlePanel", () => {
  it("renders every attempt collapsed until expanded", () => {
    render(<NeedlePanel attempts={fixtureAttempts()} />);
    expect(screen.getByText("m/full")).toBeInTheDocument();
    expect(screen.getByText("m/empty")).toBeInTheDocument();
    // Chips live in the collapsible body: hidden while collapsed.
    expect(screen.queryByText("used codegraph")).toBeNull();
    expect(screen.queryByText("linked the docs")).toBeNull();
    for (const btn of screen.getAllByRole("button")) {
      expect(btn).toHaveAttribute("aria-expanded", "false");
    }
  });

  it("expands to show hit/missed chips, the hard badge, and the response preview", () => {
    render(<NeedlePanel attempts={fixtureAttempts()} />);
    fireEvent.click(screen.getByRole("button", { name: /m\/full round 1/ }));

    const row = rowOf("m/full");
    expect(row.querySelector('[aria-expanded="true"]')).not.toBeNull();
    expect(within(row).getByText("used codegraph")).toHaveClass("np-chip", "np-hit");
    expect(within(row).getByText("cited the file")).toHaveClass("np-chip", "np-hit");
    expect(within(row).getByText("linked the docs")).toHaveClass("np-chip", "np-miss");
    expect(within(row).getByText("hard")).toHaveClass("np-hard");
    expect(within(row).getByText("75%")).toBeInTheDocument();
    expect(within(row).getByText("The answer is in the CodeGraph index.")).toBeInTheDocument();
  });

  it("collapses again on a second click", () => {
    render(<NeedlePanel attempts={fixtureAttempts()} />);
    const btn = screen.getByRole("button", { name: /m\/full round 1/ });
    fireEvent.click(btn);
    expect(screen.getByText("used codegraph")).toBeInTheDocument();
    fireEvent.click(btn);
    expect(screen.queryByText("used codegraph")).toBeNull();
    expect(btn).toHaveAttribute("aria-expanded", "false");
  });

  it("shows the hard badge only when the attempt got every hard expectation", () => {
    render(<NeedlePanel attempts={fixtureAttempts()} />);
    expect(within(rowOf("m/full")).getByText("hard")).toHaveClass("np-hard");
    expect(within(rowOf("m/empty")).queryByText("hard")).toBeNull();
  });

  it("shows no answer for an unanswered attempt instead of a percent", () => {
    render(<NeedlePanel attempts={fixtureAttempts()} />);
    const row = rowOf("m/empty");
    expect(within(row).getByText("no answer")).toBeInTheDocument();
    expect(within(row).queryByText("0%")).toBeNull();
  });

  it("renders an empty state when there are no attempts", () => {
    render(<NeedlePanel attempts={[]} />);
    expect(screen.getByText(/no attempts to inspect/i)).toBeInTheDocument();
  });
});
