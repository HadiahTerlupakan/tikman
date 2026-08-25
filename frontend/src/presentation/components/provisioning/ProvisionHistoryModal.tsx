import { Modal, Table, Tag } from "antd";
import type { ProvisionJob } from "@/domain/entities/Provisioning";
import type { ColumnsType } from "antd/es/table";

interface ProvisionHistoryModalProps {
  open: boolean;
  ontId?: string;
  jobs?: ProvisionJob[];
  loading: boolean;
  onClose: () => void;
}

export function ProvisionHistoryModal({
  open,
  ontId,
  jobs,
  loading,
  onClose,
}: ProvisionHistoryModalProps) {
  const columns: ColumnsType<ProvisionJob> = [
    {
      title: "Status",
      dataIndex: "status",
      key: "status",
      render: (status: ProvisionJob["status"]) => {
        const colorMap: Record<string, string> = {
          pending: "gold",
          running: "blue",
          success: "green",
          failed: "red",
          rolled_back: "orange",
        };
        return <Tag color={colorMap[status] || "default"}>{status}</Tag>;
      },
    },
    {
      title: "Template",
      dataIndex: "templateId",
      key: "templateId",
      render: (templateId?: string) => templateId || "-",
    },
    {
      title: "Error",
      dataIndex: "errorMessage",
      key: "errorMessage",
      ellipsis: true,
    },
    {
      title: "Created At",
      dataIndex: "createdAt",
      key: "createdAt",
      render: (date: string) => new Date(date).toLocaleString("id-ID"),
    },
  ];

  return (
    <Modal
      title={`Provisioning History ${ontId ? `for ONT ${ontId}` : ""}`}
      open={open}
      onCancel={onClose}
      footer={null}
      width={800}
      destroyOnClose
    >
      <Table
        columns={columns}
        dataSource={jobs || []}
        loading={loading}
        rowKey="id"
        pagination={{ pageSize: 5 }}
        size="small"
      />
    </Modal>
  );
}
