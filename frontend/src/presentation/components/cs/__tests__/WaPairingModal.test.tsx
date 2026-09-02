import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { WaPairingModal } from "../WaPairingModal";

// The disconnect route has been complete and admin-only since the module
// landed, and the modal offered only Sambungkan — a number could be paired
// and never given up from the page that pairs it.
describe("WaPairingModal", () => {
  it("gives the number up once the admin confirms", async () => {
    const onDisconnect = vi.fn();
    render(
      <WaPairingModal
        open
        onClose={vi.fn()}
        status="connected"
        accountId="a1"
        connecting={false}
        onConnect={vi.fn()}
        disconnecting={false}
        onDisconnect={onDisconnect}
      />,
    );

    await userEvent.click(screen.getByRole("button", { name: /^putuskan$/i }));
    await userEvent.click(
      await screen.findByRole("button", { name: /ya, putuskan/i }),
    );

    expect(onDisconnect).toHaveBeenCalledTimes(1);
  });

  // Sambungkan already waits for the account list; Putuskan has to as well,
  // or it posts to /wa-accounts/undefined/disconnect.
  it("stays disabled until there is an account to disconnect", () => {
    render(
      <WaPairingModal
        open
        onClose={vi.fn()}
        connecting={false}
        onConnect={vi.fn()}
        disconnecting={false}
        onDisconnect={vi.fn()}
      />,
    );

    expect(screen.getByRole("button", { name: /^putuskan$/i })).toBeDisabled();
  });
});
