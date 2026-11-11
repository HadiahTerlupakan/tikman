import {
  Button,
  Popconfirm,
  Space,
  Switch,
  Table,
  Tag,
  Typography,
} from "antd";
import type { ColumnsType } from "antd/es/table";
import {
  DeleteOutlined,
  EditOutlined,
  NotificationOutlined,
  SendOutlined,
} from "@ant-design/icons";
import type { GraphReport } from "@/domain/entities";
import { formatAttemptAt, formatSentOn } from "./graphReportForm";

const { Text } = Typography;

interface GraphReportTableProps {
  reports: GraphReport[];
  loading: boolean;
  sendingId?: string;
  togglingId?: string;
  onToggle: (report: GraphReport, enabled: boolean) => void;
  onSend: (report: GraphReport) => void;
  onEdit: (report: GraphReport) => void;
  onDelete: (report: GraphReport) => void;
}

function portLabels(report: GraphReport): string {
  return [...report.ports]
    .sort((a, b) => a.position - b.position)
    .map((p) => p.label)
    .join(", ");
}

function GroupCell({ report }: { report: GraphReport }) {
  return (
    <Space direction="vertical" size={0}>
      <Space size={4} wrap>
        {report.groupName}
        {report.mentionAll && (
          <Tag
            icon={<NotificationOutlined />}
            title="Semua anggota grup ditag saat laporan dikirim"
          >
            Tag semua
          </Tag>
        )}
      </Space>
      <Text type="secondary" style={{ fontSize: 12 }}>
        Lewat {report.waAccountLabel}
      </Text>
    </Space>
  );
}

function LastError({ report }: { report: GraphReport }) {
  if (!report.lastError) return <Text type="secondary">—</Text>;
  return (
    <Space direction="vertical" size={0}>
      <Text type="danger">{report.lastError}</Text>
      <Text type="secondary" style={{ fontSize: 12 }}>
        {formatAttemptAt(report.lastAttemptAt)}
      </Text>
    </Space>
  );
}

interface ReportActionsProps {
  report: GraphReport;
  sending: boolean;
  onSend: (report: GraphReport) => void;
  onEdit: (report: GraphReport) => void;
  onDelete: (report: GraphReport) => void;
}

function ReportActions({
  report: r,
  sending,
  onSend,
  onEdit,
  onDelete,
}: ReportActionsProps) {
  return (
    <Space>
      <Popconfirm
        title={`Kirim laporan ke ${r.groupName} sekarang?`}
        okText="Kirim"
        cancelText="Batal"
        onConfirm={() => onSend(r)}
      >
        <Button type="link" icon={<SendOutlined />} loading={sending}>
          Kirim sekarang
        </Button>
      </Popconfirm>
      <Button type="link" icon={<EditOutlined />} onClick={() => onEdit(r)}>
        Ubah
      </Button>
      <Popconfirm
        title="Hapus laporan ini?"
        okText="Hapus"
        cancelText="Batal"
        okButtonProps={{ danger: true }}
        onConfirm={() => onDelete(r)}
      >
        <Button type="link" danger icon={<DeleteOutlined />}>
          Hapus
        </Button>
      </Popconfirm>
    </Space>
  );
}

export function GraphReportTable(props: GraphReportTableProps) {
  const { reports, loading, sendingId, togglingId } = props;
  const columns: ColumnsType<GraphReport> = [
    {
      title: "Grup",
      key: "group",
      render: (_, r) => <GroupCell report={r} />,
    },
    { title: "Pelanggan", dataIndex: "customerName", key: "customer" },
    {
      title: "Port",
      key: "ports",
      render: (_, r) => `${r.ports.length} — ${portLabels(r)}`,
    },
    {
      title: "Jam kirim",
      key: "sendTime",
      width: 110,
      render: (_, r) => `${r.sendTime} WIB`,
    },
    {
      title: "Aktif",
      key: "enabled",
      width: 80,
      render: (_, r) => (
        <Switch
          checked={r.enabled}
          loading={togglingId === r.id}
          onChange={(checked) => props.onToggle(r, checked)}
        />
      ),
    },
    {
      title: "Terakhir terkirim",
      key: "lastSentOn",
      width: 140,
      render: (_, r) => formatSentOn(r.lastSentOn),
    },
    {
      title: "Error terakhir",
      key: "lastError",
      render: (_, r) => <LastError report={r} />,
    },
    {
      title: "Aksi",
      key: "actions",
      fixed: "right",
      width: 280,
      render: (_, r) => (
        <ReportActions
          report={r}
          sending={sendingId === r.id}
          onSend={props.onSend}
          onEdit={props.onEdit}
          onDelete={props.onDelete}
        />
      ),
    },
  ];

  return (
    <Table
      columns={columns}
      dataSource={reports}
      loading={loading}
      rowKey="id"
      pagination={false}
      scroll={{ x: 1000 }}
    />
  );
}
