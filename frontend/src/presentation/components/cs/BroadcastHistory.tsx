import { Empty, List, Tag, Typography } from "antd";
import type { BroadcastPost, BroadcastPostStatus } from "@/domain/entities";

const { Text } = Typography;

/** Three states and no more: neither a channel nor a status sends receipts,
 * so there is nothing honest to show between "sent" and whatever became of it
 * on the other end. */
const STATUS_LABEL: Record<BroadcastPostStatus, string> = {
  queued: "Antre",
  sent: "Terkirim",
  failed: "Gagal",
};

const STATUS_COLOR: Record<BroadcastPostStatus, string> = {
  queued: "default",
  sent: "success",
  failed: "error",
};

/** What a poster whose account has since been removed is shown as. A raw UUID
 * on screen tells the reader nothing and looks like a bug. */
const UNKNOWN_SENDER = "pengguna tak dikenal";
const UNKNOWN_CHANNEL = "saluran tak dikenal";
const UNKNOWN_NUMBER = "nomor tak dikenal";

/** A status disappears from WhatsApp after a day. Computed rather than
 * stored: a column would claim we know something we only calculate. */
const STATUS_LIFETIME_MS = 24 * 60 * 60 * 1000;

/** Whether a sent status is past the day WhatsApp shows it for. Exported for
 * its own test: nothing else decides whether a row still reads as live. */
export function statusHasExpired(
  post: BroadcastPost,
  now = Date.now(),
): boolean {
  if (post.destination !== "status" || !post.sentAt) return false;
  return now - new Date(post.sentAt).getTime() > STATUS_LIFETIME_MS;
}

function destinationLabel(
  post: BroadcastPost,
  channelNames: Record<string, string>,
  accountLabels: Record<string, string>,
): string {
  if (post.destination === "channel") {
    return `Saluran · ${channelNames[post.destinationJid ?? ""] ?? UNKNOWN_CHANNEL}`;
  }
  return `Status · ${accountLabels[post.waAccountId] ?? UNKNOWN_NUMBER}`;
}

interface BroadcastHistoryProps {
  posts: BroadcastPost[];
  loading: boolean;
  /** Each user's name by id. The history is what the loose permission model
   * was accepted on — Admin, CS and Technician can all broadcast irrevocably —
   * so "who" is not decoration here. */
  senderNames: Record<string, string>;
  /** The number's label per wa_account_id, shared with the composer's channel
   * picker so a status row and a channel row can both say which number it
   * came from. */
  accountLabels: Record<string, string>;
  /** Channel name per jid, so a channel row can say which channel it reached
   * without a second query. */
  channelNames: Record<string, string>;
}

/** What has been announced, where, and what became of it.
 *
 * Not decoration: sending only queues the announcement, so this is the only
 * place a sender ever learns whether it actually went out — and, with two
 * destinations now reachable from one action, which of them it reached. */
export function BroadcastHistory({
  posts,
  loading,
  senderNames,
  accountLabels,
  channelNames,
}: BroadcastHistoryProps) {
  return (
    <List
      size="small"
      loading={loading}
      dataSource={posts}
      locale={{
        emptyText: <Empty description="Belum ada pengumuman" />,
      }}
      renderItem={(post) => (
        <List.Item key={post.id}>
          <List.Item.Meta
            title={
              <>
                <Text type="secondary">
                  {destinationLabel(post, channelNames, accountLabels)}
                </Text>{" "}
                <Tag color={STATUS_COLOR[post.status]}>
                  {STATUS_LABEL[post.status]}
                </Tag>
                {statusHasExpired(post) && (
                  <Text type="secondary" style={{ fontSize: 12 }}>
                    sudah kedaluwarsa
                  </Text>
                )}{" "}
                <Text strong>
                  {senderNames[post.senderUserId] ?? UNKNOWN_SENDER}
                </Text>{" "}
                <Text type="secondary">
                  {new Date(post.createdAt).toLocaleString("id-ID")}
                </Text>
              </>
            }
            description={
              <>
                <div>{post.body || post.mediaFilename}</div>
                {post.failReason ? (
                  <Text type="danger">{post.failReason}</Text>
                ) : null}
              </>
            }
          />
        </List.Item>
      )}
    />
  );
}
