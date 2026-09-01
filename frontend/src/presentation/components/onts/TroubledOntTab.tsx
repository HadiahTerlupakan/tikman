import { Empty, theme } from "antd";
import type { TroubledOnt, TroubledSummary } from "@/domain/entities";
import { readsFineButIsNot } from "./troubledColumns";
import { durasi } from "./troubledDuration";
import { TroubledOntTable } from "./TroubledOntTable";
import { TroubledOntToolbar } from "./TroubledOntToolbar";

const SUMMARY_ROW_STYLE = {
  display: "flex",
  flexWrap: "wrap" as const,
  gap: 32,
  padding: "4px 0 20px",
};

function figureValueStyle(color: string) {
  return {
    fontSize: 26,
    fontWeight: 600,
    lineHeight: 1.2,
    fontVariantNumeric: "tabular-nums" as const,
    color,
  };
}

/**
 * Summary states the case the table then makes row by row.
 *
 * The middle figure is the point of the whole page: subscribers the ONT list
 * reports as online, which lost service anyway inside the same window. Counted
 * over every matching ONT rather than the page shown — with hundreds churning
 * on one chassis, a total drawn from the fifty listed would mislead.
 */
function Summary({
  ontCount,
  hiddenByStatus,
  totalDownMinutes,
}: {
  ontCount: number;
  hiddenByStatus: number;
  totalDownMinutes: number;
}) {
  const { token } = theme.useToken();

  const figures = [
    { value: ontCount.toLocaleString("id-ID"), label: "pelanggan beralarm" },
    {
      value: hiddenByStatus.toLocaleString("id-ID"),
      label: "terbaca online, tetap sempat mati",
      accent: token.colorWarning,
    },
    { value: durasi(totalDownMinutes), label: "akumulasi waktu mati" },
  ];

  return (
    <div style={SUMMARY_ROW_STYLE}>
      {figures.map((figure) => (
        <div key={figure.label} style={{ minWidth: 150 }}>
          <div style={figureValueStyle(figure.accent ?? token.colorText)}>
            {figure.value}
          </div>
          <div style={{ fontSize: 12, color: token.colorTextSecondary }}>
            {figure.label}
          </div>
        </div>
      ))}
    </div>
  );
}

interface TroubledOntTabProps {
  rows: TroubledOnt[];
  summary?: TroubledSummary;
  isLoading: boolean;
  hours: number;
  status?: string;
  onStatusChange: (status?: string) => void;
  ponFilter?: { slot: number; port: number };
  onClearPonFilter: () => void;
}

/**
 * TroubledOntTab is the subscriber-ranked view: the status filter, the PON
 * narrowing tag once a port has been picked on the other tab, and the ranked
 * table itself.
 */
export function TroubledOntTab({
  rows,
  summary,
  isLoading,
  hours,
  status,
  onStatusChange,
  ponFilter,
  onClearPonFilter,
}: TroubledOntTabProps) {
  // Scaled to the unfiltered population, not `shown`: a mild PON's worst row
  // would otherwise paint as red as the network's genuine worst once that PON
  // is the only thing on screen.
  const worstTrapCount = rows[0]?.trapCount ?? 0;
  const hiddenByStatus = rows.filter(readsFineButIsNot).length;
  const shown = ponFilter
    ? rows.filter((r) => r.portId === ponFilter.port)
    : rows;

  return (
    <>
      <TroubledOntToolbar
        status={status}
        onStatusChange={onStatusChange}
        ponFilter={ponFilter}
        shownCount={shown.length}
        totalCount={summary?.ontCount ?? 0}
        onClearPonFilter={onClearPonFilter}
      />
      {!isLoading && rows.length === 0 ? (
        <Empty description="Tidak ada pelanggan yang beralarm dalam rentang ini" />
      ) : (
        <>
          <Summary
            ontCount={summary?.ontCount ?? 0}
            hiddenByStatus={hiddenByStatus}
            totalDownMinutes={summary?.totalDownMinutes ?? 0}
          />
          <TroubledOntTable
            rows={shown}
            isLoading={isLoading}
            hours={hours}
            worstTrapCount={worstTrapCount}
          />
        </>
      )}
    </>
  );
}
