import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { fireEvent } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ZteProvisionModal } from "./ZteProvisionModal";

// The service step reads the OLT's cached VLAN list. These tests are about the
// form, not the fetch, so the list is stubbed rather than served by a client.
vi.mock("@/application/hooks/useOlts", () => ({
  useOltVlans: () => ({ data: [] }),
  useOltTcontProfiles: () => ({ data: [] }),
  useOltVlanProfiles: () => ({ data: [] }),
}));

const target = {
  oltId: "olt-1",
  oltModel: "zte_c300" as const,
  card: 3,
  pon: 1,
  serialNumber: "HWTCB403E8A0",
  onuId: 7,
  onuType: "HG8245H5",
};

describe("ZteProvisionModal", () => {
  it("renders identity and one Internet service form", async () => {
    render(
      <ZteProvisionModal
        open
        mode="register"
        target={target}
        onClose={vi.fn()}
        onSubmit={vi.fn()}
      />,
    );

    expect(screen.getByLabelText(/card/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/^PON$/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/serial number/i)).toHaveValue("HWTCB403E8A0");
    await userEvent.click(screen.getByRole("button", { name: "Next" }));
    expect(screen.getByText("Service 1")).toBeInTheDocument();
    expect(screen.getByLabelText(/^PPPoE password$/i)).toBeInTheDocument();
  });

  it("switches between auto and custom ONU ID", () => {
    render(
      <ZteProvisionModal
        open
        mode="register"
        target={target}
        onClose={vi.fn()}
        onSubmit={vi.fn()}
      />,
    );

    expect(screen.getByLabelText(/ONU ID/i)).toBeDisabled();
    fireEvent.click(screen.getByRole("radio", { name: "Custom" }));
    expect(screen.getByLabelText(/ONU ID/i)).toBeEnabled();
  });

  it("never puts the PPPoE password in the command preview", async () => {
    const user = userEvent.setup();
    render(
      <ZteProvisionModal
        open
        mode="register"
        target={target}
        onClose={vi.fn()}
        onSubmit={vi.fn()}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Next" }));
    await user.type(screen.getByLabelText(/^PPPoE password$/i), "secret-pass");
    await user.click(screen.getByRole("button", { name: "Next" }));
    expect(screen.getByText(/password <redacted>/i)).toBeInTheDocument();
    expect(screen.queryByText("secret-pass")).not.toBeInTheDocument();
  });
});
