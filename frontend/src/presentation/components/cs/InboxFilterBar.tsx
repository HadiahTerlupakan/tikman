import { useEffect, useState } from "react";
import { Input, Segmented } from "antd";
import { SearchOutlined } from "@ant-design/icons";
import type { CsConversationFilter } from "@/domain/entities";
import { colors } from "@/shared/theme/colors";

/** The four views a CS switches between. They are mutually exclusive on the
 * backend — the first one set wins — which is why this is a segmented control
 * and not a row of checkboxes. */
export type InboxView = "semua" | "milik-saya" | "belum-dipegang" | "selesai";

const views: { value: InboxView; label: string }[] = [
  { value: "semua", label: "Semua" },
  { value: "milik-saya", label: "Milik saya" },
  { value: "belum-dipegang", label: "Belum dipegang" },
  { value: "selesai", label: "Selesai" },
];

/** Turns a view and a search term into the filter the API expects. */
export function filterFor(
  view: InboxView,
  search: string,
): CsConversationFilter {
  const base: CsConversationFilter = search ? { search } : {};
  switch (view) {
    case "milik-saya":
      return { ...base, mine: true };
    case "belum-dipegang":
      return { ...base, unassigned: true };
    case "selesai":
      return { ...base, closed: true };
    default:
      return base;
  }
}

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
        options={views}
        onChange={(value) => onViewChange(value as InboxView)}
      />
    </div>
  );
}
