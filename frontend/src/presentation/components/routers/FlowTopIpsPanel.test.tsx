import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { FlowTopIpsPanel } from "./FlowTopIpsPanel";

const rows = [
  {
    ip: "198.51.100.7",
    asn: 64497,
    name: "EXAMPLE-NET-B",
    country: "JP",
    bytes: 2048,
    flows: 12345,
    peakSubscribers: 37,
  },
  {
    ip: "192.0.2.9",
    asn: 64496,
    name: "EXAMPLE-NET-A",
    country: "",
    bytes: 1024,
    flows: 0,
    peakSubscribers: 0,
  },
];

describe("FlowTopIpsPanel", () => {
  it("flags a located address with its server's country and leaves an unknown one bare", () => {
    render(
      <FlowTopIpsPanel available rows={rows} loading={false} failed={false} />,
    );

    expect(screen.getByTitle("Lokasi server: Jepang")).toHaveTextContent(
      "\u{1F1EF}\u{1F1F5}",
    );
    expect(screen.getByText("192.0.2.9").closest("td")?.textContent).toBe(
      "192.0.2.9",
    );
  });

  it("credits DB-IP, as its CC BY licence requires", () => {
    render(
      <FlowTopIpsPanel available rows={rows} loading={false} failed={false} />,
    );

    expect(
      screen.getByRole("link", { name: "Lokasi IP oleh DB-IP" }),
    ).toHaveAttribute("href", "https://db-ip.com");
  });

  it("shows connections and peak subscribers, and a dash where none were recorded", () => {
    render(
      <FlowTopIpsPanel available rows={rows} loading={false} failed={false} />,
    );

    const located = screen.getByText("198.51.100.7").closest("tr")!;
    expect(located).toHaveTextContent("12.345");
    expect(located).toHaveTextContent("37");
    const unrecorded = screen.getByText("192.0.2.9").closest("tr")!;
    expect(unrecorded.querySelectorAll("td")[3]).toHaveTextContent("—");
    expect(unrecorded.querySelectorAll("td")[4]).toHaveTextContent("—");
  });
});
