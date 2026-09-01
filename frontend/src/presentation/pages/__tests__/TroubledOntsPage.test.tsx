import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { PonHealth, TroubledOnt, TroubledResult } from "@/domain/entities";
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

const usePonHealth = vi.hoisted(() =>
  vi.fn<
    (
      oltId: string | undefined,
      hours: number,
    ) => { data: PonHealth | undefined; isLoading: boolean }
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
  usePonHealth,
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

// Matches `flapping`'s portId (5), so selecting it narrows the table to one row.
const healthWithMatch: PonHealth = {
  oltId: "olt-cariu",
  oltName: "Cariu",
  medianTrapPerOnt: 50,
  trapThreshold: 100,
  outageThreshold: 0.1,
  cards: [
    {
      slot: 1,
      ponCount: 1,
      pons: [
        {
          port: 5,
          ontCount: 10,
          trapPerOnt: 200,
          outageShare: 0.2,
          worst: [
            {
              ontId: "ont-1",
              label: "ONU-5:3",
              name: "MAD SURYA",
              trapCount: 7901,
              downMinutes: 325,
            },
          ],
        },
      ],
    },
  ],
};

// Port 7 matches neither `flapping` (5) nor `dark` (1): the worst port on the
// topology isn't necessarily among the fifty rows the ranked page loaded.
const healthWithoutMatch: PonHealth = {
  ...healthWithMatch,
  cards: [
    {
      slot: 1,
      ponCount: 1,
      pons: [{ ...healthWithMatch.cards[0].pons[0], port: 7 }],
    },
  ],
};

function selectOlt() {
  fireEvent.mouseDown(screen.getByText("Semua OLT"));
  // Plain `getByText("Cariu")` is ambiguous — it's also the OLT column
  // already on screen — and antd's `role="option"` mirror is a visually
  // hidden accessibility node, not the clickable one; the option a click
  // actually lands on is this content div.
  fireEvent.click(
    screen.getByText("Cariu", { selector: ".ant-select-item-option-content" }),
  );
}

function goToPonTabAndSelectPort() {
  fireEvent.click(screen.getByRole("tab", { name: /Per PON/i }));
  fireEvent.click(screen.getByText(/^PON \d+$/));
}

describe("TroubledOntsPage", () => {
  beforeEach(() => {
    useTroubledOnts.mockReturnValue({ data: result, isLoading: false });
    usePonHealth.mockReturnValue({ data: undefined, isLoading: false });
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

  it("offers both views on one page", () => {
    render(<TroubledOntsPage />);

    expect(
      screen.getByRole("tab", { name: /Per Pelanggan/i }),
    ).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: /Per PON/i })).toBeInTheDocument();
  });

  it("asks for an OLT before it can draw a topology", () => {
    render(<TroubledOntsPage />);

    fireEvent.click(screen.getByRole("tab", { name: /Per PON/i }));

    // One chassis at a time is the whole reason the view stays readable.
    expect(screen.getByText(/pilih OLT/i)).toBeInTheDocument();
  });

  it("states the narrowing on the tag instead of on the summary", () => {
    usePonHealth.mockReturnValue({ data: healthWithMatch, isLoading: false });
    render(<TroubledOntsPage />);

    selectOlt();
    goToPonTabAndSelectPort();

    // The summary keeps reporting the server's whole matching population
    // (503) — only the tag says the table itself is down to one of them, so
    // nothing on screen can be read as two different totals.
    expect(
      screen.getByText("PON 5 · 1 dari 503 pelanggan"),
    ).toBeInTheDocument();
    expect(screen.getByText("503")).toBeInTheDocument();
  });

  it("explains why the table is empty when the picked PON has no ranked row", () => {
    usePonHealth.mockReturnValue({
      data: healthWithoutMatch,
      isLoading: false,
    });
    render(<TroubledOntsPage />);

    selectOlt();
    goToPonTabAndSelectPort();

    expect(
      screen.getByText(
        "Pelanggan PON ini tidak masuk daftar peringkat pada rentang ini",
      ),
    ).toBeInTheDocument();
  });
});
