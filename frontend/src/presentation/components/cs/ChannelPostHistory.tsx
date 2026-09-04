import { Empty, List, Tag, Typography } from "antd";
import type { ChannelPost, ChannelPostStatus } from "@/domain/entities";

const { Text } = Typography;

/** Three states and no more: a channel sends no receipts, so there is nothing
 * honest to show between "sent" and whatever a subscriber did with it. */
const STATUS_LABEL: Record<ChannelPostStatus, string> = {
  queued: "Antre",
  sent: "Terkirim",
  failed: "Gagal",
};

const STATUS_COLOR: Record<ChannelPostStatus, string> = {
  queued: "default",
  sent: "success",
  failed: "error",
};

interface ChannelPostHistoryProps {
  posts: ChannelPost[];
  loading: boolean;
}

/** What has been announced on a channel, and what became of it.
 *
 * Not decoration: sending only queues the update, so this is the only place a
 * sender ever learns whether their announcement actually went out. */
export function ChannelPostHistory({
  posts,
  loading,
}: ChannelPostHistoryProps) {
  return (
    <List
      size="small"
      loading={loading}
      dataSource={posts}
      locale={{
        emptyText: <Empty description="Belum ada pembaruan di saluran ini" />,
      }}
      renderItem={(post) => (
        <List.Item key={post.id}>
          <List.Item.Meta
            title={
              <>
                <Tag color={STATUS_COLOR[post.status]}>
                  {STATUS_LABEL[post.status]}
                </Tag>
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
