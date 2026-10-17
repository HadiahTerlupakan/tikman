import { Button, Empty, Spin, Tag, Typography } from "antd";
import { ArrowLeftOutlined, InfoCircleOutlined } from "@ant-design/icons";
import type {
  WaGroup,
  WaGroupMessage,
  WaGroupParticipant,
} from "@/domain/entities";
import {
  chatBackdropColor,
  chatBackdropImage,
  chatBackdropSize,
} from "@/shared/theme/chatBackdrop";
import { colors } from "@/shared/theme/colors";
import { useStickToBottom } from "../useStickToBottom";
import { GroupComposer } from "./GroupComposer";
import { GroupThread } from "./GroupThread";
import { sendBlockedReason } from "./groupSummary";

const { Text } = Typography;

interface GroupThreadPaneProps {
  group?: WaGroup;
  messages: WaGroupMessage[];
  loading: boolean;
  hasOlder: boolean;
  loadingOlder: boolean;
  onLoadOlder: () => void;
  replyTo?: WaGroupMessage;
  sending: boolean;
  attaching: boolean;
  /** Who can be mentioned from the composer. */
  participants: WaGroupParticipant[];
  onSend: (body: string, mentions?: string[]) => Promise<boolean>;
  onAttach: (
    file: File,
    caption: string,
    mentions?: string[],
  ) => Promise<boolean>;
  onReply: (message: WaGroupMessage) => void;
  onCancelReply: () => void;
  /** Present only where the panes take turns. */
  onBack?: () => void;
  onOpenInfo?: () => void;
}

function GroupHeader({
  group,
  onBack,
  onOpenInfo,
}: {
  group: WaGroup;
  onBack?: () => void;
  onOpenInfo?: () => void;
}) {
  return (
    <div
      style={{
        display: "flex",
        alignItems: "center",
        gap: 8,
        padding: "10px 14px",
        borderBottom: `1px solid ${colors.border}`,
      }}
    >
      {onBack && (
        <Button
          type="text"
          icon={<ArrowLeftOutlined />}
          onClick={onBack}
          aria-label="Kembali ke daftar grup"
        />
      )}
      <div style={{ flex: 1, minWidth: 0 }}>
        <Text strong ellipsis style={{ display: "block", fontSize: 15 }}>
          {group.name || group.jid}
        </Text>
        <Text type="secondary" style={{ fontSize: 12 }}>
          {group.participantCount} anggota
        </Text>
      </div>
      {group.left && <Tag bordered={false}>Keluar</Tag>}
      {group.announce && <Tag bordered={false}>Hanya admin mengirim</Tag>}
      {onOpenInfo && (
        <Button
          type="text"
          icon={<InfoCircleOutlined />}
          onClick={onOpenInfo}
          aria-label="Info grup"
        />
      )}
    </div>
  );
}

/** One group being read: who it is, what was said, and the box to write in. */
export function GroupThreadPane(props: GroupThreadPaneProps) {
  const { group, messages, loading, onSend, onAttach } = props;
  const log = useStickToBottom(group?.id, messages, loading);

  if (!group) {
    return (
      <div
        style={{
          flex: 1,
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
        }}
      >
        <Empty description="Pilih grup untuk mulai membaca" />
      </div>
    );
  }

  const canSend = !sendBlockedReason(group);

  return (
    <>
      <GroupHeader
        group={group}
        onBack={props.onBack}
        onOpenInfo={props.onOpenInfo}
      />
      <div
        ref={log.containerRef}
        onScroll={log.onScroll}
        role="log"
        aria-label="Riwayat grup"
        style={{
          flex: 1,
          overflowY: "auto",
          padding: "12px 14px",
          background: chatBackdropColor,
          backgroundImage: chatBackdropImage,
          backgroundSize: chatBackdropSize,
        }}
      >
        <div ref={log.contentRef}>
          {props.hasOlder && (
            <div style={{ textAlign: "center", marginBottom: 8 }}>
              <Button
                size="small"
                loading={props.loadingOlder}
                onClick={props.onLoadOlder}
              >
                Muat pesan lebih lama
              </Button>
            </div>
          )}
          {loading ? (
            <Spin />
          ) : (
            <GroupThread
              messages={messages}
              onRetry={onSend}
              onReply={canSend ? props.onReply : undefined}
            />
          )}
        </div>
      </div>
      <GroupComposer
        group={group}
        participants={props.participants}
        onSend={(body, mentions) => {
          log.stick();
          return onSend(body, mentions);
        }}
        onAttach={(file, caption, mentions) => {
          log.stick();
          return onAttach(file, caption, mentions);
        }}
        sending={props.sending}
        attaching={props.attaching}
        replyTo={props.replyTo}
        onCancelReply={props.onCancelReply}
      />
    </>
  );
}
