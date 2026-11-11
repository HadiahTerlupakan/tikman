import { describe, expect, it } from "vitest";
import type { PortAlertDestination, TelegramChat } from "@/domain/entities";
import {
  destinationToFormValues,
  nocTopicOptions,
  toPortAlertDestinationInput,
} from "./portAlertDestination";
import { GENERAL_TOPIC } from "./telegramDestination";

// What the API answers before a destination was ever saved.
const unset: PortAlertDestination = {
  waAccountId: null,
  waGroupId: null,
  groupName: "",
  telegramChatId: null,
  telegramChatTitle: "",
  telegramThreadId: null,
  telegramTopicName: "",
  enabled: true,
};

const both: PortAlertDestination = {
  ...unset,
  waAccountId: "a1",
  waGroupId: "g1",
  groupName: "NOC",
  telegramChatId: "t1",
  telegramChatTitle: "NOC Forum",
  telegramThreadId: 42,
  telegramTopicName: "Alarm",
};

const forum: TelegramChat = {
  id: "t1",
  chatId: -100,
  title: "NOC Forum",
  type: "supergroup",
  active: true,
  isForum: true,
  topics: [{ threadId: 7, name: "Umum" }],
};

describe("destinationToFormValues", () => {
  it("opens an unset destination on WhatsApp with nothing picked", () => {
    const values = destinationToFormValues(unset);

    expect(values.channel).toBe("whatsapp");
    expect(values.waGroupId).toBeUndefined();
    expect(values.telegramChatId).toBeUndefined();
    expect(values.enabled).toBe(true);
  });

  it("round-trips a saved destination with both channels and a topic", () => {
    const values = destinationToFormValues(both);

    expect(values.channel).toBe("both");
    expect(values.waAccountId).toBe("a1");
    expect(toPortAlertDestinationInput(values)).toEqual({
      waGroupId: "g1",
      telegramChatId: "t1",
      telegramThreadId: 42,
      enabled: true,
    });
  });

  it("opens a Telegram-only destination on Telegram", () => {
    const values = destinationToFormValues({
      ...both,
      waAccountId: null,
      waGroupId: null,
    });

    expect(values.channel).toBe("telegram");
  });
});

describe("toPortAlertDestinationInput", () => {
  it("sends a switched-off channel's picks as null", () => {
    const values = {
      ...destinationToFormValues(both),
      channel: "whatsapp" as const,
    };

    expect(toPortAlertDestinationInput(values)).toEqual({
      waGroupId: "g1",
      telegramChatId: null,
      telegramThreadId: null,
      enabled: true,
    });
  });

  it("sends General as a null thread id", () => {
    const values = {
      ...destinationToFormValues(both),
      channel: "telegram" as const,
      telegramThreadId: GENERAL_TOPIC,
    };

    expect(toPortAlertDestinationInput(values).telegramThreadId).toBeNull();
  });

  it("keeps Aktif off", () => {
    const values = { ...destinationToFormValues(both), enabled: false };

    expect(toPortAlertDestinationInput(values).enabled).toBe(false);
  });
});

describe("nocTopicOptions", () => {
  it("offers General and every known topic", () => {
    expect(nocTopicOptions(forum).map((o) => o.value)).toEqual([
      GENERAL_TOPIC,
      7,
    ]);
  });

  it("keeps the saved topic by name when the bot no longer lists it", () => {
    const options = nocTopicOptions(forum, both);

    expect(options).toContainEqual({ value: 42, label: "Alarm" });
  });

  it("does not add a topic saved for another chat", () => {
    const options = nocTopicOptions({ ...forum, id: "t2" }, both);

    expect(options.map((o) => o.value)).toEqual([GENERAL_TOPIC, 7]);
  });
});
