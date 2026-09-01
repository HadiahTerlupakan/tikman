import { useMemo } from "react";
import { Table, theme } from "antd";
import type { TroubledOnt } from "@/domain/entities";
import { readsFineButIsNot, troubledColumns } from "./troubledColumns";

interface TroubledOntTableProps {
  rows: TroubledOnt[];
  isLoading: boolean;
  hours: number;
  worstTrapCount: number;
}

/**
 * TroubledOntTable renders one page of ranked subscribers.
 *
 * `worstTrapCount` is supplied by the caller rather than derived from `rows`
 * here, because the trap-count colour scale must stay relative to the whole
 * matching population even when `rows` itself has been narrowed to one PON.
 */
export function TroubledOntTable({
  rows,
  isLoading,
  hours,
  worstTrapCount,
}: TroubledOntTableProps) {
  const { token } = theme.useToken();

  const columns = useMemo(
    () =>
      troubledColumns(
        worstTrapCount,
        hours * 60,
        token.colorError,
        token.colorWarning,
      ),
    [worstTrapCount, hours, token.colorError, token.colorWarning],
  );

  return (
    <Table
      rowKey="ontId"
      loading={isLoading}
      dataSource={rows}
      columns={columns}
      size="small"
      scroll={{ x: 900 }}
      pagination={{ pageSize: 20, showSizeChanger: false }}
      rowClassName={(record) =>
        readsFineButIsNot(record) ? "troubled-row-contradiction" : ""
      }
      locale={{
        emptyText:
          "Pelanggan PON ini tidak masuk daftar peringkat pada rentang ini",
      }}
    />
  );
}
