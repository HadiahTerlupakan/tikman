import { Table, Button, Space, Tag, Popconfirm } from 'antd';
import { EditOutlined, DeleteOutlined } from '@ant-design/icons';
import { type Olt, OltStatus } from '@/domain/entities';
import type { ColumnsType } from 'antd/es/table';

interface OltTableProps {
  olts: Olt[];
  loading: boolean;
  onEdit: (olt: Olt) => void;
  onDelete: (id: string) => void;
}

export function OltTable({ olts, loading, onEdit, onDelete }: OltTableProps) {
  const getStatusColor = (status: OltStatus) => {
    switch (status) {
      case OltStatus.ONLINE:
        return 'green';
      case OltStatus.OFFLINE:
        return 'red';
      case OltStatus.ERROR:
        return 'orange';
      default:
        return 'default';
    }
  };

  const columns: ColumnsType<Olt> = [
    {
      title: 'OLT Name',
      dataIndex: 'name',
      key: 'name',
    },
    {
      title: 'Site',
      dataIndex: 'siteName',
      key: 'siteName',
    },
    {
      title: 'IP Address',
      dataIndex: 'ipAddress',
      key: 'ipAddress',
    },
    {
      title: 'Protocol',
      dataIndex: 'protocol',
      key: 'protocol',
      render: (protocol: string) => protocol.toUpperCase(),
    },
    {
      title: 'Status',
      dataIndex: 'status',
      key: 'status',
      render: (status: OltStatus) => (
        <Tag color={getStatusColor(status)}>{status.toUpperCase()}</Tag>
      ),
    },
    {
      title: 'Last Seen',
      dataIndex: 'lastSeen',
      key: 'lastSeen',
      render: (date: string | null) =>
        date ? new Date(date).toLocaleString('id-ID') : '-',
    },
    {
      title: 'Actions',
      key: 'actions',
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
            title="Hapus OLT ini?"
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
      dataSource={olts}
      loading={loading}
      rowKey="id"
      pagination={{ pageSize: 10 }}
    />
  );
}
