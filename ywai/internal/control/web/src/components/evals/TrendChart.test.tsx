import { render, screen } from "@testing-library/react";
import TrendChart, { type TrendSeries } from "./TrendChart";

describe("TrendChart", () => {
  it("renders the empty state without series", () => {
    render(<TrendChart series={[]} />);

    expect(screen.getByText(/no trend data yet/i)).toBeInTheDocument();
    expect(document.querySelector("svg")).toBeNull();
  });

  it("skips series without points", () => {
    const series: TrendSeries[] = [
      { model: "model-a", values: [] },
      { model: "model-b", values: [] },
    ];
    render(<TrendChart series={series} />);

    expect(screen.getByText(/no trend data yet/i)).toBeInTheDocument();
  });

  it("plots one polyline per series with one point per value", () => {
    const series: TrendSeries[] = [
      { model: "model-a", values: [0.2, 0.5, 0.8] },
      { model: "model-b", values: [0.4] },
    ];
    const { container } = render(<TrendChart series={series} />);

    const polylines = container.querySelectorAll("polyline");
    expect(polylines).toHaveLength(2);
    // One coordinate pair per trend point.
    expect(polylines[0].getAttribute("points")?.trim().split(/\s+/)).toHaveLength(3);
    expect(polylines[1].getAttribute("points")?.trim().split(/\s+/)).toHaveLength(1);
  });

  it("labels each sparkline with the model and shows the latest value", () => {
    const series: TrendSeries[] = [{ model: "model-a", values: [0.25, 0.75] }];
    render(<TrendChart series={series} />);

    expect(screen.getByText("model-a")).toBeInTheDocument();
    expect(screen.getByText("0.75")).toBeInTheDocument();
    expect(screen.getByRole("img")).toHaveAttribute(
      "aria-label",
      "model-a trend: 0.25, 0.75",
    );
  });
});
