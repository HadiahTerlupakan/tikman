import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import type {
  MikrotikRouter,
  RouterBgpSession,
  RouterInterface,
} from "@/domain/entities";
import { Chassis } from "./Chassis";
import { layoutFaceplate } from "./portLayout";
import { buildPortViews } from "./portModel";
import { type HealthView, healthView } from "./healthModel";
import { mikrotikRouter } from "@/test/mikrotikRouterFixture";

const router = { host: "192.0.2.1" } as MikrotikRouter;

const iface = (over: Partial<RouterInterface>): RouterInterface =>
  ({
    id: over.name,
    name: "ether1",
    comment: "",
    type: "ether",
    parentName: null,
    disabled: false,
    operStatus: "up",
    addresses: "",
    lastInBps: null,
    lastOutBps: null,
    silentSince: null,
    ...over,
  }) as RouterInterface;

const transit = {
  id: "s1",
  name: "peer-transit",
  remoteAddress: "192.0.2.9",
  state: "established",
  interfaceId: "sfp-sfpplus1",
} as RouterBgpSession;

const interfaces = [
  iface({ name: "ether1", addresses: "192.0.2.1/24", lastInBps: 2e6 }),
  iface({ name: "ether3", comment: "pop-north", silentSince: "2026-11-19" }),
  iface({ name: "ether4", operStatus: "down" }),
  iface({ name: "sfp-sfpplus1", comment: "transit", lastInBps: 3e9 }),
];

const ports = buildPortViews(router, interfaces, [transit]);
const templated = layoutFaceplate("CCR2116-12G-4S+", []);

const NO_HEALTH: HealthView = {
  fresh: false,
  psus: [],
  failures: [],
  readings: [],
};
const NOW = new Date("2026-11-19T12:00:00Z");
const health = (over: Parameters<typeof mikrotikRouter>[0]) =>
  healthView(
    mikrotikRouter({ healthAt: "2026-11-19T11:59:00Z", ...over }),
    NOW,
  );
const psu2Failed = health({
  health: [
    { name: "cpu-temperature", value: 43, unit: 1 },
    { name: "psu1-state", value: 0, unit: 6 },
    { name: "psu2-state", value: 1, unit: 6 },
  ],
});
const ledOf = (label: string) =>
  screen.getByText(label).querySelector("i")?.className ?? "";

describe("Chassis", () => {
  it("makes a button of every used port and nothing of unused ones", () => {
    render(
      <Chassis
        layout={templated}
        ports={ports}
        offline={false}
        health={NO_HEALTH}
        onSelect={vi.fn()}
      />,
    );

    const names = screen
      .getAllByRole("button")
      .map((b) => b.getAttribute("aria-label"));
    expect(names).toEqual([
      "ether1, ether1, Aktif",
      "ether3, pop-north, Kemungkinan down",
      "sfp-sfpplus1, transit, Aktif",
    ]);
  });

  it("names a port on hover when there is nothing to select", () => {
    const { container } = render(
      <Chassis
        layout={templated}
        ports={ports}
        offline={false}
        health={NO_HEALTH}
      />,
    );

    const port = container.querySelector('[data-port="sfp-sfpplus1"]');
    expect(port?.getAttribute("title")).toBe(
      "sfp-sfpplus1 · transit · Aktif · ↓ 3.00 Gbps",
    );
  });

  it("gives no port a tab stop when there is nothing to select", () => {
    const { container } = render(
      <Chassis
        layout={templated}
        ports={ports}
        offline={false}
        health={NO_HEALTH}
      />,
    );

    expect(screen.queryAllByRole("button")).toEqual([]);
    expect(container.querySelector("button, [tabindex]")).toBeNull();
  });

  it("names no state or rate it cannot read from an unreachable router", () => {
    const { container } = render(
      <Chassis layout={templated} ports={ports} offline health={NO_HEALTH} />,
    );
    const port = container.querySelector('[data-port="sfp-sfpplus1"]');
    expect(port?.getAttribute("title")).toBe(
      "sfp-sfpplus1 · transit · Tidak terjangkau",
    );
  });

  it("tells a screen reader the router is unreachable", () => {
    render(
      <Chassis
        layout={templated}
        ports={ports}
        offline
        health={NO_HEALTH}
        onSelect={vi.fn()}
      />,
    );
    expect(
      screen
        .getByRole("button", { name: /^sfp-sfpplus1/ })
        .getAttribute("aria-label"),
    ).toBe("sfp-sfpplus1, transit, Tidak terjangkau");
  });

  it("lights LEDs by state and blinks the busy port", () => {
    const { container } = render(
      <Chassis
        layout={templated}
        ports={ports}
        offline={false}
        health={NO_HEALTH}
      />,
    );

    const leds = (name: string) =>
      container.querySelector(`[data-port="${name}"]`)?.querySelectorAll("i");
    expect(leds("sfp-sfpplus1")?.[0].className).toContain("fp-led-up");
    expect(leds("ether3")?.[0].className).toContain("fp-led-warn");
    expect(ledOf("STATUS")).toBe("fp-led-up");
    const blink = container.querySelector<HTMLElement>(
      '[data-port="sfp-sfpplus1"] .fp-led-act',
    );
    expect(blink?.style.getPropertyValue("--fp-blink")).toMatch(/^0\.\d+s$/);
  });

  it("keeps an unused port dark even when its link is down", () => {
    const { container } = render(
      <Chassis
        layout={templated}
        ports={ports}
        offline={false}
        health={NO_HEALTH}
      />,
    );

    const unused = container.querySelector('[data-port="ether4"]');
    expect(unused?.getAttribute("aria-hidden")).toBe("true");
    expect(unused?.querySelector(".fp-led-down")).toBeNull();
  });

  it("turns every LED off for an unreachable router", () => {
    const { container } = render(
      <Chassis layout={templated} ports={ports} offline health={NO_HEALTH} />,
    );

    expect(container.querySelector(".fp-led-up")).toBeNull();
    expect(container.querySelector(".fp-led-warn")).toBeNull();
    expect(container.querySelector(".fp-led-act")).toBeNull();
  });

  it("selects a port by its name", () => {
    const onSelect = vi.fn();
    render(
      <Chassis
        layout={templated}
        ports={ports}
        offline={false}
        health={NO_HEALTH}
        selected="ether3"
        onSelect={onSelect}
      />,
    );

    const port = screen.getByRole("button", { name: /^sfp-sfpplus1/ });
    expect(port.getAttribute("title")).toBeNull();
    fireEvent.click(port);
    expect(onSelect).toHaveBeenCalledWith("sfp-sfpplus1");
    expect(screen.getByRole("button", { name: /^ether3/ }).className).toContain(
      "fp-selected",
    );
  });

  it("says when the layout was made from interface names", () => {
    const layout = layoutFaceplate("RB5009UG+S+IN", ["ether1", "ether2"]);
    const { container } = render(
      <Chassis
        layout={layout}
        ports={ports}
        offline={false}
        health={NO_HEALTH}
      />,
    );

    expect(screen.getByText("RB5009UG+S+IN · susunan otomatis")).toBeTruthy();
    expect(container.querySelector(".fp-ear")).toBeNull();
  });

  it("keeps the silkscreen model clear of the last jack", () => {
    const layout = layoutFaceplate("CCR2216-1G-12XS-2XQ", [
      "ether1",
      ...Array.from({ length: 12 }, (_, i) => `sfp28-${i + 1}`),
      "qsfp28-1-1",
      "qsfp28-2-1",
    ]);
    render(
      <Chassis
        layout={layout}
        ports={ports}
        offline={false}
        health={NO_HEALTH}
      />,
    );

    const model = screen.getByText("CCR2216-1G-12XS-2XQ · susunan otomatis");
    const lastJack = Math.max(...layout.jacks.map((j) => j.x + j.w));
    const left =
      layout.width -
      parseFloat(model.style.right) -
      parseFloat(model.style.maxWidth);
    expect(left).toBeGreaterThan(lastJack);
    expect(model.style.textOverflow).toBe("ellipsis");
  });

  it("lights the failed PSU and FAULT red and lists the readings on hover", () => {
    const { container } = render(
      <Chassis
        layout={templated}
        ports={ports}
        offline={false}
        health={psu2Failed}
      />,
    );

    expect(ledOf("PWR1")).toBe("fp-led-up");
    expect(ledOf("PWR2")).toBe("fp-led-down");
    expect(ledOf("FAULT")).toBe("fp-led-down");
    expect(container.querySelector(".fp-leds")?.getAttribute("title")).toBe(
      "Suhu CPU 43 °C · psu1-state ok · psu2-state fail",
    );
  });

  it("draws no PWR LED for a board without PSU state and keeps FAULT dark", () => {
    const rb5009 = health({
      health: [{ name: "cpu-temperature", value: 51, unit: 1 }],
    });
    render(
      <Chassis
        layout={templated}
        ports={ports}
        offline={false}
        health={rb5009}
      />,
    );

    expect(screen.queryByText(/^PWR/)).toBeNull();
    expect(ledOf("FAULT")).toBe("");
  });

  it("leaves a PWR LED dark when its PSU state is neither ok nor fail", () => {
    const unknown = health({
      health: [
        { name: "psu1-state", value: 0, unit: 6 },
        { name: "psu2-state", value: 2, unit: 6 },
      ],
    });
    render(
      <Chassis
        layout={templated}
        ports={ports}
        offline={false}
        health={unknown}
      />,
    );

    expect(ledOf("PWR1")).toBe("fp-led-up");
    expect(ledOf("PWR2")).toBe("");
    expect(ledOf("FAULT")).toBe("");
  });

  it("lights no health LED from a reading that is not fresh", () => {
    const stale = { ...psu2Failed, fresh: false };
    const { container } = render(
      <Chassis layout={templated} ports={ports} offline health={stale} />,
    );

    expect(ledOf("STATUS")).toBe("");
    expect(ledOf("PWR1")).toBe("");
    expect(ledOf("PWR2")).toBe("");
    expect(ledOf("FAULT")).toBe("");
    expect(
      container.querySelector(".fp-leds")?.getAttribute("title"),
    ).toBeNull();
  });
});
