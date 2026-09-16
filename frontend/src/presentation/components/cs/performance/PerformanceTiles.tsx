import {
  Button,
  Card,
  Col,
  Row,
  Space,
  Statistic,
  Tooltip,
  Typography,
} from "antd";
import { Link } from "react-router-dom";
import type { PerformanceSummary } from "@/domain/entities";
import { formatMinutes, formatPercent } from "./performanceFormat";

const { Text } = Typography;

// The drawer tags every delayed wait regardless of how it ended; this tile
// counts only the ones that were answered. Someone opening the drawer to
// check this count will see more tags than the tile claims unless this says
// why.
const SYSTEM_DELAYED_HINT =
  "Hanya giliran yang sudah dibalas. Di daftar giliran, tanda ini juga muncul pada giliran yang ditutup tanpa balasan.";

interface PerformanceTilesProps {
  summary: PerformanceSummary;
  onShowWaits: () => void;
}

/** What the customers of this period got, and who is waiting right now. */
export function PerformanceTiles({
  summary,
  onShowWaits,
}: PerformanceTilesProps) {
  const { team, waiting, targetMinutes } = summary;
  return (
    <Card
      title="Tim"
      extra={<Button onClick={onShowWaits}>Daftar giliran</Button>}
    >
      <Row gutter={[16, 16]}>
        <Col xs={12} md={6}>
          <Statistic title="Giliran dibalas" value={team.replies.count} />
        </Col>
        <Col xs={12} md={6}>
          <Statistic
            title="Median"
            value={formatMinutes(team.replies.medianMinutes)}
          />
        </Col>
        <Col xs={12} md={6}>
          <Statistic
            title={`≤ ${targetMinutes} menit`}
            value={formatPercent(team.replies.withinTargetPct)}
          />
        </Col>
        <Col xs={12} md={6}>
          <Statistic
            title="90% tercepat"
            value={formatMinutes(team.replies.p90Minutes)}
          />
        </Col>
      </Row>
      <Space wrap size="large" style={{ marginTop: 16 }}>
        <Text type="secondary">
          Ditutup tanpa balasan: {team.closedWithoutReply}
        </Text>
        <Text type="secondary">Ditinggal: {team.abandoned}</Text>
        <Tooltip title={SYSTEM_DELAYED_HINT}>
          <Text type="secondary">Tertunda sistem: {team.systemDelayed}</Text>
        </Tooltip>
        <Link to="/cs?view=belum-dibalas">
          Sedang menunggu: {waiting.count} pelanggan
          {waiting.longestMinutes !== null &&
            `, terlama ${formatMinutes(waiting.longestMinutes)}`}
        </Link>
      </Space>
    </Card>
  );
}
