import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { PushOptInButton } from "../PushOptInButton";

describe("PushOptInButton", () => {
  it("asks to enable notifications when permission has not been decided", () => {
    const onEnable = vi.fn();
    render(
      <PushOptInButton
        permission="default"
        requesting={false}
        onEnable={onEnable}
      />,
    );
    fireEvent.click(
      screen.getByRole("button", { name: /aktifkan notifikasi/i }),
    );
    expect(onEnable).toHaveBeenCalledTimes(1);
  });

  it("shows notifications are active and does not offer to enable them again", () => {
    render(
      <PushOptInButton
        permission="granted"
        requesting={false}
        onEnable={vi.fn()}
      />,
    );
    expect(
      screen.getByRole("button", { name: /notifikasi aktif/i }),
    ).toBeDisabled();
  });

  it("says the browser blocked it rather than offering a retry that cannot work", () => {
    render(
      <PushOptInButton
        permission="denied"
        requesting={false}
        onEnable={vi.fn()}
      />,
    );
    expect(
      screen.getByRole("button", { name: /notifikasi diblokir/i }),
    ).toBeDisabled();
  });

  it("renders nothing when the browser cannot do push at all", () => {
    const { container } = render(
      <PushOptInButton
        permission="unsupported"
        requesting={false}
        onEnable={vi.fn()}
      />,
    );
    expect(container).toBeEmptyDOMElement();
  });
});
