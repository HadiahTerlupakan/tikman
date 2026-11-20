import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SwitchHostKeyBanner } from "./SwitchHostKeyBanner";
import { fixtureSwitch } from "./switchFixtures";

const trust = vi.fn();
vi.mock("@/application/hooks/useSwitches", () => ({
  useTrustSwitchHostKey: () => ({ mutate: trust, isPending: false }),
}));

const changed = fixtureSwitch({
  hostKeyChanged: true,
  presentedFingerprint: "abc123+/FP",
});

describe("SwitchHostKeyBanner", () => {
  it("shows nothing while the key is as pinned", () => {
    const { container } = render(
      <SwitchHostKeyBanner sw={fixtureSwitch()} isAdmin />,
    );
    expect(container.textContent).toBe("");
  });

  it("shows the presented fingerprint and lets an admin trust it", async () => {
    render(<SwitchHostKeyBanner sw={changed} isAdmin />);
    expect(screen.getByText("abc123+/FP")).toBeTruthy();
    await userEvent.click(screen.getByRole("button", { name: "Percayai" }));
    expect(trust).toHaveBeenCalledWith("s-1");
  });

  it("gives other roles the fingerprint but no button", () => {
    render(<SwitchHostKeyBanner sw={changed} isAdmin={false} />);
    expect(screen.getByText("abc123+/FP")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Percayai" })).toBeNull();
  });
});
