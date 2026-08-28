import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { OntActions } from "./OntActions";
import type { Ont } from "@/domain/entities";

const ont = {
  id: "ont-1",
  oltId: "olt-1",
  portId: 1,
  ontId: 1,
  serialNumber: "HWTCB403E8A0",
  name: "Bapak Budi",
  status: "online",
} as Ont;

function renderActions(
  overrides: Partial<Parameters<typeof OntActions>[0]> = {},
) {
  const handlers = {
    onViewDetail: vi.fn(),
    onDelete: vi.fn(),
    onProvision: vi.fn(),
    onConfigureService: vi.fn(),
    onViewHistory: vi.fn(),
  };
  render(<OntActions ont={ont} {...handlers} {...overrides} />);
  return handlers;
}

describe("OntActions", () => {
  // The two an operator reaches for stay one click away.
  it("opens the detail and the service form directly", async () => {
    const handlers = renderActions();

    await userEvent.click(screen.getByTestId("ont-view"));
    await userEvent.click(screen.getByTestId("ont-configure"));

    expect(handlers.onViewDetail).toHaveBeenCalledWith(ont);
    expect(handlers.onConfigureService).toHaveBeenCalledWith(ont);
  });

  it("keeps the rarer actions behind the overflow", async () => {
    renderActions();

    expect(screen.queryByText("Provision")).not.toBeInTheDocument();
    expect(screen.queryByText("History")).not.toBeInTheDocument();

    await userEvent.click(screen.getByTestId("ont-more"));

    expect(await screen.findByText("Provision")).toBeInTheDocument();
    expect(screen.getByText("History")).toBeInTheDocument();
    expect(screen.getByText("Delete")).toBeInTheDocument();
  });

  // Deleting takes the ONT's metrics and events with it, so it asks first.
  it("confirms before deleting", async () => {
    const handlers = renderActions();

    await userEvent.click(screen.getByTestId("ont-more"));
    await userEvent.click(await screen.findByText("Delete"));

    expect(handlers.onDelete).not.toHaveBeenCalled();
    expect(
      await screen.findByText(/Delete ONT HWTCB403E8A0\?/),
    ).toBeInTheDocument();
  });

  // A non-ZTE OLT passes no service handler, and the icon must not appear.
  it("hides the service action when the caller offers none", () => {
    renderActions({ onConfigureService: undefined });

    expect(screen.queryByTestId("ont-configure")).not.toBeInTheDocument();
  });
});
