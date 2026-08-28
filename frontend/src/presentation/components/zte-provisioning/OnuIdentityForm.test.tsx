import { render, screen } from "@testing-library/react";
import { Form, type FormInstance } from "antd";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { OnuIdentityForm } from "./OnuIdentityForm";
import type { ZteProvisionTarget } from "@/domain/entities";

const useOltOnuTypes = vi.hoisted(() => vi.fn());

vi.mock("@/application/hooks/useOlts", () => ({ useOltOnuTypes }));

const target: ZteProvisionTarget = {
  oltId: "olt-1",
  card: 3,
  pon: 6,
  serialNumber: "RTEGC609DA61",
  // What the ONU announces over OMCI, which is not a type the OLT accepts.
  onuType: "F609V9",
};

beforeEach(() => {
  useOltOnuTypes.mockReturnValue({ data: [] });
});

// Validation is driven through the form instance rather than a click: antd's
// stylesheet makes jsdom's selector engine throw on pointer interaction, and
// the rule is what these tests are about.
function renderForm(
  onuType?: string,
  mode: "register" | "configure" = "register",
) {
  const captured: { form?: FormInstance } = {};

  function Harness() {
    const [form] = Form.useForm();
    captured.form = form;
    return (
      <Form form={form} initialValues={{ onuIdMode: "auto", onuType }}>
        <OnuIdentityForm target={target} mode={mode} />
      </Form>
    );
  }

  render(<Harness />);
  return captured;
}

describe("OnuIdentityForm", () => {
  it("offers the types the OLT accepts and names the detected model", () => {
    useOltOnuTypes.mockReturnValue({ data: ["ZTEG-F609", "HG8245H5"] });

    renderForm();

    expect(screen.getByText("Select an ONU type")).toBeInTheDocument();
    expect(
      screen.getByText(/The OLT reports this ONU as F609V9/),
    ).toBeInTheDocument();
  });

  // Registering with the reported model fails on the OLT with an error that
  // names the wrong thing, so the form refuses it first.
  it("rejects a type the OLT does not accept", async () => {
    useOltOnuTypes.mockReturnValue({ data: ["ZTEG-F609"] });

    const captured = renderForm("F609V9");

    await expect(
      captured.form?.validateFields(["onuType"]),
    ).rejects.toMatchObject({
      errorFields: [{ errors: ["The OLT does not accept the type F609V9."] }],
    });
  });

  it("accepts a type from the list", async () => {
    useOltOnuTypes.mockReturnValue({ data: ["ZTEG-F609"] });

    const captured = renderForm("ZTEG-F609");

    await expect(
      captured.form?.validateFields(["onuType"]),
    ).resolves.toMatchObject({ onuType: "ZTEG-F609" });
  });

  // An OLT that has not been polled yet leaves the operator to type it, and
  // then nothing may be rejected on a list that is not there.
  it("falls back to a typed ONU type when nothing is cached", async () => {
    const captured = renderForm("F609V9");

    expect(
      screen.getByText("ONU types appear here once the OLT has been polled."),
    ).toBeInTheDocument();
    await expect(
      captured.form?.validateFields(["onuType"]),
    ).resolves.toMatchObject({ onuType: "F609V9" });
  });

  // Configuring an ONU the OLT already knows never sends "onu N type X sn Y",
  // so holding the operator to the OLT's list there blocks them on a field
  // that goes nowhere.
  it("does not police the type when configuring an existing ONU", async () => {
    useOltOnuTypes.mockReturnValue({ data: ["ZTEG-F609"] });

    const captured = renderForm("HWTC", "configure");

    await expect(
      captured.form?.validateFields(["onuType"]),
    ).resolves.toMatchObject({ onuType: "HWTC" });
    expect(
      screen.getByText(
        "Not sent when configuring an ONU the OLT already knows.",
      ),
    ).toBeInTheDocument();
  });
});
