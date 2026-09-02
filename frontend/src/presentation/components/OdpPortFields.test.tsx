import { Form } from "antd";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { Odp, Ont } from "@/domain/entities";
import { OdpPortFields } from "./OdpPortFields";

const odp = {
  id: "odp-1",
  code: "ODP-CARIU-01",
  portCount: 4,
  usedPorts: 1,
  address: "",
  notes: "",
  routeMeters: 0,
} as Odp;

const subscribers: Ont[] = [
  { id: "ont-9", serialNumber: "ZTEGC0000001", odpPort: 2 } as Ont,
  { id: "ont-self", serialNumber: "ZTEGC0000002", odpPort: 3 } as Ont,
];

vi.mock("@/application/hooks/useDistribution", () => ({
  useOdps: () => ({ data: [odp], isLoading: false }),
  useOdpSubscribers: (odpId?: string) => ({
    data: odpId === "odp-1" ? subscribers : undefined,
  }),
}));

/** The chosen port is shown as text, so a click's effect can be read directly. */
function renderFields(currentOntId?: string) {
  function Harness() {
    const [form] = Form.useForm();
    const port = Form.useWatch("odpPort", form);
    return (
      <Form form={form} layout="vertical">
        <OdpPortFields currentOntId={currentOntId} />
        <span data-testid="chosen">{port ?? "none"}</span>
      </Form>
    );
  }
  render(<Harness />);
}

async function chooseTheBox() {
  await userEvent.click(screen.getByRole("combobox", { name: "ODP" }));
  await userEvent.click(await screen.findByTitle(/ODP-CARIU-01/));
  await userEvent.click(screen.getByRole("combobox", { name: "Port" }));
}

describe("OdpPortFields", () => {
  it("names who holds a taken port, and will not hand it over", async () => {
    renderFields();

    await chooseTheBox();
    await userEvent.click(
      await screen.findByTitle("Port 2 · dipakai ZTEGC0000001"),
    );

    expect(screen.getByTestId("chosen")).toHaveTextContent("none");
  });

  it("takes a free port", async () => {
    renderFields();

    await chooseTheBox();
    await userEvent.click(await screen.findByTitle("Port 1"));

    expect(screen.getByTestId("chosen")).toHaveTextContent("1");
  });

  // Reopening the form on a subscriber already sitting on a port must not read
  // as that subscriber conflicting with itself.
  it("leaves the ONT being placed its own port", async () => {
    renderFields("ont-self");

    await chooseTheBox();
    await userEvent.click(await screen.findByTitle("Port 3"));

    expect(screen.getByTestId("chosen")).toHaveTextContent("3");
  });

  it("offers no port until a box is chosen", () => {
    renderFields();

    expect(
      screen.queryByRole("combobox", { name: "Port" }),
    ).not.toBeInTheDocument();
  });
});
