import { Table, Button, Space, Tag, Popconfirm } from "antd";
import { EditOutlined, DeleteOutlined } from "@ant-design/icons";
import { type User, UserRole } from "@/domain/entities";
import type { ColumnsType } from "antd/es/table";

// Keyed by the whole enum on purpose: a role added without a colour here
// stops the build, instead of quietly rendering as some other role's tag.
const ROLE_TAG_COLOR: Record<UserRole, string> = {
  [UserRole.ADMIN]: "red",
  [UserRole.TECHNICIAN]: "blue",
  [UserRole.CS]: "purple",
  [UserRole.VIEWER]: "green",
};

interface UserTableProps {
  users: User[];
  loading: boolean;
  onEdit: (user: User) => void;
  onDelete: (id: string) => void;
}

export function UserTable({
  users,
  loading,
  onEdit,
  onDelete,
}: UserTableProps) {
  const columns: ColumnsType<User> = [
    {
      title: "Username",
      dataIndex: "username",
      key: "username",
      fixed: "left",
      width: 150,
    },
    {
      title: "Email",
      dataIndex: "email",
      key: "email",
      width: 200,
      responsive: ["md"],
    },
    {
      title: "Role",
      dataIndex: "role",
      key: "role",
      width: 120,
      render: (role: UserRole) => (
        <Tag color={ROLE_TAG_COLOR[role]}>{role.toUpperCase()}</Tag>
      ),
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
            title="Hapus user ini?"
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
      dataSource={users}
      loading={loading}
      rowKey="id"
      pagination={{ pageSize: 10 }}
      scroll={{ x: 800 }}
    />
  );
}
