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

// A freshly-placed box: the new map page makes capacity optional when
// creating a node, so this — not a box with a stated size — is the ODP an
// operator in the field is most likely to open first.
const odpUnlimited = {
  id: "odp-2",
  code: "ODP-UNLIMITED",
  portCount: 0,
  usedPorts: 0,
  address: "",
  notes: "",
  routeMeters: 0,
} as Odp;

const subscribers: Ont[] = [
  { id: "ont-9", serialNumber: "ZTEGC0000001", odpPort: 2 } as Ont,
  { id: "ont-self", serialNumber: "ZTEGC0000002", odpPort: 3 } as Ont,
];

const unlimitedSubscribers: Ont[] = [
  { id: "ont-77", serialNumber: "ZTEGC0000077", odpPort: 5 } as Ont,
];

vi.mock("@/application/hooks/useDistribution", () => ({
  useOdps: () => ({ data: [odp, odpUnlimited], isLoading: false }),
  useOdpSubscribers: (odpId?: string) => ({
    data:
      odpId === "odp-1"
        ? subscribers
        : odpId === "odp-2"
          ? unlimitedSubscribers
          : undefined,
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

  // usedPorts on the mock is 1, so a fabricated "free port" count would read
  // "3 port kosong" here — a plausible number nothing actually computed.
  // Only the stated capacity may reach the label.
  it("shows the box's capacity, never a fabricated free-port count", async () => {
    renderFields();

    await userEvent.click(screen.getByRole("combobox", { name: "ODP" }));

    expect(
      await screen.findByTitle("ODP-CARIU-01 · kapasitas 4"),
    ).toBeInTheDocument();
    expect(screen.queryByText(/port kosong/)).not.toBeInTheDocument();
  });

  it("offers exactly as many ports as a box with a stated capacity", async () => {
    renderFields();

    await chooseTheBox();

    expect(screen.getAllByRole("option")).toHaveLength(4);
  });

  // A box placed on the map with no capacity filled in yet still has to be
  // usable: "capacity 0 means unlimited" is this feature's convention
  // everywhere, so there is no fixed list of ports to offer.
  it("lets a port be entered by number when the box has no stated capacity", async () => {
    renderFields();

    await userEvent.click(screen.getByRole("combobox", { name: "ODP" }));
    await userEvent.click(await screen.findByTitle("ODP-UNLIMITED"));

    expect(
      screen.queryByRole("combobox", { name: "Port" }),
    ).not.toBeInTheDocument();
    // The occupant of port 5 is real (useOdpSubscribers), unlike usedPorts.
    expect(await screen.findByText(/5 \(ZTEGC0000077\)/)).toBeInTheDocument();

    await userEvent.type(screen.getByPlaceholderText("Nomor port"), "12");

    expect(screen.getByTestId("chosen")).toHaveTextContent("12");
  });
});
