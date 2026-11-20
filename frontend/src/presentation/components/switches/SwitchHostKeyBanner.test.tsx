import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SwitchHostKeyBanner } from "./SwitchHostKeyBanner";
import { fixtureSwitch } from "./switchFixtures";
import { ApiError } from "@/infrastructure/http/errorMapper";

const trust = vi.fn();
let trustFailure: unknown = null;
vi.mock("@/application/hooks/useSwitches", () => ({
  useTrustSwitchHostKey: () => ({
    mutate: trust,
    isPending: false,
    error: trustFailure,
  }),
}));

const changed = fixtureSwitch({
  hostKeyChanged: true,
  hostKeyFingerprint: "SHA256:old",
  presentedFingerprint: "abc123+/FP",
});

const unpinned = fixtureSwitch({
  hostKeyFingerprint: "",
  presentedFingerprint: "SHA256:first",
});

describe("SwitchHostKeyBanner", () => {
  beforeEach(() => {
    trust.mockClear();
    trustFailure = null;
  });

  it("shows nothing while the key is as pinned", () => {
    const { container } = render(
      <SwitchHostKeyBanner
        sw={fixtureSwitch({
          hostKeyFingerprint: "SHA256:same",
          presentedFingerprint: "SHA256:same",
        })}
        isAdmin
      />,
    );
    expect(container.textContent).toBe("");
  });

  it("shows nothing for telnet, which has no host key", () => {
    const { container } = render(
      <SwitchHostKeyBanner
        sw={fixtureSwitch({
          protocol: "telnet",
          presentedFingerprint: "SHA256:old",
        })}
        isAdmin
      />,
    );
    expect(container.textContent).toBe("");
  });

  it("trusts exactly the fingerprint it shows when the key changed", async () => {
    render(<SwitchHostKeyBanner sw={changed} isAdmin />);
    expect(screen.getByText(/Kunci SSH switch berubah/)).toBeTruthy();
    expect(screen.getByText("abc123+/FP")).toBeTruthy();
    await userEvent.click(screen.getByRole("button", { name: "Percayai" }));
    expect(trust).toHaveBeenCalledWith({
      id: "s-1",
      fingerprint: "abc123+/FP",
    });
  });

  it("offers trust for a switch nobody has pinned yet", async () => {
    render(<SwitchHostKeyBanner sw={unpinned} isAdmin />);
    expect(screen.getByText(/belum dipercaya/)).toBeTruthy();
    await userEvent.click(screen.getByRole("button", { name: "Percayai" }));
    expect(trust).toHaveBeenCalledWith({
      id: "s-1",
      fingerprint: "SHA256:first",
    });
  });

  it("tells the admin to reload when the switch now presents another key", () => {
    trustFailure = new ApiError(409, "HOST_KEY_NOT_PRESENTED");
    render(<SwitchHostKeyBanner sw={unpinned} isAdmin />);
    expect(screen.getByText(/Muat ulang halaman/)).toBeTruthy();
  });

  it("gives other roles the fingerprint but no button", () => {
    render(<SwitchHostKeyBanner sw={changed} isAdmin={false} />);
    expect(screen.getByText("abc123+/FP")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Percayai" })).toBeNull();
  });
});
