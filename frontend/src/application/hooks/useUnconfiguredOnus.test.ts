import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createElement } from "react";
import type { Olt } from "@/domain/entities";
import { useUnconfiguredOnus } from "./useUnconfiguredOnus";

const getUnconfiguredOnus = vi.fn();

vi.mock("@/infrastructure/repositories", () => ({
  OltRepository: class {
    getUnconfiguredOnus = (id: string) => getUnconfiguredOnus(id);
  },
}));

function wrapper() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return ({ children }: { children: React.ReactNode }) =>
    createElement(QueryClientProvider, { client }, children);
}

const olt = (id: string, name: string) => ({ id, name }) as Olt;

const onu = (serialNumber: string, slot = 1, port = 1) => ({
  slot,
  port,
  serialNumber,
});

describe("useUnconfiguredOnus", () => {
  beforeEach(() => {
    getUnconfiguredOnus.mockReset();
  });

  it("merges every OLT's scan so nothing waits behind a site picker", async () => {
    // The page used to open on whichever OLT sorted first. When that one had
    // nothing, ONUs ready to register at another site stayed invisible.
    getUnconfiguredOnus.mockImplementation((id: string) =>
      Promise.resolve(id === "o1" ? [] : [onu("ZTEGC0FFEE01")]),
    );

    const { result } = renderHook(
      () => useUnconfiguredOnus([olt("o1", "Empty"), olt("o2", "Busy")]),
      { wrapper: wrapper() },
    );

    await waitFor(() => expect(result.current.rows).toHaveLength(1));
    expect(result.current.rows[0]).toMatchObject({
      serialNumber: "ZTEGC0FFEE01",
      oltId: "o2",
      oltName: "Busy",
    });
  });

  it("orders rows by OLT and physical position so they hold still", async () => {
    getUnconfiguredOnus.mockImplementation((id: string) =>
      Promise.resolve(
        id === "o1"
          ? [onu("SN-B", 2, 1), onu("SN-A", 1, 4)]
          : [onu("SN-C", 1, 1)],
      ),
    );

    const { result } = renderHook(
      () => useUnconfiguredOnus([olt("o1", "Bekasi"), olt("o2", "Depok")]),
      { wrapper: wrapper() },
    );

    await waitFor(() => expect(result.current.rows).toHaveLength(3));
    expect(result.current.rows.map((r) => r.serialNumber)).toEqual([
      "SN-A",
      "SN-B",
      "SN-C",
    ]);
  });

  it("names the OLT it could not scan instead of showing a short list", async () => {
    // An empty table reads as "nothing waiting". When an OLT was never asked,
    // the truth is "we do not know", and the page has to say which one.
    getUnconfiguredOnus.mockImplementation((id: string) =>
      id === "o1"
        ? Promise.reject(new Error("SNMP timeout"))
        : Promise.resolve([onu("ZTEGC0FFEE01")]),
    );

    const { result } = renderHook(
      () => useUnconfiguredOnus([olt("o1", "Bekasi"), olt("o2", "Depok")]),
      { wrapper: wrapper() },
    );

    await waitFor(() => expect(result.current.failed).toEqual(["Bekasi"]));
    expect(result.current.rows).toHaveLength(1);
  });

  it("shows what has arrived rather than waiting for the slowest OLT", async () => {
    getUnconfiguredOnus.mockImplementation((id: string) =>
      id === "o1"
        ? new Promise(() => {}) // never settles
        : Promise.resolve([onu("ZTEGC0FFEE01")]),
    );

    const { result } = renderHook(
      () => useUnconfiguredOnus([olt("o1", "Slow"), olt("o2", "Fast")]),
      { wrapper: wrapper() },
    );

    await waitFor(() => expect(result.current.rows).toHaveLength(1));
    expect(result.current.isLoading).toBe(false);
  });

  it("scans nothing before any OLT is registered", () => {
    const { result } = renderHook(() => useUnconfiguredOnus(undefined), {
      wrapper: wrapper(),
    });

    expect(result.current.rows).toEqual([]);
    expect(result.current.isLoading).toBe(false);
    expect(getUnconfiguredOnus).not.toHaveBeenCalled();
  });
});
