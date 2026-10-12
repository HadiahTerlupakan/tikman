import { useEffect, useState } from "react";
import { Input, Segmented } from "antd";
import { SearchOutlined } from "@ant-design/icons";
import { inboxViews, type InboxView } from "./inboxView";
import { colors } from "@/shared/theme/colors";

interface InboxFilterBarProps {
  view: InboxView;
  onViewChange: (view: InboxView) => void;
  /** Debounced: a request per keystroke would refetch the list four times
   * while someone types a name. */
  onSearchChange: (search: string) => void;
}

export function InboxFilterBar({
  view,
  onViewChange,
  onSearchChange,
}: InboxFilterBarProps) {
  const [typed, setTyped] = useState("");

  useEffect(() => {
    const timer = setTimeout(() => onSearchChange(typed.trim()), 300);
    return () => clearTimeout(timer);
  }, [typed, onSearchChange]);

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
      <Input
        allowClear
        prefix={<SearchOutlined style={{ color: colors.textMuted }} />}
        placeholder="Cari nama atau nomor"
        value={typed}
        onChange={(e) => setTyped(e.target.value)}
        variant="filled"
        style={{ borderRadius: 18 }}
      />
      <Segmented
        block
        size="small"
        value={view}
        options={inboxViews}
        onChange={(value) => onViewChange(value as InboxView)}
      />
    </div>
  );
}
