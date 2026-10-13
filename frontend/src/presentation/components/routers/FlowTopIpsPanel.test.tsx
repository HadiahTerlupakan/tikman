import { describe, expect, it } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { FlowTopIpsPanel } from "./FlowTopIpsPanel";

const rows = [
  {
    ip: "198.51.100.7",
    asn: 64497,
    name: "EXAMPLE-NET-B",
    country: "JP",
    countrySource: "server" as const,
    bytes: 2048,
    flows: 12345,
    asnType: "Content",
    anonymity: "vpn" as const,
  },
  {
    ip: "192.0.2.9",
    asn: 64496,
    name: "EXAMPLE-NET-A",
    country: "",
    countrySource: "" as const,
    bytes: 1024,
    flows: 0,
    asnType: "",
    anonymity: "" as const,
  },
  {
    ip: "203.0.113.20",
    asn: 64498,
    name: "EXAMPLE-NET-C",
    country: "NL",
    countrySource: "as" as const,
    bytes: 512,
    flows: 0,
    asnType: "",
    anonymity: "" as const,
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

  it("credits IPinfo, as its CC BY-SA licence requires", () => {
    render(
      <FlowTopIpsPanel available rows={rows} loading={false} failed={false} />,
    );

    expect(
      screen.getByRole("link", { name: "Data IP oleh IPinfo" }),
    ).toHaveAttribute("href", "https://ipinfo.io");
  });

  it("shows connections, and a dash where none were recorded", () => {
    render(
      <FlowTopIpsPanel available rows={rows} loading={false} failed={false} />,
    );

    const located = screen.getByText("198.51.100.7").closest("tr")!;
    expect(located).toHaveTextContent("12.345");
    const unrecorded = screen.getByText("192.0.2.9").closest("tr")!;
    expect(unrecorded.querySelectorAll("td")[5]).toHaveTextContent("—");
  });

  it("falls back to the AS's registered country, dimmed and labelled as such", () => {
    render(
      <FlowTopIpsPanel available rows={rows} loading={false} failed={false} />,
    );

    const fallback = screen.getByTitle(
      "Negara terdaftar AS: Belanda (lokasi server belum tercatat)",
    );
    expect(fallback).toHaveTextContent("\u{1F1F3}\u{1F1F1}");
    expect(fallback).toHaveStyle({ opacity: "0.45" });
    expect(screen.getByTitle("Lokasi server: Jepang")).toHaveStyle({
      opacity: "1",
    });
  });

  it("names the ASN type in Indonesian and tags the anonymity flag", () => {
    render(
      <FlowTopIpsPanel available rows={rows} loading={false} failed={false} />,
    );

    const flagged = screen.getByText("198.51.100.7").closest("tr")!;
    expect(flagged).toHaveTextContent("Konten");
    expect(flagged.querySelector(".ant-tag")).toHaveTextContent("VPN");
    const plain = screen.getByText("192.0.2.9").closest("tr")!;
    expect(plain.querySelector(".ant-tag")).toBeNull();
  });
});

describe("FlowTopIpsPanel pagination", () => {
  const many = (bytesOffset: number) =>
    Array.from({ length: 100 }, (_, i) => ({
      ip: `198.51.100.${i}`,
      asn: 64497,
      name: "EXAMPLE-NET-B",
      country: "JP",
      countrySource: "server" as const,
      bytes: 100_000 - i + bytesOffset,
      flows: 1,
      asnType: "",
      anonymity: "" as const,
    }));

  it("moves to another page and stays there when the data refreshes", () => {
    const { rerender } = render(
      <FlowTopIpsPanel
        available
        rows={many(0)}
        loading={false}
        failed={false}
      />,
    );

    fireEvent.click(screen.getByTitle("2"));
    expect(screen.getByText("198.51.100.20")).toBeInTheDocument();
    expect(screen.queryByText("198.51.100.0")).toBeNull();

    rerender(
      <FlowTopIpsPanel
        available
        rows={many(1)}
        loading={false}
        failed={false}
      />,
    );
    expect(screen.getByText("198.51.100.20")).toBeInTheDocument();
  });

  it("changes how many rows a page shows", async () => {
    render(
      <FlowTopIpsPanel
        available
        rows={many(0)}
        loading={false}
        failed={false}
      />,
    );
    expect(screen.queryByText("198.51.100.20")).toBeNull();

    fireEvent.mouseDown(
      document.querySelector(".ant-pagination-options .ant-select-selector")!,
    );
    fireEvent.click(await screen.findByTitle("50 / page"));

    expect(screen.getByText("198.51.100.49")).toBeInTheDocument();
  });
});
