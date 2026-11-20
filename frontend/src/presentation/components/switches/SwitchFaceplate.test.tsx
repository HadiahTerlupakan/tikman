import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SwitchFaceplate } from "./SwitchFaceplate";
import { fixturePort, fixtureSwitch } from "./switchFixtures";

const ports = [
  fixturePort({ name: "MEth0/0/0", kind: "rj" }),
  fixturePort({ name: "10GE1/0/1", alias: "ke-olt" }),
  fixturePort({ name: "10GE1/0/2", alias: "ke-pop", operStatus: "down" }),
];

vi.mock("@/application/hooks/useSwitches", () => ({
  useSwitchInterfaces: () => ({
    data: ports,
    isLoading: false,
    isError: false,
  }),
}));

describe("SwitchFaceplate", () => {
  it("draws the template with a detail panel for the port picked", async () => {
    const { container } = render(<SwitchFaceplate sw={fixtureSwitch()} />);
    expect(container.querySelectorAll("[data-port]").length).toBeGreaterThan(
      40,
    );

    await userEvent.click(container.querySelector('[data-port="10GE1/0/2"]')!);
    expect(await screen.findByText("Mati")).toBeTruthy();
  });
});
