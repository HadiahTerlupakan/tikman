import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { TroubledOnt, TroubledResult } from "@/domain/entities";
import { TroubledOntsPage } from "../TroubledOntsPage";

const useTroubledOnts = vi.hoisted(() =>
  vi.fn<
    (
      hours: number,
      oltId?: string,
      status?: string,
    ) => { data: TroubledResult | undefined; isLoading: boolean }
  >(),
);

vi.mock("@/application/hooks", () => ({
  useTroubledOnts,
  useOlts: () => ({
    data: [
      { id: "olt-cariu", name: "Cariu" },
      { id: "olt-bekasi", name: "Bekasi" },
    ],
  }),
}));

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

const dark: TroubledOnt = {
  ...flapping,
  ontId: "ont-2",
  serialNumber: "HWTCDF219D9A",
  name: "YADI",
  portId: 1,
  ontNumber: 21,
  status: "offline",
  trapCount: 4953,
  downMinutes: 826,
};

const result: TroubledResult = {
  data: [flapping, dark],
  summary: { ontCount: 503, totalDownMinutes: 74000 },
};

describe("TroubledOntsPage", () => {
  beforeEach(() => {
    useTroubledOnts.mockReturnValue({ data: result, isLoading: false });
  });

  it("shows a subscriber that reads online but keeps failing", () => {
    render(<TroubledOntsPage />);

    // The row the ONT list clears every time it is asked, which is the reason
    // this page exists.
    expect(screen.getByText("MAD SURYA")).toBeInTheDocument();
    expect(screen.getByText("ONU-5:3")).toBeInTheDocument();
    expect(screen.getByText("7.901")).toBeInTheDocument();
  });

  it("counts the whole population, not the page shown", () => {
    render(<TroubledOntsPage />);

    // Two rows are listed; the summary speaks for all five hundred, or it would
    // tell an operator a fraction of the truth.
    expect(screen.getByText("503")).toBeInTheDocument();
    expect(screen.getByText("1233 jam 20 mnt")).toBeInTheDocument();
  });

  it("names how many are hidden behind an online status", () => {
    render(<TroubledOntsPage />);

    // Asserted through the label rather than the bare figure: "1" also appears
    // in the pagination, and a test that matches either proves neither.
    const label = screen.getByText(/terbaca online, tetap sempat mati/i);
    expect(label.parentElement).toHaveTextContent("1");
  });

  it("marks the contradicting row so it is visible while scanning", () => {
    const { container } = render(<TroubledOntsPage />);

    const marked = container.querySelectorAll(".troubled-row-contradiction");
    expect(marked).toHaveLength(1);
    expect(
      within(marked[0] as HTMLElement).getByText("MAD SURYA"),
    ).toBeTruthy();
  });

  it("asks for a day and every OLT by default", () => {
    render(<TroubledOntsPage />);

    expect(useTroubledOnts).toHaveBeenCalledWith(24, undefined, undefined);
  });

  it("asks for a week when the operator picks one", () => {
    render(<TroubledOntsPage />);

    // fireEvent rather than userEvent: antd hides a Radio.Button's real input
    // behind pointer-events: none, which jsdom refuses to click through.
    fireEvent.click(screen.getByRole("radio", { name: "7 hari" }));

    expect(useTroubledOnts).toHaveBeenLastCalledWith(168, undefined, undefined);
  });

  it("asks for one status when the operator picks one", () => {
    render(<TroubledOntsPage />);

    fireEvent.mouseDown(screen.getByText("Semua status"));
    fireEvent.click(screen.getByText("LOS"));

    expect(useTroubledOnts).toHaveBeenLastCalledWith(24, undefined, "los");
  });

  it("says so plainly when nothing is in trouble", () => {
    useTroubledOnts.mockReturnValue({
      data: { data: [], summary: { ontCount: 0, totalDownMinutes: 0 } },
      isLoading: false,
    });
    render(<TroubledOntsPage />);

    expect(
      screen.getByText(/Tidak ada pelanggan yang beralarm/i),
    ).toBeInTheDocument();
  });
});
