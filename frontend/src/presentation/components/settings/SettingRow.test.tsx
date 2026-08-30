import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { SettingRow } from "./SettingRow";

const CONFIGURED = {
  name: "google_maps_api_key",
  label: "Google Maps API key",
  description: "Enables the site map.",
  configured: true,
  preview: "AIza••••••••Y123",
  updatedAt: "2026-08-30T02:00:00.000Z",
};

describe("SettingRow", () => {
  it("shows the masked preview and never a full value", () => {
    render(
      <SettingRow setting={CONFIGURED} onEdit={vi.fn()} onDelete={vi.fn()} />,
    );

    expect(screen.getByText("Google Maps API key")).toBeInTheDocument();
    expect(screen.getByText("AIza••••••••Y123")).toBeInTheDocument();
  });

  it("says a setting is not configured rather than showing an empty box", () => {
    render(
      <SettingRow
        setting={{
          ...CONFIGURED,
          configured: false,
          preview: "",
          updatedAt: undefined,
        }}
        onEdit={vi.fn()}
        onDelete={vi.fn()}
      />,
    );

    expect(screen.getByText("Not configured")).toBeInTheDocument();
  });

  it("offers removal only for a setting that has a value", () => {
    // A delete button on an unset setting does nothing and invites a click.
    const { rerender } = render(
      <SettingRow setting={CONFIGURED} onEdit={vi.fn()} onDelete={vi.fn()} />,
    );
    expect(screen.getByRole("button", { name: /remove/i })).toBeInTheDocument();

    rerender(
      <SettingRow
        setting={{ ...CONFIGURED, configured: false, preview: "" }}
        onEdit={vi.fn()}
        onDelete={vi.fn()}
      />,
    );
    expect(
      screen.queryByRole("button", { name: /remove/i }),
    ).not.toBeInTheDocument();
  });

  it("says a value cannot be read, and still offers to remove it", async () => {
    // After an ENCRYPTION_KEY rotation the stored row is undecryptable. Showing
    // a blank preview would leave the operator with nothing to act on.
    const onDelete = vi.fn();
    render(
      <SettingRow
        setting={{ ...CONFIGURED, preview: "", unreadable: true }}
        onEdit={vi.fn()}
        onDelete={onDelete}
      />,
    );

    expect(screen.getByText(/not readable/i)).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: /remove/i }));
    expect(onDelete).toHaveBeenCalled();
  });

  it("asks to edit the setting it was given", async () => {
    const onEdit = vi.fn();
    render(
      <SettingRow setting={CONFIGURED} onEdit={onEdit} onDelete={vi.fn()} />,
    );

    await userEvent.click(screen.getByRole("button", { name: /replace/i }));

    expect(onEdit).toHaveBeenCalledWith(CONFIGURED);
  });
});
