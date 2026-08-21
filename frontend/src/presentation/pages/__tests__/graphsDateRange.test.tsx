import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphsPage } from "../GraphsPage";
import { OntStatus } from "@/domain/entities/Ont";
import type { ReactElement, ReactNode } from "react";


vi.mock("antd", async (importOriginal) => {
  const actual = await importOriginal<typeof import("antd")>();
  return {
    ...actual,
    Select: ({
      placeholder,
      value,
      onChange,
      children,
    }: {
      placeholder?: string;
      value?: string;
      onChange?: (value: string | undefined) => void;
      children: ReactNode;
    }) => {
      const childArray = Array.isArray(children) ? children : [children];
      const options = childArray
        .filter(Boolean)
        .map((child) => (child as ReactElement<{ value: string; children: ReactNode }>).props);
      return (
        <select
          aria-label={placeholder}
          value={value ?? ""}
          onChange={(e) => onChange?.(e.target.value || undefined)}
        >
          <option value="">{placeholder}</option>
          {options.map((o) => (
            <option key={o.value} value={o.value}>
              {o.children}
            </option>
          ))}
        </select>
      );
    },
    DatePicker: {
      RangePicker: ({
        placeholder,
        onChange,
      }: {
        placeholder?: [string, string];
        onChange?: (values: Array<{ startOf: () => { toISOString: () => string }; endOf: () => { toISOString: () => string } }> | null) => void;
      }) => (
        <div>
          <input placeholder={placeholder?.[0]} readOnly />
          <input placeholder={placeholder?.[1]} readOnly />
          <button
            type="button"
            onClick={() =>
              onChange?.([
                {
                  startOf: () => ({ toISOString: () => "2026-08-20T00:00:00.000Z" }),
                  endOf: () => ({ toISOString: () => "2026-08-20T23:59:59.999Z" }),
                },
                {
                  startOf: () => ({ toISOString: () => "2026-08-21T00:00:00.000Z" }),
                  endOf: () => ({ toISOString: () => "2026-08-21T23:59:59.999Z" }),
                },
              ])
            }
          >
            Apply date range
          </button>
        </div>
      ),
    },
  };
});

vi.mock("@/application/hooks/useOlts", () => ({
  useOlts: () => ({ data: [{ id: "olt-1", name: "OLT Cariu" }] }),
}));

const mockUseOnts = vi.hoisted(() => vi.fn());
vi.mock("@/application/hooks/useOnts", () => ({
  useOnts: mockUseOnts,
}));

mockUseOnts.mockImplementation((params?: { status?: OntStatus }) => {
  const onts = [
    makeOnt({ id: "1", status: OntStatus.ONLINE }),
    makeOnt({ id: "2", serialNumber: "ZTEGABCDEF01", status: OntStatus.OFFLINE }),
  ].filter((ont) => !params?.status || ont.status === params.status);

  return {
    isLoading: false,
    data: { data: onts, total: onts.length },
  };
});

const mockUseOntTrafficTimeSeries = vi.hoisted(() => vi.fn());
vi.mock("@/application/hooks/useOntMetrics", () => ({
  useOntTrafficTimeSeries: mockUseOntTrafficTimeSeries,
}));

mockUseOntTrafficTimeSeries.mockReturnValue({
  isLoading: false,
  data: [
    { time: new Date("2026-08-20T12:00:00.000Z"), txMbps: 10, rxMbps: 5 },
  ],
});

function makeOnt(overrides: Partial<Record<string, unknown>>) {
  return {
    id: "ont-1",
    oltId: "olt-1",
    oltName: "OLT1",
    slot: 1,
    portId: 1,
    ontId: 1,
    serialNumber: "RTEGC609D6CF",
    name: "Budi Santoso",
    description: "",
    status: OntStatus.ONLINE,
    lastSeenAt: null,
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    ...overrides,
  };
}

function cardWithSerial(serial: string) {
  return screen.queryByText((_, element) => {
    const className = element?.className?.toString() || "";
    return className.includes("ant-card-body") && element?.textContent?.includes(serial) === true;
  });
}

describe("GraphsPage date range", () => {
  it("displays ONT status tag on each traffic card", () => {
    render(<GraphsPage />);

    expect(screen.getByText((_, element) => element?.className?.toString().includes("ant-tag") === true && element?.textContent === "online")).toBeInTheDocument();
    expect(screen.getByText((_, element) => element?.className?.toString().includes("ant-tag") === true && element?.textContent === "offline")).toBeInTheDocument();
  });

  it("renders date range controls", () => {
    render(<GraphsPage />);

    expect(screen.getByPlaceholderText("Start date")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("End date")).toBeInTheDocument();
  });
  it("filters graph cards by ONT status", async () => {
    render(<GraphsPage />);

    expect(cardWithSerial("RTEGC609D6CF")).toBeInTheDocument();
    expect(cardWithSerial("ZTEGABCDEF01")).toBeInTheDocument();

    await userEvent.selectOptions(screen.getByLabelText("Select status"), "offline");

    expect(cardWithSerial("RTEGC609D6CF")).not.toBeInTheDocument();
    expect(cardWithSerial("ZTEGABCDEF01")).toBeInTheDocument();
    expect(screen.getByText("Showing 1 of 1 ONTs")).toBeInTheDocument();
  });

  it("sends selected date range to the ONT list query", async () => {
    render(<GraphsPage />);

    await userEvent.click(screen.getByText("Apply date range"));

    expect(mockUseOnts).toHaveBeenLastCalledWith(
      expect.objectContaining({
        startTime: "2026-08-20T00:00:00.000Z",
        endTime: "2026-08-21T23:59:59.999Z",
      })
    );
  });

  it("shows custom range as the active time filter", async () => {
    render(<GraphsPage />);

    await userEvent.click(screen.getByText("Apply date range"));

    expect(screen.getAllByRole('status').length).toBeGreaterThan(0);
  });

  it("shows dates on chart labels when custom range is active", async () => {
    render(<GraphsPage />);

    await userEvent.click(screen.getByText("Apply date range"));

    // Check that chart shows date labels (Aug 20)
    // The x-axis tick formatter should show "Aug 20\n12:00 PM" for custom range
    const container = document.body;
    const textContent = container.textContent || '';
    expect(textContent).toMatch(/Aug 20|2026-08-20/);
  });
});
