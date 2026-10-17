import { Spin, Typography } from "antd";
import { StarFilled } from "@ant-design/icons";
import { useWaGroupStarred } from "@/application/hooks";
import type { WaGroupMessage } from "@/domain/entities";
import { colors } from "@/shared/theme/colors";
import { shortTime } from "../conversationSummary";
import { senderLabel } from "./groupSummary";
import { messageAnchor } from "./groupThreadLayout";

const { Text } = Typography;

/** The message in the thread, when it is among the pages already loaded. */
function jumpTo(message: WaGroupMessage) {
  document
    .getElementById(messageAnchor(message.id))
    ?.scrollIntoView({ behavior: "smooth", block: "center" });
}

function StarredRow({ message }: { message: WaGroupMessage }) {
  return (
    <div
      role="button"
      tabIndex={0}
      onClick={() => jumpTo(message)}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          jumpTo(message);
        }
      }}
      style={{
        padding: "6px 0",
        cursor: "pointer",
        borderBottom: `1px solid ${colors.border}`,
      }}
    >
      <div style={{ display: "flex", justifyContent: "space-between", gap: 8 }}>
        <Text strong style={{ fontSize: 12 }}>
          {senderLabel(message)}
        </Text>
        <Text type="secondary" style={{ fontSize: 11 }}>
          {shortTime(message.waTimestamp)}
        </Text>
      </div>
      <Text ellipsis style={{ display: "block", fontSize: 13 }}>
        {message.deleted
          ? "Pesan ini telah dihapus"
          : message.body || message.mediaFilename || "Lampiran"}
      </Text>
    </div>
  );
}

/** What the team starred in this group, newest first. */
export function StarredMessages({ groupId }: { groupId: string }) {
  const starred = useWaGroupStarred(groupId);
  if (starred.isLoading) return <Spin size="small" />;
  const messages = starred.data ?? [];
  if (messages.length === 0) {
    return (
      <Text type="secondary" style={{ fontSize: 12 }}>
        <StarFilled style={{ marginRight: 6 }} />
        Belum ada pesan berbintang.
      </Text>
    );
  }
  return (
    <div role="list" aria-label="Pesan berbintang">
      {messages.map((message) => (
        <StarredRow key={message.id} message={message} />
      ))}
    </div>
  );
}
