import { describe, expect, it } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import type { OwnPrefix } from "@/domain/entities";
import { FlowOwnPrefixPanel } from "./FlowOwnPrefixPanel";

const own: OwnPrefix = {
  prefix: "203.0.113.0/24",
  downloadBytes: 1000,
  uploadBytes: 40,
  peers: [
    {
      peer: "peer-cdn",
      transit: false,
      sessions: ["peer-cdn"],
      outputFilter: "CDN_OUT",
      rule: "if (dst in 203.0.113.0/24 && dst-len == 24) { set bgp-path-prepend 2; accept; }",
      announced: true,
      prepend: 2,
      downloadBytes: 700,
      uploadBytes: 0,
    },
    {
      peer: "peer-ix-rs1",
      transit: false,
      sessions: ["peer-ix-rs1"],
      outputFilter: "IX_OUT",
      rule: "if (dst in 203.0.113.0/24 && dst-len == 24) { accept; }",
      announced: true,
      prepend: 0,
      downloadBytes: 300,
      uploadBytes: 40,
    },
    {
      peer: "peer-transit",
      transit: true,
      sessions: ["peer-transit"],
      outputFilter: "TRANSIT_OUT",
      rule: "if (dst in 203.0.113.0/24) { reject; }",
      announced: false,
      prepend: 0,
      downloadBytes: 0,
      uploadBytes: 0,
    },
  ],
};

describe("FlowOwnPrefixPanel", () => {
  it("shows where download comes in and how each peer is told", () => {
    render(
      <FlowOwnPrefixPanel
        available
        rows={[own]}
        loading={false}
        failed={false}
      />,
    );

    expect(screen.getByText("peer-cdn (70%)")).toBeInTheDocument();
    expect(screen.getByText("peer-cdn: prepend 2")).toBeInTheDocument();
    expect(screen.getByText("peer-ix-rs1: tanpa prepend")).toBeInTheDocument();
    expect(
      screen.getByText("peer-transit: tidak diumumkan"),
    ).toBeInTheDocument();
  });

  it("drafts leaning download towards the peer chosen", () => {
    render(
      <FlowOwnPrefixPanel
        available
        rows={[own]}
        loading={false}
        failed={false}
      />,
    );
    fireEvent.click(document.querySelector(".ant-table-row-expand-icon")!);

    expect(
      screen.getByText(/Arahkan download untuk 203\.0\.113\.0\/24 lewat/),
    ).toBeInTheDocument();
    expect(screen.getAllByText(/chain="CDN_OUT"/).length).toBeGreaterThan(0);
    expect(screen.queryByText(/chain="TRANSIT_OUT"/)).toBeNull();
  });

  it("says when no prefix has been read yet", () => {
    render(
      <FlowOwnPrefixPanel available rows={[]} loading={false} failed={false} />,
    );

    expect(
      screen.getByText(/Belum ada prefix yang terbaca/),
    ).toBeInTheDocument();
  });
});
