import { Alert, Button, Popconfirm, Space } from "antd";
import type { CsConversation } from "@/domain/entities";
import { colors } from "@/shared/theme/colors";

interface TakeOverNoticeProps {
  conversation: CsConversation;
  holderName: string;
  onTakeOver: () => void;
}

/**
 * Stands in for the reply box on a thread this CS may not answer: who is
 * serving the customer, and the way in. A greyed-out send button with no reason
 * reads as a broken page, and taking a customer from a colleague is never one
 * stray click — it asks first, and the takeover is audited.
 */
export function TakeOverNotice({
  conversation,
  holderName,
  onTakeOver,
}: TakeOverNoticeProps) {
  const finished = conversation.status === "closed";
  const served = finished
    ? `diselesaikan ${holderName}`
    : `sedang dilayani ${holderName}`;

  return (
    <Space
      direction="vertical"
      style={{
        width: "100%",
        padding: "10px 12px",
        borderTop: `1px solid ${colors.border}`,
        background: colors.surface,
      }}
    >
      <Alert
        type="info"
        showIcon
        message={
          conversation.assignedUserId
            ? `${finished ? "Diselesaikan" : "Sedang dilayani"} ${holderName}`
            : "Belum ada yang melayani percakapan ini"
        }
      />
      {conversation.assignedUserId ? (
        <Popconfirm
          title={`Percakapan ini ${served}. Ambil alih?`}
          okText="Ya, ambil alih"
          cancelText="Batal"
          onConfirm={onTakeOver}
        >
          <Button>Ambil alih</Button>
        </Popconfirm>
      ) : (
        <Button onClick={onTakeOver}>Ambil alih</Button>
      )}
    </Space>
  );
}
