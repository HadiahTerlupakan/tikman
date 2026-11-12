import { describe, expect, it } from "vitest";
import type { GraphReport, TelegramChat } from "@/domain/entities";
import {
  GENERAL_TOPIC,
  initialTopic,
  telegramChatOptions,
  telegramDestinationLabel,
  topicOptions,
} from "./telegramDestination";

const chat = (id: string, extra: Partial<TelegramChat> = {}): TelegramChat => ({
  id,
  chatId: -100,
  title: `Chat ${id}`,
  type: "supergroup",
  active: true,
  isForum: false,
  topics: [],
  ...extra,
});

const forum = (id: string, extra: Partial<TelegramChat> = {}) =>
  chat(id, {
    isForum: true,
    topics: [
      { threadId: 7, name: "PT Contoh" },
      { threadId: 9, name: "" },
    ],
    ...extra,
  });

const telegramReport = (
  id: string,
  chatId: string,
  threadId: number | null = null,
  topicName = "",
): GraphReport => ({
  id,
  waGroupId: null,
  groupName: "",
  waGroupJid: "",
  waAccountId: null,
  waAccountLabel: "",
  telegramChatId: chatId,
  telegramChatTitle: `Chat ${chatId}`,
  telegramThreadId: threadId,
  telegramTopicName: topicName,
  customerName: "PT Contoh",
  message: "",
  enabled: true,
  mentionAll: false,
  sendTime: "06:00",
  lastSentOn: null,
  lastAttemptAt: null,
  lastError: "",
  ports: [],
});

const values = (options: { value: unknown }[]) => options.map((o) => o.value);

describe("telegramChatOptions", () => {
  it("lists only chats the bot is still in and no other report uses", () => {
    const chats = [chat("t1"), chat("t2", { active: false }), chat("t3")];

    expect(
      values(telegramChatOptions(chats, [telegramReport("r1", "t1")])),
    ).toEqual(["t3"]);
  });

  it("keeps the edited report's own chat, disabled once the bot has left it", () => {
    const reports = [telegramReport("r1", "t1")];

    expect(telegramChatOptions([chat("t1")], reports, "t1")).toEqual([
      { value: "t1", label: "Chat t1" },
    ]);
    expect(
      telegramChatOptions([chat("t1", { active: false })], reports, "t1"),
    ).toEqual([
      { value: "t1", label: "Chat t1 (bot sudah keluar)", disabled: true },
    ]);
  });

  // Each customer has a topic in one forum, so the forum takes a report per
  // topic, not one in all.
  it("keeps a forum listed while one of its topics or General is free", () => {
    const reports = [telegramReport("r1", "f1"), telegramReport("r2", "f1", 7)];

    expect(values(telegramChatOptions([forum("f1")], reports))).toEqual(["f1"]);
  });

  it("drops a forum once General and every known topic have a report", () => {
    const reports = [
      telegramReport("r1", "f1"),
      telegramReport("r2", "f1", 7),
      telegramReport("r3", "f1", 9),
    ];

    expect(telegramChatOptions([forum("f1")], reports)).toEqual([]);
  });
});

describe("topicOptions", () => {
  it("offers General first, then the topics, numbering one without a name", () => {
    expect(topicOptions(forum("f1"), [])).toEqual([
      { value: GENERAL_TOPIC, label: "General" },
      { value: 7, label: "PT Contoh" },
      { value: 9, label: "Topik #9" },
    ]);
  });

  it("hides topics, and General, that another report on the forum posts in", () => {
    const reports = [
      telegramReport("r1", "f1"),
      telegramReport("r2", "f1", 7),
      telegramReport("r3", "f2", 9),
    ];

    expect(values(topicOptions(forum("f1"), reports))).toEqual([9]);
  });

  it("keeps the edited report's own topic", () => {
    const own = telegramReport("r2", "f1", 7);

    expect(values(topicOptions(forum("f1"), [own], { report: own }))).toEqual([
      GENERAL_TOPIC,
      7,
      9,
    ]);
  });

  it("keeps the edited report's topic by name when the forum's list lacks it", () => {
    const own = telegramReport("r2", "f1", 12, "PT Lama");

    expect(topicOptions(forum("f1"), [own], { report: own })).toContainEqual({
      value: 12,
      label: "PT Lama",
    });
  });
});

describe("initialTopic", () => {
  it("starts a forum on General while General is free", () => {
    expect(initialTopic(forum("f1"), [telegramReport("r1", "f1", 7)])).toBe(
      GENERAL_TOPIC,
    );
  });

  it("starts on the first free topic, not General, once General has a report", () => {
    const reports = [telegramReport("r1", "f1")];

    expect(initialTopic(forum("f1"), reports)).toBe(7);
  });

  it("starts on nothing when every topic of the forum is taken", () => {
    const reports = [
      telegramReport("r1", "f1"),
      telegramReport("r2", "f1", 7),
      telegramReport("r3", "f1", 9),
    ];

    expect(initialTopic(forum("f1"), reports)).toBeUndefined();
  });

  it("starts the edited report on its own General when it holds it", () => {
    const own = telegramReport("r1", "f1");

    expect(initialTopic(forum("f1"), [own], { report: own })).toBe(
      GENERAL_TOPIC,
    );
  });

  it("leaves a plain group, or no chat, on General", () => {
    const reports = [telegramReport("r1", "c1")];

    expect(initialTopic(chat("c1"), reports)).toBe(GENERAL_TOPIC);
    expect(initialTopic(undefined, reports)).toBe(GENERAL_TOPIC);
  });
});

describe("telegramDestinationLabel", () => {
  it("names the group alone for General", () => {
    expect(telegramDestinationLabel(telegramReport("r1", "f1"))).toBe(
      "Chat f1",
    );
  });

  it("names the group and its topic, or the topic's number", () => {
    expect(
      telegramDestinationLabel(telegramReport("r1", "f1", 7, "PT Contoh")),
    ).toBe("Chat f1 › PT Contoh");
    expect(telegramDestinationLabel(telegramReport("r1", "f1", 9))).toBe(
      "Chat f1 › Topik #9",
    );
  });
});
