import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { OntLinkPanel } from "../OntLinkPanel";
import { OntStatus, type Ont } from "@/domain/entities";

vi.mock("@/presentation/components/OntDetailModal", () => ({
  OntDetailModal: ({ visible }: { visible: boolean }) =>
    visible ? <div>detail ONT</div> : null,
}));

function ont(over: Partial<Ont> = {}): Ont {
  return {
    id: "o1",
    oltId: "olt1",
    oltName: "SERVER PKP",
    slot: 1,
    portId: 3,
    ontId: 12,
    serialNumber: "ZTEG1234",
    name: "Budi",
    description: "",
    status: OntStatus.ONLINE,
    lastSeenAt: null,
    createdAt: "",
    updatedAt: "",
    rxPower: -24.5,
    txPower: 2.1,
    ...over,
  } as Ont;
}

describe("OntLinkPanel", () => {
  // A customer reporting a slow line is usually answered by the optical power,
  // not by "online" — which says the link is up without saying how well.
  it("shows the optical readings, not just the status", () => {
    render(
      <OntLinkPanel
        ont={ont()}
        loading={false}
        unlinking={false}
        onUnlink={vi.fn()}
      />,
    );

    expect(screen.getByText("-24.50 dBm")).toBeInTheDocument();
    expect(screen.getByText("2.10 dBm")).toBeInTheDocument();
  });

  // A reading nobody took is not zero. "0.00 dBm" would be a measurement.
  it("shows a dash for a reading the ONT never reported", () => {
    render(
      <OntLinkPanel
        ont={ont({ rxPower: null, txPower: null })}
        loading={false}
        unlinking={false}
        onUnlink={vi.fn()}
      />,
    );

    expect(screen.queryByText(/dBm/)).toBeNull();
    expect(screen.getAllByText("—")).toHaveLength(2);
  });

  // A CS reads this address out to a technician, who then types it into the
  // OLT. Leaving the card out makes two ONUs on different cards read alike.
  it("names the OLT and the card, pon and onu the CLI uses", () => {
    render(
      <OntLinkPanel
        ont={ont()}
        loading={false}
        unlinking={false}
        onUnlink={vi.fn()}
      />,
    );

    expect(screen.getByText("SERVER PKP")).toBeInTheDocument();
    expect(screen.getByText("1/1/3:12")).toBeInTheDocument();
  });

  it("opens the full ONT detail without leaving the thread", async () => {
    render(
      <OntLinkPanel
        ont={ont()}
        loading={false}
        unlinking={false}
        onUnlink={vi.fn()}
      />,
    );

    expect(screen.queryByText("detail ONT")).toBeNull();
    await userEvent.click(screen.getByRole("button", { name: /detail/i }));
    expect(screen.getByText("detail ONT")).toBeInTheDocument();
  });

  // Unlinking also takes the customer's number back off the ONT, so a misclick
  // would undo data entry nobody asked to undo.
  it("asks before letting a link go", async () => {
    const onUnlink = vi.fn();
    render(
      <OntLinkPanel
        ont={ont()}
        loading={false}
        unlinking={false}
        onUnlink={onUnlink}
      />,
    );

    await userEvent.click(screen.getByRole("button", { name: /lepas/i }));
    expect(onUnlink).not.toHaveBeenCalled();

    await userEvent.click(screen.getByRole("button", { name: /^lepas$/i }));
    expect(onUnlink).toHaveBeenCalled();
  });
});
