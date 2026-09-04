import { useState } from "react";
import {
  Alert,
  Button,
  Checkbox,
  Input,
  Modal,
  Select,
  Space,
  Typography,
  Upload,
} from "antd";
import { PaperClipOutlined, ReloadOutlined } from "@ant-design/icons";
import type {
  BroadcastPost,
  BroadcastTarget,
  WaChannel,
} from "@/domain/entities";
import { BroadcastHistory } from "./BroadcastHistory";

const { TextArea } = Input;
const { Text } = Typography;

/** A status accepts text, image and video — never a document. Anything that
 * is not one of the two media kinds WhatsApp shows on a status is treated as
 * a document, same as the API enforces it. */
function isDocumentFile(file?: File): boolean {
  if (!file) return false;
  return !file.type.startsWith("image/") && !file.type.startsWith("video/");
}

interface BroadcastModalProps {
  open: boolean;
  channels: WaChannel[];
  /** Ids of the numbers currently connected — the only ones a status can go
   * out from. */
  statusAccountIds: string[];
  /** The number's label per wa_account_id, so one flat list can still say
   * which number administers each channel or sends each status. */
  accountLabels: Record<string, string>;
  /** Channel name per jid, so the history can label a channel row without a
   * second query. */
  channelNames: Record<string, string>;
  /** Each user's name by id, so the history can say who posted rather than
   * showing a raw sender id or nothing at all. */
  senderNames: Record<string, string>;
  posts: BroadcastPost[];
  loadingPosts: boolean;
  refreshing: boolean;
  sending: boolean;
  selectedChannelId?: string;
  onSelectChannel: (id: string) => void;
  onRefresh: () => void;
  /** Answers whether the announcement was queued, so the composer clears only
   * when it actually was and a rejected one is not silently thrown away. */
  onSend: (
    body: string,
    targets: BroadcastTarget[],
    file?: File,
  ) => Promise<boolean>;
  onClose: () => void;
}

/** Posting one announcement to a WhatsApp Channel, a WhatsApp Status, or
 * both at once, from whichever of the team's numbers admins or holds each.
 *
 * The two destinations are independent checkboxes rather than a single
 * picker: a channel belongs to one number and a status to another, and a
 * team announcing something wants both reached in one action, not two trips
 * through this modal. */
export function BroadcastModal({
  open,
  channels,
  statusAccountIds,
  accountLabels,
  channelNames,
  senderNames,
  posts,
  loadingPosts,
  refreshing,
  sending,
  selectedChannelId,
  onSelectChannel,
  onRefresh,
  onSend,
  onClose,
}: BroadcastModalProps) {
  const [body, setBody] = useState("");
  const [file, setFile] = useState<File>();
  // Seeded from selectedChannelId rather than always false: the hook keeps
  // that id across renders even while this modal is closed, so reopening on
  // an already-chosen channel should not force the sender to re-tick it.
  const [channelChecked, setChannelChecked] = useState(!!selectedChannelId);
  const [statusChecked, setStatusChecked] = useState(false);
  const [statusTargets, setStatusTargets] = useState<string[]>([]);

  const documentAttached = isDocumentFile(file);
  // Derived rather than cleared on attach: a document removed later restores
  const statusActive = statusChecked && !documentAttached;

  const targets: BroadcastTarget[] = [];
  if (channelChecked && selectedChannelId) {
    targets.push({ type: "channel", channelId: selectedChannelId });
  }
  if (statusActive) {
    for (const waAccountId of statusTargets) {
      targets.push({ type: "status", waAccountId });
    }
  }

  const canSend = targets.length > 0 && (body.trim() !== "" || !!file);

  const handleSend = async () => {
    if (!canSend) return;
    if (await onSend(body.trim(), targets, file)) {
      setBody("");
      setFile(undefined);
    }
  };

  return (
    <Modal
      open={open}
      onCancel={onClose}
      footer={null}
      width={640}
      title="Pengumuman"
    >
      <Space direction="vertical" style={{ width: "100%" }} size="middle">
        <Space direction="vertical" style={{ width: "100%" }} size="small">
          <Checkbox
            checked={channelChecked}
            disabled={channels.length === 0}
            onChange={(e) => setChannelChecked(e.target.checked)}
          >
            Saluran
          </Checkbox>
          {channels.length === 0 ? (
            <Text type="secondary" style={{ fontSize: 12 }}>
              Belum ada saluran. Nomor WhatsApp harus sudah menjadi admin sebuah
              saluran — TikMan tidak bisa membuatkannya.
            </Text>
          ) : (
            channelChecked && (
              <Space.Compact style={{ width: "100%" }}>
                <Select
                  style={{ flex: 1 }}
                  placeholder="Pilih saluran"
                  value={selectedChannelId}
                  onChange={onSelectChannel}
                  options={channels.map((channel) => ({
                    value: channel.id,
                    label: `${channel.name} — ${accountLabels[channel.waAccountId] ?? "nomor tak dikenal"}`,
                  }))}
                />
                <Button
                  icon={<ReloadOutlined />}
                  loading={refreshing}
                  onClick={onRefresh}
                >
                  Segarkan
                </Button>
              </Space.Compact>
            )
          )}
        </Space>

        <Space direction="vertical" style={{ width: "100%" }} size="small">
          <Checkbox
            checked={statusActive}
            disabled={documentAttached}
            onChange={(e) => setStatusChecked(e.target.checked)}
          >
            Status WA
          </Checkbox>
          {documentAttached && (
            <Text type="secondary" style={{ fontSize: 12 }}>
              Status hanya menerima teks, gambar, dan video.
            </Text>
          )}
          {statusActive && (
            <Checkbox.Group
              value={statusTargets}
              onChange={(values) => setStatusTargets(values as string[])}
              options={statusAccountIds.map((id) => ({
                value: id,
                label: accountLabels[id] ?? "nomor tak dikenal",
              }))}
            />
          )}
        </Space>

        <TextArea
          rows={4}
          value={body}
          onChange={(event) => setBody(event.target.value)}
          placeholder="Tulis pengumuman"
        />

        <Upload
          maxCount={1}
          // false: the file is held until send rather than posted the moment
          // it is chosen. There is no withdrawing an announcement that has
          // already reached a channel's subscribers or a status's viewers.
          beforeUpload={(chosen) => {
            setFile(chosen);
            return false;
          }}
          onRemove={() => setFile(undefined)}
        >
          <Button icon={<PaperClipOutlined />}>Lampiran</Button>
        </Upload>

        <Alert
          type="info"
          showIcon
          message="Kirim akan mengantrekan pengumuman untuk setiap tujuan yang dipilih. Statusnya muncul di riwayat di bawah beberapa saat kemudian."
        />

        <Button
          type="primary"
          disabled={!canSend}
          loading={sending}
          onClick={handleSend}
        >
          Kirim Pembaruan
        </Button>

        <BroadcastHistory
          posts={posts}
          loading={loadingPosts}
          senderNames={senderNames}
          accountLabels={accountLabels}
          channelNames={channelNames}
        />
      </Space>
    </Modal>
  );
}
