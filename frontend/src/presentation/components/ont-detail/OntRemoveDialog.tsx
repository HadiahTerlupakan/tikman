import { useState } from "react";
import { Alert, Checkbox, Modal, Spin, Typography } from "antd";
import { useOntRemovalPreview } from "@/application/hooks/useOnts";
import type { Ont } from "@/domain/entities";

const { Paragraph, Text } = Typography;

interface OntRemoveDialogProps {
  ont: Ont | null;
  open: boolean;
  loading?: boolean;
  onCancel: () => void;
  onConfirm: (removeFromOlt: boolean) => void;
}

// Removing from the OLT writes to a live device, so the exact commands are
// shown before the operator agrees to them, the same way provisioning does.
export function OntRemoveDialog({
  ont,
  open,
  loading = false,
  onCancel,
  onConfirm,
}: OntRemoveDialogProps) {
  const [removeFromOlt, setRemoveFromOlt] = useState(true);
  const {
    data: commands,
    isLoading,
    error,
  } = useOntRemovalPreview(open && ont ? ont.id : undefined);

  return (
    <Modal
      title={`Remove ${ont?.serialNumber ?? "ONT"}?`}
      open={open}
      okText="Remove"
      okButtonProps={{ danger: true }}
      confirmLoading={loading}
      onCancel={onCancel}
      onOk={() => onConfirm(removeFromOlt)}
      destroyOnClose
    >
      <Paragraph>
        TikMan&apos;s record of this ONT goes, along with its metrics and event
        history.
      </Paragraph>

      <Checkbox
        checked={removeFromOlt}
        onChange={(e) => setRemoveFromOlt(e.target.checked)}
      >
        Also delete the ONU from the OLT
      </Checkbox>

      {removeFromOlt ? (
        <div style={{ marginTop: 12 }}>
          {isLoading && <Spin size="small" />}
          {error && (
            <Alert
              type="error"
              showIcon
              message={
                error instanceof Error
                  ? error.message
                  : "Cannot work out what to send to the OLT"
              }
            />
          )}
          {commands && (
            <pre
              data-testid="removal-commands"
              style={{
                margin: 0,
                padding: 12,
                borderRadius: 6,
                background: "rgba(127,127,127,0.12)",
                fontSize: 12,
                whiteSpace: "pre-wrap",
              }}
            >
              {commands.join("\n")}
            </pre>
          )}
          <Alert
            type="warning"
            showIcon
            style={{ marginTop: 12 }}
            message="This cuts the subscriber's service."
            description="The OLT is changed first; TikMan's records are cleared only if it accepts. Nothing is deleted here if the OLT refuses."
          />
        </div>
      ) : (
        <Text type="secondary" style={{ display: "block", marginTop: 12 }}>
          The ONU stays configured on the OLT, so the next discovery poll will
          list it again once it comes back online.
        </Text>
      )}
    </Modal>
  );
}
