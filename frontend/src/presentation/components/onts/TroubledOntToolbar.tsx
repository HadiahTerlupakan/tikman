import { Select, Space, Tag } from "antd";

const STATUS_OPTIONS = [
  { value: "online", label: "Online" },
  { value: "los", label: "LOS" },
  { value: "dying_gasp", label: "Dying gasp" },
  { value: "offline", label: "Offline" },
];

interface TroubledOntToolbarProps {
  status?: string;
  onStatusChange: (status?: string) => void;
  ponFilter?: { slot: number; port: number };
  shownCount: number;
  totalCount: number;
  onClearPonFilter: () => void;
}

/**
 * TroubledOntToolbar holds the status filter — a PON has no status, so this
 * control lives only on the subscriber tab — and, once a PON has been picked
 * on the other tab, the tag that states the narrowing. The summary below
 * keeps describing the whole matching population, so the narrowing has to be
 * said here or the two numbers on screen would contradict.
 */
export function TroubledOntToolbar({
  status,
  onStatusChange,
  ponFilter,
  shownCount,
  totalCount,
  onClearPonFilter,
}: TroubledOntToolbarProps) {
  return (
    <Space style={{ marginBottom: 16 }}>
      <Select
        allowClear
        style={{ width: 150 }}
        placeholder="Semua status"
        value={status}
        onChange={onStatusChange}
        options={STATUS_OPTIONS}
      />
      {ponFilter && (
        <Tag closable onClose={onClearPonFilter}>
          {`Kartu ${ponFilter.slot} · PON ${ponFilter.port} · ${shownCount} dari ${totalCount} pelanggan`}
        </Tag>
      )}
    </Space>
  );
}
