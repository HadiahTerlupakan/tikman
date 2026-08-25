import { Table, Button, Space, Popconfirm } from "antd";
import { EditOutlined, DeleteOutlined } from "@ant-design/icons";
import type { ConfigTemplate } from "@/domain/entities/ConfigTemplate";
import type { ColumnsType } from "antd/es/table";

interface ConfigTemplateTableProps {
  templates: ConfigTemplate[];
  loading: boolean;
  onEdit: (template: ConfigTemplate) => void;
  onDelete: (id: string) => void;
}

export function ConfigTemplateTable({
  templates,
  loading,
  onEdit,
  onDelete,
}: ConfigTemplateTableProps) {
  const columns: ColumnsType<ConfigTemplate> = [
    {
      title: "Name",
      dataIndex: "name",
      key: "name",
      fixed: "left",
      width: 200,
    },
    {
      title: "Vendor",
      dataIndex: "vendor",
      key: "vendor",
      width: 100,
    },
    {
      title: "Description",
      dataIndex: "description",
      key: "description",
      ellipsis: true,
      width: 250,
    },
    {
      title: "Default",
      dataIndex: "isDefault",
      key: "isDefault",
      width: 80,
      render: (isDefault: boolean) =>
        isDefault ? <span style={{ color: "#52c41a" }}>✓</span> : "-",
    },
    {
      title: "Created At",
      dataIndex: "createdAt",
      key: "createdAt",
      width: 160,
      responsive: ["lg"],
      render: (date: string) => new Date(date).toLocaleDateString("id-ID"),
    },
    {
      title: "Actions",
      key: "actions",
      fixed: "right",
      width: 180,
      render: (_, record) => (
        <Space>
          <Button
            type="link"
            icon={<EditOutlined />}
            onClick={() => onEdit(record)}
          >
            Edit
          </Button>
          <Popconfirm
            title="Hapus template ini?"
            description="Template yang digunakan oleh job provisioning tidak dapat dihapus"
            onConfirm={() => onDelete(record.id)}
            okText="Ya"
            cancelText="Tidak"
          >
            <Button type="link" danger icon={<DeleteOutlined />}>
              Delete
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <Table
      columns={columns}
      dataSource={templates}
      loading={loading}
      rowKey="id"
      pagination={{ pageSize: 10 }}
      scroll={{ x: 900 }}
    />
  );
}
