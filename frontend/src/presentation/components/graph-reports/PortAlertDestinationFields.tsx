import { Form, Radio, Select, type FormInstance } from "antd";
import {
  useTelegramChats,
  useWaAccounts,
  useWaGroups,
} from "@/application/hooks";
import type { PortAlertDestination } from "@/domain/entities";
import {
  CHANNEL_OPTIONS,
  accountOptions,
  groupOptions,
  sendsTelegram,
  sendsWhatsApp,
} from "./graphReportForm";
import {
  nocTopicOptions,
  type PortAlertFormValues,
} from "./portAlertDestination";
import { telegramChatOptions } from "./telegramDestination";

interface FieldsProps {
  form: FormInstance<PortAlertFormValues>;
  saved: PortAlertDestination;
}

// The report helpers are given no reports: the NOC may share a group, chat
// or topic with any report, so nothing is hidden as taken.
function WhatsAppFields({ form }: Pick<FieldsProps, "form">) {
  const { data: accounts = [], isLoading: accountsLoading } = useWaAccounts();
  const waAccountId = Form.useWatch("waAccountId", form);
  const { data: groups = [], isLoading: groupsLoading } = useWaGroups(
    waAccountId ? { waAccountId } : undefined,
  );
  return (
    <>
      <Form.Item name="waAccountId" label="Nomor WhatsApp">
        <Select
          placeholder="Pilih nomor"
          allowClear
          loading={accountsLoading}
          options={accountOptions(accounts)}
        />
      </Form.Item>
      <Form.Item
        name="waGroupId"
        label="Grup WhatsApp"
        rules={[{ required: true, message: "Pilih grup WhatsApp" }]}
      >
        <Select
          showSearch
          allowClear
          placeholder={waAccountId ? "Pilih grup" : "Pilih nomor dahulu"}
          optionFilterProp="label"
          disabled={!waAccountId}
          loading={groupsLoading}
          options={groupOptions(groups, [], { waAccountId })}
        />
      </Form.Item>
    </>
  );
}

function TelegramFields({ form, saved }: FieldsProps) {
  const { data: chats = [], isLoading } = useTelegramChats(true);
  const chatId = Form.useWatch("telegramChatId", form);
  const chat = chats.find((c) => c.id === chatId);
  return (
    <>
      <Form.Item
        name="telegramChatId"
        label="Grup Telegram"
        rules={[{ required: true, message: "Pilih grup Telegram" }]}
      >
        <Select
          showSearch
          allowClear
          placeholder="Pilih grup"
          optionFilterProp="label"
          loading={isLoading}
          options={telegramChatOptions(
            chats,
            [],
            saved.telegramChatId ?? undefined,
          )}
        />
      </Form.Item>
      {chat?.isForum && (
        <Form.Item
          name="telegramThreadId"
          label="Topik"
          rules={[{ required: true, message: "Pilih topik" }]}
        >
          <Select
            showSearch
            placeholder="Pilih topik"
            optionFilterProp="label"
            options={nocTopicOptions(chat, saved)}
          />
        </Form.Item>
      )}
    </>
  );
}

/** Where the NOC's notices go: a WhatsApp group, a Telegram chat, or both. */
export function PortAlertDestinationFields({ form, saved }: FieldsProps) {
  const channel = Form.useWatch("channel", form);
  return (
    <>
      <Form.Item name="channel" label="Kirim ke">
        <Radio.Group
          optionType="button"
          buttonStyle="solid"
          options={CHANNEL_OPTIONS}
        />
      </Form.Item>
      {sendsWhatsApp(channel) && <WhatsAppFields form={form} />}
      {sendsTelegram(channel) && <TelegramFields form={form} saved={saved} />}
    </>
  );
}
