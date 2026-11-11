import dayjs from "dayjs";
import { describe, expect, it } from "vitest";
import {
  DEFAULT_GRAPH_REPORT_MESSAGE,
  type GraphReport,
  type WaGroup,
} from "@/domain/entities";
import {
  defaultPortLabel,
  deviceOptions,
  emptyFormValues,
  formatSendTime,
  formatSentOn,
  groupOptions,
  portOptionLabel,
  reportCaption,
  reportToFormValues,
  reportWithEnabled,
  toGraphReportInput,
  toPreviewInput,
} from "./graphReportForm";

const report: GraphReport = {
  id: "r1",
  waGroupId: "g1",
  groupName: "TEST MRTG",
  customerName: "PT Contoh",
  message: "Halo {pelanggan}",
  enabled: true,
  mentionAll: true,
  sendTime: "14:30",
  lastSentOn: null,
  lastAttemptAt: null,
  lastError: "",
  ports: [
    {
      id: "p2",
      deviceId: 7,
      deviceName: "core",
      ifName: "sfp2",
      label: "Kedua",
      position: 1,
    },
    {
      id: "p1",
      deviceId: 7,
      deviceName: "core",
      ifName: "ether 1/1",
      label: "Pertama",
      position: 0,
    },
  ],
};

const group = (id: string, extra: Partial<WaGroup> = {}) =>
  ({
    id,
    name: `Grup ${id}`,
    jid: `${id}@g.us`,
    left: false,
    ...extra,
  }) as WaGroup;

describe("toGraphReportInput", () => {
  it("trims names and keeps the ports in the order they will be sent", () => {
    const input = toGraphReportInput({
      waGroupId: "g1",
      customerName: "  PT Contoh ",
      message: "  Halo {pelanggan}\n ",
      enabled: false,
      mentionAll: true,
      sendTime: dayjs("2026-11-11T07:05:00"),
      ports: [
        { deviceId: 3, deviceName: "edge", ifName: "ether2", label: " B " },
        { deviceId: 7, deviceName: "core", ifName: "ether 1/1", label: "A" },
      ],
    });

    expect(input).toEqual({
      waGroupId: "g1",
      customerName: "PT Contoh",
      message: "Halo {pelanggan}",
      enabled: false,
      mentionAll: true,
      sendTime: "07:05",
      ports: [
        { deviceId: 3, deviceName: "edge", ifName: "ether2", label: "B" },
        { deviceId: 7, deviceName: "core", ifName: "ether 1/1", label: "A" },
      ],
    });
  });
});

describe("toPreviewInput", () => {
  it("sends no group, so a preview works before one is chosen", () => {
    const input = toPreviewInput({
      customerName: "PT Contoh",
      ports: [{ deviceId: 7, deviceName: "core", ifName: "sfp1", label: "A" }],
    });

    expect(input).toEqual({
      customerName: "PT Contoh",
      message: "",
      ports: [{ deviceId: 7, deviceName: "core", ifName: "sfp1", label: "A" }],
    });
  });
});

describe("reportCaption", () => {
  it("puts the customer name in every {pelanggan}", () => {
    expect(reportCaption("{pelanggan}: grafik {pelanggan}", "PT A")).toBe(
      "PT A: grafik PT A",
    );
  });

  it("falls back to the default message when the message is blank", () => {
    const caption = reportCaption("  \n", "PT Contoh");

    expect(caption).toBe(
      DEFAULT_GRAPH_REPORT_MESSAGE.replace("{pelanggan}", "PT Contoh"),
    );
    expect(caption).toContain("untuk PT Contoh.");
  });
});

describe("reportToFormValues", () => {
  it("orders the ports by position, not by the order the API listed them", () => {
    const values = reportToFormValues(report);

    expect(values.ports.map((p) => p.label)).toEqual(["Pertama", "Kedua"]);
    expect(values.ports[0]).not.toHaveProperty("id");
  });

  it("carries the saved tag-everyone choice into the form", () => {
    expect(reportToFormValues(report).mentionAll).toBe(true);
    expect(
      reportToFormValues({ ...report, mentionAll: false }).mentionAll,
    ).toBe(false);
  });
});

describe("send time", () => {
  it("round-trips the saved HH:MM through the TimePicker's value", () => {
    const values = reportToFormValues(report);

    expect(formatSendTime(values.sendTime)).toBe("14:30");
    expect(toGraphReportInput(values).sendTime).toBe("14:30");
  });

  it("zero-pads early hours, since the API accepts only two digits each", () => {
    const values = reportToFormValues({ ...report, sendTime: "00:05" });

    expect(toGraphReportInput(values).sendTime).toBe("00:05");
  });

  it("starts a new report at 06:00", () => {
    expect(formatSendTime(emptyFormValues().sendTime)).toBe("06:00");
  });
});

describe("emptyFormValues", () => {
  // Tagging everyone reaches members who muted the group, so it is opt-in.
  it("starts a new report without tagging everyone", () => {
    expect(emptyFormValues().mentionAll).toBe(false);
  });
});

describe("reportWithEnabled", () => {
  it("resends every other field unchanged when only Aktif flips", () => {
    const input = reportWithEnabled(report, false);

    expect(input.enabled).toBe(false);
    expect(input.customerName).toBe("PT Contoh");
    expect(input.message).toBe("Halo {pelanggan}");
    expect(input.mentionAll).toBe(true);
    expect(input.sendTime).toBe("14:30");
    expect(input.ports.map((p) => p.ifName)).toEqual(["ether 1/1", "sfp2"]);
  });
});

describe("port labels", () => {
  const port = { portId: 1, ifName: "sfp1", ifAlias: "", operStatus: "up" };

  it("defaults the label to the interface description when there is one", () => {
    expect(defaultPortLabel({ ...port, ifAlias: " Uplink PT Contoh " })).toBe(
      "Uplink PT Contoh",
    );
    expect(defaultPortLabel(port)).toBe("sfp1");
  });

  it("shows ifName and ifAlias together in the picker", () => {
    expect(portOptionLabel({ ...port, ifAlias: "Uplink" })).toBe(
      "sfp1 — Uplink",
    );
    expect(portOptionLabel(port)).toBe("sfp1");
  });
});

describe("deviceOptions", () => {
  it("keeps a saved device LibreNMS did not list, under its stored name", () => {
    const options = deviceOptions(
      [{ deviceId: 1, hostname: "10.0.0.1", sysName: "core" }],
      { deviceId: 9, deviceName: "old-edge" },
    );

    expect(options).toEqual([
      { value: 1, label: "core (10.0.0.1)" },
      { value: 9, label: "old-edge" },
    ]);
  });
});

describe("groupOptions", () => {
  it("leaves out groups the number left and groups that already have a report", () => {
    const groups = [group("g1"), group("g2", { left: true }), group("g3")];

    expect(groupOptions(groups, [report]).map((o) => o.value)).toEqual(["g3"]);
  });

  it("leaves out admins-only groups unless the number is an admin there", () => {
    const groups = [
      group("g2", { announce: true }),
      group("g3", { announce: true, isAdmin: true }),
    ];

    expect(groupOptions(groups, []).map((o) => o.value)).toEqual(["g3"]);
  });

  it("keeps the report's own group while it is being edited", () => {
    expect(groupOptions([group("g1")], [report], "g1")).toEqual([
      { value: "g1", label: "Grup g1" },
    ]);
  });
});

describe("formatSentOn", () => {
  it("does not shift a WIB date by the browser's zone", () => {
    expect(formatSentOn("2026-11-11")).toMatch(/^11 Nov/);
    expect(formatSentOn(null)).toBe("Belum pernah");
  });
});
