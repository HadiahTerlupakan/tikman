import { Button, Input, Select, Tooltip } from "antd";
import {
  PlusOutlined,
  ReloadOutlined,
  SearchOutlined,
} from "@ant-design/icons";
import type { WaAccount } from "@/domain/entities";
import { colors } from "@/shared/theme/colors";

interface GroupListToolbarProps {
  search: string;
  onSearchChange: (search: string) => void;
  accounts: WaAccount[];
  waAccountId?: string;
  onAccountChange: (waAccountId?: string) => void;
  refreshing: boolean;
  onRefresh: () => void;
  /** Absent for anyone but an admin, who alone may create a group. */
  onCreate?: () => void;
}

/** Search, number filter, refresh and create, above the group list. */
export function GroupListToolbar({
  search,
  onSearchChange,
  accounts,
  waAccountId,
  onAccountChange,
  refreshing,
  onRefresh,
  onCreate,
}: GroupListToolbarProps) {
  return (
    <div
      style={{
        padding: "10px 12px",
        borderBottom: `1px solid ${colors.border}`,
        display: "flex",
        flexDirection: "column",
        gap: 8,
      }}
    >
      <div style={{ display: "flex", gap: 6 }}>
        <Input
          allowClear
          prefix={<SearchOutlined style={{ color: colors.textMuted }} />}
          placeholder="Cari nama grup"
          value={search}
          onChange={(e) => onSearchChange(e.target.value)}
          variant="filled"
          style={{ borderRadius: 18 }}
        />
        <Tooltip title="Baca ulang daftar grup dari WhatsApp">
          <Button
            icon={<ReloadOutlined />}
            loading={refreshing}
            onClick={onRefresh}
            aria-label="Muat ulang daftar grup"
          />
        </Tooltip>
        {onCreate && (
          <Tooltip title="Buat grup">
            <Button
              type="primary"
              icon={<PlusOutlined />}
              onClick={onCreate}
              aria-label="Buat grup"
            />
          </Tooltip>
        )}
      </div>
      {/* With one number there is nothing to choose between. */}
      {accounts.length > 1 && (
        <Select
          allowClear
          size="small"
          placeholder="Semua nomor"
          value={waAccountId}
          onChange={(value?: string) => onAccountChange(value)}
          options={accounts.map((a) => ({ value: a.id, label: a.label }))}
        />
      )}
    </div>
  );
}
