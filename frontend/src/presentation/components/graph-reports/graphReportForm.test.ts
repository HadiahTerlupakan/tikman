import dayjs from "dayjs";
import { describe, expect, it } from "vitest";
import {
  type GraphReport,
  type TelegramChat,
  type WaAccount,
  type WaGroup,
} from "@/domain/entities";
import {
  accountOptions,
  defaultPortLabel,
  destinationNames,
  deviceOptions,
  emptyFormValues,
  formatSendTime,
  formatSentOn,
  groupOptions,
  initialAccountId,
  missingDestination,
  portOptionLabel,
  reportToFormValues,
  reportWithEnabled,
  telegramChatOptions,
  toGraphReportInput,
  toPreviewInput,
  type GraphReportFormValues,
} from "./graphReportForm";

const report: GraphReport = {
  id: "r1",
  waGroupId: "g1",
  groupName: "TEST MRTG",
  waGroupJid: "g1@g.us",
  waAccountId: "a1",
  waAccountLabel: "Kantor",
  telegramChatId: null,
  telegramChatTitle: "",
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
    waAccountId: "a1",
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
      telegramChatId: null,
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

// The shape a Telegram-only report comes back from the API in.
const telegramReport: GraphReport = {
  ...report,
  id: "r2",
  waGroupId: null,
  groupName: "",
  waGroupJid: "",
  waAccountId: null,
  waAccountLabel: "",
  mentionAll: false,
  telegramChatId: "t1",
  telegramChatTitle: "NOC Telegram",
};

const formValues = (
  destinations: Pick<GraphReportFormValues, "waGroupId" | "telegramChatId">,
): GraphReportFormValues => ({
  ...emptyFormValues(),
  customerName: "PT Contoh",
  ports: [{ deviceId: 7, deviceName: "core", ifName: "sfp1", label: "A" }],
  ...destinations,
});

describe("destinations", () => {
  it("sends a Telegram-only report with no WhatsApp group", () => {
    const values = formValues({ telegramChatId: "t1" });

    expect(missingDestination(values)).toBe(false);
    expect(toGraphReportInput(values)).toMatchObject({
      waGroupId: null,
      telegramChatId: "t1",
    });
  });

  it("sends both destinations when both are chosen", () => {
    const values = formValues({ waGroupId: "g1", telegramChatId: "t1" });

    expect(missingDestination(values)).toBe(false);
    expect(toGraphReportInput(values)).toMatchObject({
      waGroupId: "g1",
      telegramChatId: "t1",
    });
  });

  it("rejects a report with neither, including a cleared select's empty value", () => {
    expect(missingDestination(formValues({}))).toBe(true);
    expect(
      missingDestination(formValues({ waGroupId: "", telegramChatId: "" })),
    ).toBe(true);
  });

  it("round-trips a Telegram-only report, so an edit does not invent a WhatsApp group", () => {
    const values = reportToFormValues(telegramReport);

    expect(values.waGroupId).toBeUndefined();
    expect(values.waAccountId).toBeUndefined();
    expect(toGraphReportInput(values)).toMatchObject({
      waGroupId: null,
      telegramChatId: "t1",
    });
  });

  // Switching number clears the group without unticking the box, so the
  // form can still hold mentionAll=true for a report with no WhatsApp group.
  it("never tags everyone on a report without a WhatsApp group", () => {
    const values = {
      ...formValues({ telegramChatId: "t1" }),
      mentionAll: true,
    };

    expect(toGraphReportInput(values).mentionAll).toBe(false);
  });

  it("keeps the tag-everyone choice while a WhatsApp group is chosen", () => {
    const values = { ...formValues({ waGroupId: "g1" }), mentionAll: true };

    expect(toGraphReportInput(values).mentionAll).toBe(true);
  });

  it("names every destination in the send confirmation", () => {
    expect(destinationNames(report)).toBe("TEST MRTG");
    expect(destinationNames(telegramReport)).toBe("NOC Telegram");
    expect(
      destinationNames({
        ...report,
        telegramChatId: "t1",
        telegramChatTitle: "NOC Telegram",
      }),
    ).toBe("TEST MRTG dan NOC Telegram");
  });
});

describe("telegramChatOptions", () => {
  const chat = (id: string, extra: Partial<TelegramChat> = {}) =>
    ({
      id,
      chatId: -100,
      title: `Chat ${id}`,
      type: "supergroup",
      active: true,
      ...extra,
    }) as TelegramChat;

  it("lists only chats the bot is still in and no other report uses", () => {
    const chats = [chat("t1"), chat("t2", { active: false }), chat("t3")];

    expect(
      telegramChatOptions(chats, [telegramReport]).map((o) => o.value),
    ).toEqual(["t3"]);
  });

  it("keeps the edited report's own chat, disabled once the bot has left it", () => {
    expect(telegramChatOptions([chat("t1")], [telegramReport], "t1")).toEqual([
      { value: "t1", label: "Chat t1" },
    ]);
    expect(
      telegramChatOptions(
        [chat("t1", { active: false })],
        [telegramReport],
        "t1",
      ),
    ).toEqual([
      { value: "t1", label: "Chat t1 (bot sudah keluar)", disabled: true },
    ]);
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

describe("reportToFormValues", () => {
  it("orders the ports by position, not by the order the API listed them", () => {
    const values = reportToFormValues(report);

    expect(values.ports.map((p) => p.label)).toEqual(["Pertama", "Kedua"]);
    expect(values.ports[0]).not.toHaveProperty("id");
  });

  it("preselects the report's sending number but never sends it", () => {
    const values = reportToFormValues(report);

    expect(values.waAccountId).toBe("a1");
    expect(toGraphReportInput(values)).not.toHaveProperty("waAccountId");
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
  const own = { waAccountId: "a1" };

  // Its empty JID must not mark a group as reported, nor its null id
  // anything as taken.
  it("ignores Telegram-only reports when listing WhatsApp groups", () => {
    expect(
      groupOptions(
        [group("g1"), group("g2", { jid: "" })],
        [telegramReport],
        own,
      ),
    ).toEqual([
      { value: "g1", label: "Grup g1" },
      { value: "g2", label: "Grup g2" },
    ]);
  });

  it("leaves out groups the number left and groups that already have a report", () => {
    const groups = [group("g1"), group("g2", { left: true }), group("g3")];

    expect(groupOptions(groups, [report], own).map((o) => o.value)).toEqual([
      "g3",
    ]);
  });

  it("leaves out admins-only groups unless the number is an admin there", () => {
    const groups = [
      group("g2", { announce: true }),
      group("g3", { announce: true, isAdmin: true }),
    ];

    expect(groupOptions(groups, [], own).map((o) => o.value)).toEqual(["g3"]);
  });

  it("keeps the report's own group while it is being edited", () => {
    expect(
      groupOptions([group("g1")], [report], { ...own, currentGroupId: "g1" }),
    ).toEqual([{ value: "g1", label: "Grup g1" }]);
  });

  it("lists only the chosen number's groups, and none before one is chosen", () => {
    const groups = [group("g1"), group("g2", { waAccountId: "a2" })];

    expect(
      groupOptions(groups, [], { waAccountId: "a2" }).map((o) => o.value),
    ).toEqual(["g2"]);
    expect(groupOptions(groups, [])).toEqual([]);
  });

  // Two TikMan numbers in one WhatsApp group mirror it as two rows with the
  // same JID; a report on each would deliver it twice.
  it("disables the same group reached through another number", () => {
    const groups = [
      group("g1"),
      group("g9", { jid: "g1@g.us", name: "Grup g1", waAccountId: "a2" }),
    ];

    expect(groupOptions(groups, [report], { waAccountId: "a2" })).toEqual([
      {
        value: "g9",
        label: "Grup g1 (sudah ada laporan lewat Kantor)",
        disabled: true,
      },
    ]);
  });

  it("disables a reported group whose own row is not among the loaded groups", () => {
    const groups = [
      group("g9", { jid: "g1@g.us", name: "Grup g1", waAccountId: "a2" }),
    ];

    expect(
      groupOptions(groups, [report], { waAccountId: "a2" })[0].disabled,
    ).toBe(true);
  });

  it("lets the report being edited move to its group's other number", () => {
    const groups = [
      group("g1"),
      group("g9", { jid: "g1@g.us", name: "Grup g1", waAccountId: "a2" }),
    ];

    expect(
      groupOptions(groups, [report], {
        waAccountId: "a2",
        currentGroupId: "g1",
      }),
    ).toEqual([{ value: "g9", label: "Grup g1" }]);
  });
});

describe("accountOptions", () => {
  const account = (extra: Partial<WaAccount>) =>
    ({ id: "a1", label: "Kantor", status: "connected", ...extra }) as WaAccount;

  it("names each number with its label and phone, without the device suffix", () => {
    expect(
      accountOptions([account({ jid: "6281234567890:12@s.whatsapp.net" })]),
    ).toEqual([{ value: "a1", label: "Kantor — +6281234567890" }]);
  });

  it("marks a number that is not connected, or not paired yet", () => {
    expect(
      accountOptions([
        account({ jid: "62811@s.whatsapp.net", status: "disconnected" }),
        account({ id: "a2", label: "Baru", status: "pairing" }),
      ]).map((o) => o.label),
    ).toEqual(["Kantor — +62811 (tidak terhubung)", "Baru (tidak terhubung)"]);
  });
});

describe("initialAccountId", () => {
  it("uses the report's number, else its group's", () => {
    const groups = [group("g1", { waAccountId: "a2" })];

    expect(initialAccountId(report, groups)).toBe("a1");
    expect(initialAccountId({ ...report, waAccountId: "" }, groups)).toBe("a2");
    expect(
      initialAccountId({ ...report, waAccountId: "" }, []),
    ).toBeUndefined();
  });
});

describe("formatSentOn", () => {
  it("does not shift a WIB date by the browser's zone", () => {
    expect(formatSentOn("2026-11-11")).toMatch(/^11 Nov/);
    expect(formatSentOn(null)).toBe("Belum pernah");
  });
});
