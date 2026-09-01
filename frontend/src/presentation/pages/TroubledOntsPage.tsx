import { useState } from "react";
import { Card, Table, Tag, Radio, Empty, Alert } from "antd";
import type { ColumnsType } from "antd/es/table";
import { PageHeader } from "@/presentation/components/common/PageHeader";
import { useTroubledOnts } from "@/application/hooks";
import type { TroubledOnt } from "@/domain/entities";

const WINDOWS = [
  { label: "24 jam", hours: 24 },
  { label: "7 hari", hours: 168 },
];

/** durasi renders minutes the way an operator reads them. */
function durasi(minutes: number): string {
  if (minutes < 60) return `${minutes} mnt`;
  const hours = Math.floor(minutes / 60);
  return `${hours} jam ${minutes % 60} mnt`;
}

const columns: ColumnsType<TroubledOnt> = [
  { title: "OLT", dataIndex: "oltName", width: 110 },
  {
    title: "Posisi",
    width: 120,
    render: (_, r) => `ONU-${r.portId}:${r.ontNumber}`,
  },
  { title: "Pelanggan", dataIndex: "name", ellipsis: true },
  { title: "Serial", dataIndex: "serialNumber", width: 150 },
  {
    title: "Trap",
    dataIndex: "trapCount",
    width: 110,
    align: "right",
    defaultSortOrder: "descend",
    sorter: (a, b) => a.trapCount - b.trapCount,
    render: (value: number) => value.toLocaleString("id-ID"),
  },
  {
    title: "Waktu mati",
    dataIndex: "downMinutes",
    width: 140,
    align: "right",
    sorter: (a, b) => a.downMinutes - b.downMinutes,
    render: (value: number) => durasi(value),
  },
  {
    title: "Status",
    dataIndex: "status",
    width: 110,
    render: (status: string) => (
      <Tag color={status === "online" ? "green" : "red"}>
        {status.toUpperCase()}
      </Tag>
    ),
  },
];

/**
 * TroubledOntsPage ranks subscribers by how much they have been churning.
 *
 * The ONT list answers "is this subscriber up", and an ONU that drops and
 * returns every few seconds passes that test every time anyone looks. Counting
 * the traps it sends is what makes such a subscriber visible at all; the outage
 * beside it says what the churn has cost the person paying for the line.
 */
export function TroubledOntsPage() {
  const [hours, setHours] = useState(24);
  const { data, isLoading } = useTroubledOnts(hours);

  return (
    <div>
      <PageHeader
        title="Pelanggan Bermasalah"
        description="Diperingkat dari trap yang dikirim OLT — termasuk pelanggan yang statusnya terbaca online"
      />

      <Card
        extra={
          <Radio.Group
            value={hours}
            onChange={(e) => setHours(e.target.value)}
            optionType="button"
            buttonStyle="solid"
          >
            {WINDOWS.map((w) => (
              <Radio.Button key={w.hours} value={w.hours}>
                {w.label}
              </Radio.Button>
            ))}
          </Radio.Group>
        }
      >
        {!isLoading && (data?.length ?? 0) === 0 ? (
          <Empty description="Tidak ada pelanggan yang beralarm dalam rentang ini" />
        ) : (
          <>
            <Alert
              type="info"
              showIcon
              style={{ marginBottom: 16 }}
              message="Kolom Trap menghitung seluruh trap, bukan hanya alarm"
              description="Tingkat keparahan baru disimpan sejak 31 Agustus 2026, jadi menyaring hanya alarm akan mengosongkan sebagian besar rentang. Alarm dan cleared datang berpasangan, sehingga jumlahnya tetap mengukur seberapa bergejolak ONU itu."
            />
            <Table
              rowKey="ontId"
              loading={isLoading}
              dataSource={data ?? []}
              columns={columns}
              size="small"
              scroll={{ x: 900 }}
              pagination={{ pageSize: 20, showSizeChanger: false }}
            />
          </>
        )}
      </Card>
    </div>
  );
}

export default TroubledOntsPage;
