import { useEffect } from "react";
import { App, Button, Form, Modal } from "antd";
import type { Ont } from "@/domain/entities";
import { useAssignOntToOdp, useUnassignOntFromOdp } from "@/application/hooks";
import { OdpPortFields } from "@/presentation/components/OdpPortFields";

interface OntOdpModalProps {
  ont: Ont;
  onClose: () => void;
}

interface OdpPlacement {
  odpId?: string;
  odpPort?: number;
}

/** Where one subscriber's drop lands in the plant, set or taken back. */
export function OntOdpModal({ ont, onClose }: OntOdpModalProps) {
  const [form] = Form.useForm<OdpPlacement>();
  const { message } = App.useApp();
  const assign = useAssignOntToOdp();
  const unassign = useUnassignOntFromOdp();

  useEffect(() => {
    form.setFieldsValue({ odpId: ont.odpId, odpPort: ont.odpPort });
  }, [form, ont]);

  const save = async () => {
    let values: OdpPlacement;
    try {
      values = await form.validateFields();
    } catch {
      return;
    }
    try {
      await assign.mutateAsync({
        ontId: ont.id,
        odpId: values.odpId as string,
        port: values.odpPort as number,
      });
      message.success(`${ont.serialNumber} dipasang di port ${values.odpPort}`);
      onClose();
    } catch (error) {
      message.error((error as Error).message);
    }
  };

  const release = async () => {
    try {
      await unassign.mutateAsync(ont.id);
      message.success(`${ont.serialNumber} dilepas dari ODP`);
      onClose();
    } catch (error) {
      message.error((error as Error).message);
    }
  };

  return (
    <Modal
      open
      title={`ODP untuk ${ont.serialNumber}`}
      onCancel={onClose}
      onOk={save}
      okText="Simpan"
      cancelText="Batal"
      confirmLoading={assign.isPending}
      destroyOnClose
      footer={(_, { OkBtn, CancelBtn }) => (
        <>
          {ont.odpId && (
            <Button danger loading={unassign.isPending} onClick={release}>
              Lepas dari ODP
            </Button>
          )}
          <CancelBtn />
          <OkBtn />
        </>
      )}
    >
      <Form form={form} layout="vertical">
        <OdpPortFields currentOntId={ont.id} required />
      </Form>
    </Modal>
  );
}
