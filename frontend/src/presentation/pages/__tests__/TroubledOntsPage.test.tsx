import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { TroubledOnt } from "@/domain/entities";
import { TroubledOntsPage } from "../TroubledOntsPage";

const useTroubledOnts = vi.hoisted(() =>
  vi.fn<
    (hours: number) => { data: TroubledOnt[] | undefined; isLoading: boolean }
  >(),
);

vi.mock("@/application/hooks", () => ({ useTroubledOnts }));

const flapping: TroubledOnt = {
  ontId: "ont-1",
  serialNumber: "ZTEGCACC308A",
  name: "MAD SURYA",
  oltName: "Cariu",
  portId: 5,
  ontNumber: 3,
  status: "online",
  trapCount: 7901,
  downMinutes: 325,
};

describe("TroubledOntsPage", () => {
  beforeEach(() => {
    useTroubledOnts.mockReturnValue({ data: [flapping], isLoading: false });
  });

  it("shows a subscriber that reads online but keeps failing", () => {
    render(<TroubledOntsPage />);

    // The whole point of the page: this row is invisible on the ONT list,
    // which only ever asks whether the subscriber is up right now.
    expect(screen.getByText("MAD SURYA")).toBeInTheDocument();
    expect(screen.getByText("ONU-5:3")).toBeInTheDocument();
    expect(screen.getByText("ONLINE")).toBeInTheDocument();
  });

  it("renders the churn and what it cost in readable units", () => {
    render(<TroubledOntsPage />);

    expect(screen.getByText("7.901")).toBeInTheDocument();
    expect(screen.getByText("5 jam 25 mnt")).toBeInTheDocument();
  });

  it("asks for a day by default", () => {
    render(<TroubledOntsPage />);

    expect(useTroubledOnts).toHaveBeenCalledWith(24);
  });

  it("asks for a week when the operator picks one", () => {
    render(<TroubledOntsPage />);

    // fireEvent rather than userEvent: antd hides a Radio.Button's real input
    // behind pointer-events: none, which jsdom refuses to click through.
    fireEvent.click(screen.getByRole("radio", { name: "7 hari" }));

    expect(useTroubledOnts).toHaveBeenLastCalledWith(168);
  });

  it("says so plainly when nothing is in trouble", () => {
    useTroubledOnts.mockReturnValue({ data: [], isLoading: false });
    render(<TroubledOntsPage />);

    expect(
      screen.getByText(/Tidak ada pelanggan yang beralarm/i),
    ).toBeInTheDocument();
  });
});
