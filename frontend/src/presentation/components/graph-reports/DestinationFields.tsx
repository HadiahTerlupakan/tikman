import { useEffect } from "react";
import { Form, Select, Typography, type FormInstance } from "antd";
import {
  useTelegramChats,
  useWaAccounts,
  useWaGroups,
} from "@/application/hooks";
import type { GraphReport, TelegramChat } from "@/domain/entities";
import {
  NO_DESTINATION_MESSAGE,
  accountOptions,
  groupOptions,
  initialAccountId,
  missingDestination,
  type GraphReportFormValues,
} from "./graphReportForm";
import { telegramChatOptions, topicOptions } from "./telegramDestination";

interface DestinationFieldsProps {
  form: FormInstance<GraphReportFormValues>;
  open: boolean;
  report?: GraphReport;
  reports: GraphReport[];
}

// Telegram keeps a bot's updates for only a day, so a group the bot joined
// earlier is unknown to TikMan; with privacy mode on, a command addressed to
// the bot is the one ordinary message that still reaches it.
function TelegramHelp() {
  return (
    <>
      Tambahkan bot ke grup. Jika grup belum muncul, kirim{" "}
      <Typography.Text code>/start@nama_bot</Typography.Text> di grup itu. Token
      bot diatur di Pengaturan.
    </>
  );
}

interface TelegramChatFieldProps {
  chats: TelegramChat[];
  loading: boolean;
  report?: GraphReport;
  reports: GraphReport[];
}

function destinationRule({
  getFieldValue,
}: Pick<FormInstance<GraphReportFormValues>, "getFieldValue">) {
  return {
    validator: () =>
      missingDestination({
        waGroupId: getFieldValue("waGroupId"),
        telegramChatId: getFieldValue("telegramChatId"),
      })
        ? Promise.reject(new Error(NO_DESTINATION_MESSAGE))
        : Promise.resolve(),
  };
}

// The rule sits on this field and depends on the WhatsApp group, so the
// "pick one" message shows once, and clears as soon as either is chosen.
function TelegramChatField({
  chats,
  loading: chatsLoading,
  report,
  reports,
}: TelegramChatFieldProps) {
  return (
    <Form.Item
      name="telegramChatId"
      label="Grup Telegram"
      extra={<TelegramHelp />}
      dependencies={["waGroupId"]}
      rules={[destinationRule]}
    >
      <Select
        showSearch
        allowClear
        placeholder="Pilih grup"
        optionFilterProp="label"
        loading={chatsLoading}
        options={telegramChatOptions(
          chats,
          reports,
          report?.telegramChatId ?? undefined,
        )}
        notFoundContent={
          chatsLoading ? undefined : "Bot belum ada di grup mana pun"
        }
      />
    </Form.Item>
  );
}

interface TelegramTopicFieldProps {
  chat: TelegramChat;
  report?: GraphReport;
  reports: GraphReport[];
}

// Telegram has no call that lists a forum's topics; the bot learns one from
// a message posted in it, and a command is what reaches it in privacy mode.
function TelegramTopicField({
  chat,
  report,
  reports,
}: TelegramTopicFieldProps) {
  return (
    <Form.Item
      name="telegramThreadId"
      label="Topik"
      extra={
        <>
          Topik belum ada di daftar? Kirim{" "}
          <Typography.Text code>/start@nama_bot</Typography.Text> di dalam topik
          itu, lalu buka form ini lagi.
        </>
      }
      // An empty topic would be sent as General, which may be taken.
      rules={[{ required: true, message: "Pilih topik" }]}
    >
      <Select
        showSearch
        placeholder="Pilih topik"
        optionFilterProp="label"
        options={topicOptions(chat, reports, { report })}
      />
    </Form.Item>
  );
}

function TelegramFields({
  form,
  open,
  report,
  reports,
}: DestinationFieldsProps) {
  const { data: chats = [], isLoading } = useTelegramChats(open);
  const chatId = Form.useWatch("telegramChatId", form);
  const chat = chats.find((c) => c.id === chatId);
  return (
    <>
      <TelegramChatField
        chats={chats}
        loading={isLoading}
        report={report}
        reports={reports}
      />
      {chat?.isForum && (
        <TelegramTopicField chat={chat} report={report} reports={reports} />
      )}
    </>
  );
}

/** Where a report goes: a WhatsApp group, a Telegram chat, or both. */
export function DestinationFields({
  form,
  open,
  report,
  reports,
}: DestinationFieldsProps) {
  const { data: accounts = [], isLoading: accountsLoading } = useWaAccounts();
  const waAccountId = Form.useWatch("waAccountId", form);
  // The unfiltered list is capped across every number, so a quiet number's
  // groups could fall off it; asking for the chosen number's own is not.
  const { data: groups = [], isLoading: groupsLoading } = useWaGroups(
    waAccountId ? { waAccountId } : undefined,
  );
  // A report listed without its number gets it from its group, which may
  // load after the form has mounted.
  useEffect(() => {
    if (!report?.waGroupId || form.getFieldValue("waAccountId")) return;
    const id = initialAccountId(report, groups);
    if (id) form.setFieldValue("waAccountId", id);
  }, [report, groups, form]);

  return (
    <>
      <Form.Item
        name="waAccountId"
        label="Nomor WhatsApp"
        extra="Laporan WhatsApp dikirim dari nomor ini."
      >
        <Select
          placeholder="Pilih nomor"
          allowClear
          loading={accountsLoading}
          options={accountOptions(accounts)}
        />
      </Form.Item>
      <Form.Item name="waGroupId" label="Grup WhatsApp">
        <Select
          showSearch
          allowClear
          placeholder={waAccountId ? "Pilih grup" : "Pilih nomor dahulu"}
          optionFilterProp="label"
          disabled={!waAccountId}
          loading={groupsLoading}
          options={groupOptions(groups, reports, {
            waAccountId,
            currentGroupId: report?.waGroupId ?? undefined,
          })}
          notFoundContent={
            waAccountId && !groupsLoading
              ? "Nomor ini belum punya grup yang bisa dikirimi"
              : undefined
          }
        />
      </Form.Item>
      <TelegramFields
        form={form}
        open={open}
        report={report}
        reports={reports}
      />
    </>
  );
}
