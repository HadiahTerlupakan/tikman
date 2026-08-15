import { Table, Button, Space, Badge, Popconfirm } from "antd";
import { EditOutlined, DeleteOutlined } from "@ant-design/icons";
import type { Site } from "@/domain/entities";
import type { ColumnsType } from "antd/es/table";

interface SiteTableProps {
  sites: Site[];
  loading: boolean;
  onEdit: (site: Site) => void;
  onDelete: (id: string) => void;
}

export function SiteTable({
  sites,
  loading,
  onEdit,
  onDelete,
}: SiteTableProps) {
  const columns: ColumnsType<Site> = [
    {
      title: "Site Name",
      dataIndex: "name",
      key: "name",
      fixed: "left",
      width: 180,
    },
    {
      title: "Location",
      dataIndex: "location",
      key: "location",
      width: 200,
      responsive: ["md"],
    },
    {
      title: "Description",
      dataIndex: "description",
      key: "description",
      ellipsis: true,
      width: 250,
      responsive: ["lg"],
    },
    {
      title: "OLT Count",
      dataIndex: "oltCount",
      key: "oltCount",
      width: 120,
      render: (count: number) => <Badge count={count} showZero color="blue" />,
    },
    {
      title: "Created At",
      dataIndex: "createdAt",
      key: "createdAt",
      width: 150,
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
            title="Hapus site ini?"
            description="OLT yang terhubung dengan site ini tidak akan terhapus"
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
      dataSource={sites}
      loading={loading}
      rowKey="id"
      pagination={{ pageSize: 10 }}
      scroll={{ x: 900 }}
    />
  );
}
