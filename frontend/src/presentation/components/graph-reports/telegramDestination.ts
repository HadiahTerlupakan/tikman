import type { GraphReport, TelegramChat } from "@/domain/entities";
import type { SelectOption } from "./graphReportForm";

/** The Topik select's value for General. A Select treats null as "nothing
 * chosen" and would show its placeholder, so General gets a number no real
 * topic has (Telegram's thread ids are positive). */
export const GENERAL_TOPIC = 0;

const GENERAL_LABEL = "General";

export function topicLabel(threadId: number, name: string): string {
  return name || `Topik #${threadId}`;
}

/** The report's Telegram destination by name: the group, and its topic when
 * it posts in one. */
export function telegramDestinationLabel(report: GraphReport): string {
  if (report.telegramThreadId === null) return report.telegramChatTitle;
  const topic = topicLabel(report.telegramThreadId, report.telegramTopicName);
  return `${report.telegramChatTitle} › ${topic}`;
}

/** The topics of chatId other reports already post in, General as
 * GENERAL_TOPIC: the API refuses a second report on the same topic. */
function takenTopics(
  chatId: string,
  reports: GraphReport[],
  currentReportId?: string,
): Set<number> {
  return new Set(
    reports
      .filter((r) => r.telegramChatId === chatId && r.id !== currentReportId)
      .map((r) => r.telegramThreadId ?? GENERAL_TOPIC),
  );
}

export interface TopicOptionScope {
  /** The edited report, whose own topic stays selectable. */
  report?: GraphReport;
}

/** General first, then the forum's known topics, leaving out those another
 * report already posts in. The edited report's own topic is kept even when
 * the chat's list no longer has it, so it shows by name rather than as a
 * bare number. */
export function topicOptions(
  chat: TelegramChat,
  reports: GraphReport[],
  { report }: TopicOptionScope = {},
): SelectOption<number>[] {
  const taken = takenTopics(chat.id, reports, report?.id);
  const options = [
    { value: GENERAL_TOPIC, label: GENERAL_LABEL },
    ...chat.topics.map((t) => ({
      value: t.threadId,
      label: topicLabel(t.threadId, t.name),
    })),
  ].filter((o) => !taken.has(o.value));
  if (report?.telegramChatId !== chat.id || !report.telegramThreadId) {
    return options;
  }
  const own = report.telegramThreadId;
  if (!options.some((o) => o.value === own)) {
    options.push({
      value: own,
      label: topicLabel(own, report.telegramTopicName),
    });
  }
  return options;
}

/** The topic a newly chosen chat starts on: the first one topicOptions
 * offers, or none when every topic is taken. Always General would leave the
 * Select on a value it hides once General has a report, and saving that
 * posts to General, which the API refuses. A plain group has no topics, and
 * General is what it sends. */
export function initialTopic(
  chat: TelegramChat | undefined,
  reports: GraphReport[],
  scope: TopicOptionScope = {},
): number | undefined {
  if (!chat?.isForum) return GENERAL_TOPIC;
  return topicOptions(chat, reports, scope)[0]?.value;
}

/** Whether another report could still go to the chat: a plain group takes
 * one, a forum one per topic plus General. */
function hasFreeDestination(chat: TelegramChat, reports: GraphReport[]) {
  if (chat.isForum) return topicOptions(chat, reports).length > 0;
  return !reports.some((r) => r.telegramChatId === chat.id);
}

/** Only chats the bot is still in and that can take another report: the API
 * refuses both. The edited report's own chat stays listed even after the bot
 * left it, disabled, so the admin sees why it no longer sends rather than a
 * bare id. */
export function telegramChatOptions(
  chats: TelegramChat[],
  reports: GraphReport[],
  currentChatId?: string,
): SelectOption<string>[] {
  return chats
    .filter(
      (c) =>
        (c.active && hasFreeDestination(c, reports)) || c.id === currentChatId,
    )
    .map((c) => {
      const name = c.title || String(c.chatId);
      if (c.active) return { value: c.id, label: name };
      return {
        value: c.id,
        label: `${name} (bot sudah keluar)`,
        disabled: true,
      };
    });
}
