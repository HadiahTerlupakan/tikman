import { Button, Empty, Spin, Tag, Typography } from "antd";
import { ArrowLeftOutlined, InfoCircleOutlined } from "@ant-design/icons";
import type {
  WaGroup,
  WaGroupMessage,
  WaGroupParticipant,
  WaGroupPollRequest,
} from "@/domain/entities";
import {
  chatBackdropColor,
  chatBackdropImage,
  chatBackdropSize,
} from "@/shared/theme/chatBackdrop";
import { colors } from "@/shared/theme/colors";
import { useStickToBottom } from "../useStickToBottom";
import { GroupAvatar } from "./GroupAvatars";
import { GroupComposer } from "./GroupComposer";
import { GroupThread } from "./GroupThread";
import type { GroupMessageHandlers } from "./groupMessageActions";
import { memberSummary, sendBlockedReason, typingLine } from "./groupSummary";

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
  onCancelReply: () => void;
  /** The viewer is a TikMan admin, which widens who may delete a message. */
  isAdmin: boolean;
  /** The viewer, who alone may edit what they wrote. */
  currentUserId: string;
  handlers: GroupMessageHandlers;
  /** Members writing in this group right now. */
  typingNames: string[];
  onTypingChange: (groupId: string, typing: boolean) => void;
  onCreatePoll: (poll: WaGroupPollRequest) => Promise<boolean>;
  creatingPoll: boolean;
  /** Present only where the panes take turns. */
  onBack?: () => void;
  onOpenInfo?: () => void;
}

function GroupHeader({
  group,
  participants,
  typingNames,
  onBack,
  onOpenInfo,
}: {
  group: WaGroup;
  participants: WaGroupParticipant[];
  typingNames: string[];
  onBack?: () => void;
  onOpenInfo?: () => void;
}) {
  const typing = typingLine(typingNames);
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
      <GroupAvatar groupId={group.id} hasAvatar={group.hasAvatar} size={36} />
      <div style={{ flex: 1, minWidth: 0 }}>
        <Text strong ellipsis style={{ display: "block", fontSize: 15 }}>
          {group.name || group.jid}
        </Text>
        <Text
          ellipsis
          type={typing ? undefined : "secondary"}
          style={{
            display: "block",
            fontSize: 12,
            color: typing ? colors.success : undefined,
          }}
        >
          {typing ?? memberSummary(participants, group.participantCount)}
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
        participants={props.participants}
        typingNames={props.typingNames}
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
              context={{
                group,
                isAdmin: props.isAdmin,
                userId: props.currentUserId,
                canSend,
              }}
              handlers={props.handlers}
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
        onTypingChange={props.onTypingChange}
        onCreatePoll={props.onCreatePoll}
        creatingPoll={props.creatingPoll}
      />
    </>
  );
}
