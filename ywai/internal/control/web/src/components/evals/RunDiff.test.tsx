import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import RunDiff from "./RunDiff";
import type { RunDiffDelta } from "./RunDiff";

// Fixture mirrors evals.ModelDelta JSON (ywai/internal/evals/delta.go): a null
// field is "no baseline to compare", a number is current minus baseline.
function fixtureDeltas(): RunDiffDelta[] {
  return [
    { model: "m/up", weightedDelta: 0.25, hardDelta: -0.5, turnsDelta: 1.5, costDeltaUsd: 0.05 },
    { model: "m/null", weightedDelta: null, hardDelta: null, turnsDelta: null, costDeltaUsd: null },
    { model: "m/zero", weightedDelta: 0, hardDelta: 0, turnsDelta: 0, costDeltaUsd: 0 },
  ];
}

function rowOf(model: string): HTMLTableRowElement {
  const row = screen.getByText(model).closest("tr");
  expect(row).not.toBeNull();
  return row as HTMLTableRowElement;
}

// Cells in column order: model, weighted, hard, turns, cost.
function cellsOf(model: string): HTMLElement[] {
  return Array.from(rowOf(model).querySelectorAll("td"));
}

describe("RunDiff", () => {
  it("renders one row per model under the a-vs-b titles", () => {
    render(<RunDiff aTitle="run A (now)" bTitle="run B (baseline)" deltas={fixtureDeltas()} />);
    expect(screen.getByText("m/up")).toBeInTheDocument();
    expect(screen.getByText("m/null")).toBeInTheDocument();
    expect(screen.getByText("m/zero")).toBeInTheDocument();
    expect(screen.getByText("run A (now)")).toBeInTheDocument();
    expect(screen.getByText("run B (baseline)")).toBeInTheDocument();
  });

  it("renders null deltas as a no-comparison em dash, distinct from zero", () => {
    render(<RunDiff aTitle="a" bTitle="b" deltas={fixtureDeltas()} />);

    for (const cell of cellsOf("m/null").slice(1)) {
      expect(cell.textContent).toBe("—");
      expect(cell.querySelector(".rd-none")).toHaveAttribute("title", "no comparison");
    }

    // A computed zero keeps its flat glyph and a visible value, and never
    // claims "no comparison". Money keeps its four decimals.
    const zeroCells = cellsOf("m/zero");
    expect(zeroCells[1].textContent).toBe("— 0.00");
    expect(zeroCells[2].textContent).toBe("— 0.00");
    expect(zeroCells[3].textContent).toBe("— 0.00");
    expect(zeroCells[4].textContent).toBe("— $0.0000");
    for (const cell of zeroCells.slice(1)) {
      expect(cell.querySelector('[title="no comparison"]')).toBeNull();
    }
  });

  it("formats value columns: scores with two decimals, cost in dollars", () => {
    render(<RunDiff aTitle="a" bTitle="b" deltas={fixtureDeltas()} />);
    const [model, weighted, hard, turns, cost] = cellsOf("m/up");
    expect(model.textContent).toBe("m/up");
    expect(weighted.textContent).toBe("▲ +0.25");
    expect(hard.textContent).toBe("▼ -0.50");
    expect(turns.textContent).toBe("▲ +1.50");
    expect(cost.textContent).toBe("▲ $+0.0500");
  });

  it("colours up as good for scores and up as heavier for turns/cost", () => {
    render(<RunDiff aTitle="a" bTitle="b" deltas={fixtureDeltas()} />);
    const [, weighted, hard, turns, cost] = cellsOf("m/up");
    expect(weighted.querySelector(".rd-delta")).toHaveClass("rd-good");
    expect(hard.querySelector(".rd-delta")).toHaveClass("rd-bad");
    expect(turns.querySelector(".rd-delta")).toHaveClass("rd-bad");
    expect(cost.querySelector(".rd-delta")).toHaveClass("rd-bad");
  });

  it("renders an empty state when there is nothing to compare", () => {
    render(<RunDiff aTitle="a" bTitle="b" deltas={[]} />);
    expect(screen.getByText(/no models to compare/i)).toBeInTheDocument();
    expect(screen.queryByRole("table")).toBeNull();
  });
});
