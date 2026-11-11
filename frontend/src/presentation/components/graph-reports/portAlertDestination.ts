import type {
  PortAlertDestination,
  PortAlertDestinationInput,
  TelegramChat,
} from "@/domain/entities";
import {
  channelOf,
  sendsTelegram,
  sendsWhatsApp,
  type DeliveryChannel,
  type SelectOption,
} from "./graphReportForm";
import { GENERAL_TOPIC, topicLabel, topicOptions } from "./telegramDestination";

export interface PortAlertFormValues {
  channel: DeliveryChannel;
  /** Only narrows the group picker; the group row already names its number. */
  waAccountId?: string;
  waGroupId?: string;
  telegramChatId?: string;
  /** GENERAL_TOPIC for General; only read while the chat is a forum. */
  telegramThreadId?: number;
  enabled: boolean;
}

export function destinationToFormValues(
  destination: PortAlertDestination,
): PortAlertFormValues {
  return {
    channel: channelOf(destination.waGroupId, destination.telegramChatId),
    waAccountId: destination.waAccountId ?? undefined,
    waGroupId: destination.waGroupId ?? undefined,
    telegramChatId: destination.telegramChatId ?? undefined,
    telegramThreadId: destination.telegramThreadId ?? GENERAL_TOPIC,
    enabled: destination.enabled,
  };
}

/** The picks of a channel switched off stay in the form but are sent as
 * null, which the API reads as "no destination there". */
export function toPortAlertDestinationInput(
  values: PortAlertFormValues,
): PortAlertDestinationInput {
  const waGroupId = sendsWhatsApp(values.channel)
    ? values.waGroupId ?? null
    : null;
  const telegramChatId = sendsTelegram(values.channel)
    ? values.telegramChatId ?? null
    : null;
  return {
    waGroupId,
    telegramChatId,
    telegramThreadId:
      telegramChatId && values.telegramThreadId
        ? values.telegramThreadId
        : null,
    enabled: values.enabled,
  };
}

/** Every topic of the forum: the NOC may share a topic with a report. The
 * saved topic is kept even when the bot no longer lists it, so it shows by
 * name rather than as a bare number. */
export function nocTopicOptions(
  chat: TelegramChat,
  saved?: PortAlertDestination,
): SelectOption<number>[] {
  const options = topicOptions(chat, []);
  const own = saved?.telegramThreadId;
  if (saved?.telegramChatId !== chat.id || !own) return options;
  if (!options.some((o) => o.value === own)) {
    options.push({
      value: own,
      label: topicLabel(own, saved.telegramTopicName),
    });
  }
  return options;
}
