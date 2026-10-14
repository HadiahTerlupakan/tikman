import { describe, expect, it } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import type { RouteAdvice } from "@/domain/entities";
import { FlowRouteAdvicePanel } from "./FlowRouteAdvicePanel";

const path = {
  peer: "peer-cdn",
  remoteAs: 64511,
  transit: false,
  asPath: "64511,64520,64521,64496",
  pathLength: 4,
  localPref: 600,
};

const rows: RouteAdvice[] = [
  {
    prefix: "198.51.100.0/24",
    asn: 64496,
    name: "EXAMPLE-NET-A",
    downloadBytes: 5000,
    uploadBytes: 50,
    status: "shorter_outbound",
    active: path,
    alternatives: [
      {
        ...path,
        peer: "peer-small-ix",
        asPath: "64512,64496",
        pathLength: 2,
        localPref: 300,
      },
    ],
    inbound: [{ peer: "peer-cdn", transit: false, bytes: 5000 }],
    draft:
      '/routing/filter/rule add chain=SMALLIX_IN comment="tikman: 198.51.100.0/24 via peer-small-ix"',
    checkedAt: "2026-10-14T03:00:00Z",
  },
  {
    prefix: "203.0.113.0/24",
    asn: 64497,
    name: "EXAMPLE-NET-B",
    downloadBytes: 1000,
    uploadBytes: 10,
    status: "optimal",
    active: { ...path, peer: "peer-ix" },
    alternatives: null,
    inbound: null,
    draft: "",
    checkedAt: "2026-10-14T03:00:00Z",
  },
];

describe("FlowRouteAdvicePanel", () => {
  it("shows only what needs action until asked for everything", () => {
    render(
      <FlowRouteAdvicePanel
        available
        rows={rows}
        loading={false}
        failed={false}
      />,
    );

    expect(screen.getByText("198.51.100.0/24")).toBeInTheDocument();
    expect(screen.queryByText("203.0.113.0/24")).toBeNull();

    fireEvent.click(screen.getByText("Semua"));

    expect(screen.getByText("203.0.113.0/24")).toBeInTheDocument();
  });

  it("opens a prefix to its advice and the draft to copy", () => {
    render(
      <FlowRouteAdvicePanel
        available
        rows={rows}
        loading={false}
        failed={false}
      />,
    );

    fireEvent.click(document.querySelector(".ant-table-row-expand-icon")!);

    expect(screen.getByText(/Draf konfigurasi/)).toBeInTheDocument();
    expect(screen.getByText(/chain=SMALLIX_IN/)).toBeInTheDocument();
    expect(
      screen.getByText(/hanya memindahkan trafik keluar/),
    ).toBeInTheDocument();
  });

  it("says when nothing has been read yet", () => {
    render(
      <FlowRouteAdvicePanel
        available
        rows={[]}
        loading={false}
        failed={false}
      />,
    );

    expect(screen.getByText(/Belum ada rute yang dibaca/)).toBeInTheDocument();
  });
});
