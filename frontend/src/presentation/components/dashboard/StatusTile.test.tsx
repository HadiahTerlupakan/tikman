import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { StatusTile } from "./StatusTile";

describe("StatusTile", () => {
  it("shows the count, label and hint", () => {
    render(
      <StatusTile
        tone="success"
        label="Online"
        value={7}
        hint="Operating normally"
      />,
    );

    expect(screen.getByText("7")).toBeInTheDocument();
    expect(screen.getByText("Online")).toBeInTheDocument();
    expect(screen.getByText("Operating normally")).toBeInTheDocument();
  });

  it("shows a share of the total when one is given", () => {
    render(<StatusTile tone="danger" label="Error" value={3} total={12} />);

    expect(screen.getByText("25% of 12")).toBeInTheDocument();
  });

  it("omits the share when the total is zero, avoiding a division by zero", () => {
    render(<StatusTile tone="neutral" label="Offline" value={0} total={0} />);

    expect(screen.queryByText(/% of/)).not.toBeInTheDocument();
  });

  it("renders a dash instead of a count when the value is unknown", () => {
    // A failed query must not render 0, which is indistinguishable from a real
    // zero and would tell the operator everything is fine.
    render(<StatusTile tone="neutral" label="Online" value={null} />);

    expect(screen.getByText("—")).toBeInTheDocument();
    expect(screen.queryByText("0")).not.toBeInTheDocument();
  });

  it("calms a fault tile that has nothing to report", () => {
    // A red block reading "Error 0" demands attention while saying there is no
    // problem. Visual weight has to track whether something is actionable.
    const { container } = render(
      <StatusTile tone="danger" label="Error" value={0} total={12} />,
    );

    expect(container.querySelector("[data-tone]")).toHaveAttribute(
      "data-tone",
      "quiet",
    );
  });

  it("keeps the alarm tone once a fault is actually present", () => {
    const { container } = render(
      <StatusTile tone="danger" label="Error" value={1} total={12} />,
    );

    expect(container.querySelector("[data-tone]")).toHaveAttribute(
      "data-tone",
      "danger",
    );
  });

  it("stays quiet for a fault whose count is unknown", () => {
    // A dash means "we do not know", which is not grounds for an alarm colour.
    const { container } = render(
      <StatusTile tone="warning" label="Dying Gasp" value={null} />,
    );

    expect(container.querySelector("[data-tone]")).toHaveAttribute(
      "data-tone",
      "quiet",
    );
  });

  it("does not calm a success tile at zero, since zero online is meaningful", () => {
    // "Online 0" is the one zero that matters: it means everything is down.
    const { container } = render(
      <StatusTile tone="success" label="Online" value={0} total={4} />,
    );

    expect(container.querySelector("[data-tone]")).toHaveAttribute(
      "data-tone",
      "success",
    );
  });
});
