import { Row, Col, Alert, Progress } from "antd";
import {
  UserOutlined,
  EnvironmentOutlined,
  ApiOutlined,
  ClusterOutlined,
} from "@ant-design/icons";
import {
  useUsers,
  useSites,
  useOlts,
  useDashboardStats,
  useHealth,
  useWireguardPeers,
} from "@/application/hooks";
import { useAuthStore } from "@/application/stores";
import { UserRole } from "@/domain/entities";
import { colors } from "@/shared/theme";
import { PageHeader, DarkCard } from "../components/common";
import {
  LastUpdated,
  OltBreakdownTable,
  StatusBar,
  SummaryCard,
  SystemHealthCard,
  VpnStatusCard,
  WeakSignalList,
} from "../components/dashboard";
import {
  availabilityTone,
  rankOltRows,
  summariseOlts,
  toWeakSignals,
  uptimePercent,
} from "./dashboardStats";

export default function DashboardPage() {
  const user = useAuthStore((state) => state.user);
  const isAdmin = user?.role === UserRole.ADMIN;

  const { data: users, isLoading: usersLoading } = useUsers();
  const {
    data: sites,
    isLoading: sitesLoading,
    error: sitesError,
  } = useSites();
  const { data: olts, isLoading: oltsLoading, error: oltsError } = useOlts();
  const {
    data: stats,
    isLoading: statsLoading,
    isFetching: statsFetching,
    dataUpdatedAt,
    error: statsError,
  } = useDashboardStats();
  const { data: health, isLoading: healthLoading } = useHealth();
  const {
    data: peers,
    isLoading: peersLoading,
    error: peersError,
  } = useWireguardPeers();

  const oltSummary = summariseOlts(olts);
  const ontCounts = stats?.onts;
  const oltUptime = uptimePercent(oltSummary.online, oltSummary.total);
  const ontUptime = uptimePercent(
    ontCounts?.online ?? 0,
    ontCounts?.total ?? 0,
  );
  const oltRows = rankOltRows(stats?.olts);
  const signals = toWeakSignals(stats?.weakestSignals);

  // A failed query must render a dash, not 0 — a zero here is indistinguishable
  // from a healthy empty system.
  const ontValue = (n: number | undefined) =>
    statsError || n === undefined ? null : n;

  const failed = [
    sitesError && "sites",
    oltsError && "OLTs",
    statsError && "ONT statistics",
  ].filter(Boolean);

  return (
    <div>
      <PageHeader
        title="Dashboard Overview"
        description="Monitor your OLT provisioning system in real-time"
        extra={
          <LastUpdated updatedAt={dataUpdatedAt} isFetching={statsFetching} />
        }
      />

      {failed.length > 0 && (
        <Alert
          type="error"
          showIcon
          style={{ marginBottom: 24 }}
          message={`Could not load ${failed.join(", ")}`}
          description="The figures below are incomplete. Values that could not be loaded show a dash."
        />
      )}

      <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
        {isAdmin && (
          <Col xs={24} sm={12} xl={6}>
            <SummaryCard
              label="Total Users"
              value={users?.length ?? 0}
              isLoading={usersLoading}
              icon={<UserOutlined />}
            />
          </Col>
        )}
        <Col xs={24} sm={12} xl={isAdmin ? 6 : 8}>
          <SummaryCard
            label="Total Sites"
            value={sitesError ? null : sites?.length ?? 0}
            isLoading={sitesLoading}
            icon={<EnvironmentOutlined />}
          />
        </Col>
        <Col xs={24} sm={12} xl={isAdmin ? 6 : 8}>
          <SummaryCard
            label="Total OLTs"
            value={oltsError ? null : oltSummary.total}
            isLoading={oltsLoading}
            icon={<ApiOutlined />}
            caption={
              oltUptime === null ? "No OLTs registered" : `${oltUptime}% online`
            }
          />
        </Col>
        <Col xs={24} sm={12} xl={isAdmin ? 6 : 8}>
          <SummaryCard
            label="Total ONTs"
            value={ontValue(ontCounts?.total)}
            isLoading={statsLoading}
            icon={<ClusterOutlined />}
            caption={
              ontUptime === null
                ? "No ONTs registered"
                : `${ontCounts?.online ?? 0} of ${ontCounts?.total ?? 0} online`
            }
          />
        </Col>
      </Row>

      <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
        <Col xs={24} lg={16}>
          <DarkCard title="ONT Status" style={{ height: "100%" }}>
            <StatusBar
              isLoading={statsLoading}
              total={ontCounts?.total ?? 0}
              emptyText="No ONTs registered yet"
              segments={[
                {
                  label: "Online",
                  tone: "success",
                  value: ontValue(ontCounts?.online),
                },
                {
                  label: "Offline",
                  tone: "neutral",
                  value: ontValue(ontCounts?.offline),
                },
                {
                  label: "LOS",
                  tone: "danger",
                  value: ontValue(ontCounts?.los),
                  hint: "Signal lost",
                },
                {
                  label: "Dying Gasp",
                  tone: "warning",
                  value: ontValue(ontCounts?.dyingGasp),
                  hint: "Power lost",
                },
                {
                  label: "Unknown",
                  tone: "neutral",
                  value: ontValue(ontCounts?.unknown),
                  hint: "Not yet polled",
                },
              ]}
            />
          </DarkCard>
        </Col>

        <Col xs={24} lg={8}>
          <DarkCard title="Network Availability" style={{ height: "100%" }}>
            <div
              style={{
                display: "flex",
                flexDirection: "column",
                gap: 20,
                alignItems: "center",
              }}
            >
              <Progress
                type="dashboard"
                percent={ontUptime ?? 0}
                strokeWidth={7}
                strokeColor={colors[availabilityTone(ontUptime)]}
                trailColor="rgba(161, 161, 170, 0.14)"
                format={(p) => (
                  <span
                    style={{
                      color: colors.textPrimary,
                      fontSize: 22,
                      fontWeight: 600,
                      fontVariantNumeric: "tabular-nums",
                    }}
                  >
                    {ontUptime === null ? "—" : `${p}%`}
                  </span>
                )}
              />
              <div
                style={{
                  color: colors.textSecondary,
                  fontSize: 12,
                  textAlign: "center",
                }}
              >
                {ontUptime === null
                  ? "No ONTs to measure"
                  : `${ontCounts?.online ?? 0} of ${ontCounts?.total ?? 0} ONTs online`}
              </div>
            </div>
          </DarkCard>
        </Col>
      </Row>

      <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
        <Col xs={24} lg={16}>
          <DarkCard title="Status by OLT" style={{ height: "100%" }}>
            <OltBreakdownTable
              rows={statsError ? [] : oltRows}
              isLoading={statsLoading}
            />
          </DarkCard>
        </Col>

        <Col xs={24} lg={8}>
          <VpnStatusCard
            peers={peers}
            olts={olts}
            isLoading={peersLoading}
            isError={!!peersError}
          />
        </Col>
      </Row>

      <Row gutter={[16, 16]}>
        <Col xs={24} lg={16}>
          <DarkCard title="Weakest Optical Signal" style={{ height: "100%" }}>
            <WeakSignalList signals={signals} isLoading={statsLoading} />
          </DarkCard>
        </Col>

        <Col xs={24} lg={8}>
          <SystemHealthCard health={health} isLoading={healthLoading} />
        </Col>
      </Row>
    </div>
  );
}
