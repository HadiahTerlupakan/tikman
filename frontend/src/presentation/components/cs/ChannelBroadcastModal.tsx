import { useState } from "react";
import {
  Alert,
  Button,
  Empty,
  Input,
  Modal,
  Select,
  Space,
  Upload,
} from "antd";
import { PaperClipOutlined, ReloadOutlined } from "@ant-design/icons";
import type { ChannelPost, WaChannel } from "@/domain/entities";
import { ChannelPostHistory } from "./ChannelPostHistory";

const { TextArea } = Input;

interface ChannelBroadcastModalProps {
  open: boolean;
  channels: WaChannel[];
  /** The number's label per wa_account_id, so one flat list can still say
   * which number administers each channel. */
  accountLabels: Record<string, string>;
  posts: ChannelPost[];
  loadingPosts: boolean;
  refreshing: boolean;
  sending: boolean;
  selectedChannelId?: string;
  onSelectChannel: (id: string) => void;
  onRefresh: () => void;
  /** Answers whether the update was queued, so the composer clears only when
   * it actually was and a rejected update is not silently thrown away. */
  onSend: (body: string, file?: File) => Promise<boolean>;
  onClose: () => void;
}

/** Posting an update to a WhatsApp Channel one of the team's numbers admins.
 *
 * One flat channel list rather than picking a number first: a channel belongs
 * to exactly one number, so asking for both would ask the sender to repeat
 * what they have already said. */
export function ChannelBroadcastModal({
  open,
  channels,
  accountLabels,
  posts,
  loadingPosts,
  refreshing,
  sending,
  selectedChannelId,
  onSelectChannel,
  onRefresh,
  onSend,
  onClose,
}: ChannelBroadcastModalProps) {
  const [body, setBody] = useState("");
  const [file, setFile] = useState<File>();

  const canSend = !!selectedChannelId && (body.trim() !== "" || !!file);

  const handleSend = async () => {
    if (!canSend) return;
    if (await onSend(body.trim(), file)) {
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
      title="Pembaruan Saluran"
    >
      {channels.length === 0 ? (
        <Empty description="Belum ada saluran. Nomor WhatsApp harus sudah menjadi admin sebuah saluran — TikMan tidak bisa membuatkannya." />
      ) : (
        <Space direction="vertical" style={{ width: "100%" }} size="middle">
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

          <TextArea
            rows={4}
            value={body}
            onChange={(event) => setBody(event.target.value)}
            placeholder="Tulis pembaruan untuk pengikut saluran"
          />

          <Upload
            maxCount={1}
            // false: the file is held until send rather than posted the moment
            // it is chosen. There is no withdrawing an update that has already
            // reached subscribers.
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
            message="Kirim akan mengantrekan pembaruan. Statusnya muncul di riwayat di bawah beberapa saat kemudian."
          />

          <Button
            type="primary"
            disabled={!canSend}
            loading={sending}
            onClick={handleSend}
          >
            Kirim Pembaruan
          </Button>

          <ChannelPostHistory posts={posts} loading={loadingPosts} />
        </Space>
      )}
    </Modal>
  );
}
