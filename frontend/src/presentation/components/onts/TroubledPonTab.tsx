import { Empty, Skeleton } from "antd";
import type { PonHealth } from "@/domain/entities";
import { PonTopology } from "./PonTopology";

interface TroubledPonTabProps {
  oltId?: string;
  ponHealth: PonHealth | undefined;
  isLoading: boolean;
  onSelectPon: (slot: number, port: number) => void;
}

/**
 * TroubledPonTab draws the fault tree for one OLT, or says why it can't yet.
 *
 * One chassis at a time is what keeps the topology readable, so nothing is
 * drawn until an OLT is chosen — a topology of every OLT at once is the
 * thing this view exists to avoid.
 */
export function TroubledPonTab({
  oltId,
  ponHealth,
  isLoading,
  onSelectPon,
}: TroubledPonTabProps) {
  if (!oltId) {
    return <Empty description="Pilih OLT untuk melihat topologinya" />;
  }
  if (isLoading || !ponHealth) {
    return <Skeleton active />;
  }
  return <PonTopology health={ponHealth} onSelectPon={onSelectPon} />;
}
