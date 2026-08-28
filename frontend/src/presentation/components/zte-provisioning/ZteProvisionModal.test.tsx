import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { fireEvent } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { OntServiceConfig } from "@/domain/entities";
import { ZteProvisionModal } from "./ZteProvisionModal";

// The service step reads the OLT's cached VLAN list. These tests are about the
// form, not the fetch, so the list is stubbed rather than served by a client.
const useOntServiceConfig = vi.hoisted(() =>
  vi.fn<() => { data: OntServiceConfig | null }>(() => ({ data: null })),
);

vi.mock("@/application/hooks/useOnts", () => ({ useOntServiceConfig }));

vi.mock("@/application/hooks/useOlts", () => ({
  useOltVlans: () => ({ data: [] }),
  useOltTcontProfiles: () => ({ data: [] }),
  useOltVlanProfiles: () => ({ data: [] }),
  useOltOnuTypes: () => ({ data: [] }),
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

// The service step is required in full before the wizard will advance, so both
// tests that need the review step have to fill it.
async function fillInternetService(password: string) {
  await userEvent.type(screen.getByLabelText(/VLAN ID/i), "214");
  await userEvent.type(screen.getByLabelText(/Download profile/i), "1G");
  await userEvent.type(screen.getByLabelText(/Upload profile/i), "1G");
  await userEvent.type(screen.getByLabelText(/VLAN profile/i), "PPPOE-214");
  await userEvent.type(screen.getByLabelText(/PPPoE username/i), "user");
  await userEvent.type(screen.getByLabelText(/^PPPoE password$/i), password);
}

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

    // Auto has no ID to show yet, so it says so rather than displaying the
    // zero it sends for the allocator to replace.
    expect(screen.getByDisplayValue("Assigned automatically")).toBeDisabled();

    fireEvent.click(screen.getByRole("radio", { name: "Custom" }));

    expect(
      screen.queryByDisplayValue("Assigned automatically"),
    ).not.toBeInTheDocument();
    expect(screen.getByLabelText(/^ONU ID$/i)).toBeEnabled();
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
    await fillInternetService("secret-pass");
    await user.click(screen.getByRole("button", { name: "Next" }));

    expect(screen.getByText(/password <redacted>/i)).toBeInTheDocument();
    expect(screen.queryByText("secret-pass")).not.toBeInTheDocument();
  });

  // Configuring an existing ONT opens on what it is already running, so the
  // operator changes one field instead of retyping the whole service.
  it("opens pre-filled with the ONT's stored service", async () => {
    useOntServiceConfig.mockReturnValue({
      data: {
        vlanId: 214,
        vlanMode: "tag",
        serviceType: "internet",
        tcontProfile: "1G",
        wanMode: "wan_ip",
        wanIpMode: "pppoe",
        vlanProfile: "PPPOE-214",
        pppoeUsername: "258179206252",
        pppoePassword: "12345",
      },
    });

    render(
      <ZteProvisionModal
        open
        mode="configure"
        target={target}
        ontId="ont-1"
        onClose={vi.fn()}
        onSubmit={vi.fn()}
      />,
    );

    await userEvent.click(screen.getByRole("button", { name: "Next" }));

    expect(screen.getByLabelText(/PPPoE username/i)).toHaveValue(
      "258179206252",
    );
    // Without this the operator would need a password the OLT already has, and
    // a reconfigure that omitted it would break the subscriber's session.
    expect(screen.getByLabelText(/^PPPoE password$/i)).toHaveValue("12345");
  });

  // The wizard unmounts each step as it advances, and antd only returns the
  // fields still mounted. Submitting from the review step therefore has to
  // carry the card and PON entered on the first one.
  it("submits the identity entered on the first step", async () => {
    const onSubmit = vi.fn();
    render(
      <ZteProvisionModal
        open
        mode="register"
        target={target}
        onClose={vi.fn()}
        onSubmit={onSubmit}
      />,
    );

    await userEvent.click(screen.getByRole("button", { name: "Next" }));
    await fillInternetService("pass");
    await userEvent.click(screen.getByRole("button", { name: "Next" }));
    await userEvent.click(screen.getByRole("switch"));
    await userEvent.click(screen.getByRole("button", { name: "Submit" }));

    expect(onSubmit).toHaveBeenCalled();
    expect(onSubmit.mock.calls[0][0]).toMatchObject({ card: 3, pon: 1 });
  });
});
