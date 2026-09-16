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

// This count spans every ending, so it overlaps the two counts beside it. Say
// so, or the three look like they should add up to the answered ones and do
// not.
const SYSTEM_DELAYED_HINT =
  "Dihitung apa pun akhirnya, jadi satu giliran bisa masuk ke angka ini sekaligus ke “Ditutup tanpa balasan” atau “Ditinggal”. Waktunya tidak pernah masuk angka tim.";

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
