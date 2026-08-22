import { Row, Col, Alert, Empty, Progress, Skeleton } from "antd";
import {
  UserOutlined,
  EnvironmentOutlined,
  ApiOutlined,
  ClusterOutlined,
  WarningOutlined,
  DisconnectOutlined,
  CheckCircleOutlined,
} from "@ant-design/icons";
import {
  useUsers,
  useSites,
  useOlts,
  useOnts,
  useHealth,
} from "@/application/hooks";
import { useAuthStore } from "@/application/stores";
import { UserRole } from "@/domain/entities";
import { colors } from "@/shared/theme";
import { PageHeader, DarkCard } from "../components/common";
import {
  StatusTile,
  SummaryCard,
  SystemHealthCard,
} from "../components/dashboard";
import {
  availabilityTone,
  summariseOlts,
  summariseOnts,
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
  const { data: ontPage, isLoading: ontsLoading, error: ontsError } = useOnts();
  const { data: health, isLoading: healthLoading } = useHealth();

  const oltSummary = summariseOlts(olts);
  const ontSummary = summariseOnts(ontPage?.data);
  const oltUptime = uptimePercent(oltSummary.online, oltSummary.total);
  const ontUptime = uptimePercent(ontSummary.online, ontSummary.total);
  const availabilityColor = colors[availabilityTone(ontUptime)];

  // A failed query must render a dash, not 0 — a zero here is indistinguishable
  // from a healthy empty system.
  const oltValue = (n: number) => (oltsError ? null : n);
  const ontValue = (n: number) => (ontsError ? null : n);

  const failed = [
    sitesError && "sites",
    oltsError && "OLTs",
    ontsError && "ONTs",
  ].filter(Boolean);

  return (
    <div>
      <PageHeader
        title="Dashboard Overview"
        description="Monitor your OLT provisioning system in real-time"
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
            value={ontsError ? null : ontSummary.total}
            isLoading={ontsLoading}
            icon={<ClusterOutlined />}
            caption={
              ontUptime === null
                ? "No ONTs registered"
                : `${ontSummary.online} of ${ontSummary.total} online`
            }
          />
        </Col>
      </Row>

      <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
        <Col xs={24} lg={16}>
          <DarkCard title="OLT Status Distribution">
            {oltsLoading ? (
              <Skeleton active paragraph={{ rows: 3 }} title={false} />
            ) : oltSummary.total === 0 && !oltsError ? (
              <Empty
                description={
                  <span style={{ color: colors.textSecondary }}>
                    No OLTs registered yet
                  </span>
                }
              />
            ) : (
              <Row gutter={[16, 16]}>
                <Col xs={24} sm={8}>
                  <StatusTile
                    tone="success"
                    label="Online"
                    value={oltValue(oltSummary.online)}
                    total={oltSummary.total}
                    icon={<CheckCircleOutlined />}
                  />
                </Col>
                <Col xs={24} sm={8}>
                  <StatusTile
                    tone="neutral"
                    label="Offline"
                    value={oltValue(oltSummary.offline)}
                    total={oltSummary.total}
                    icon={<DisconnectOutlined />}
                  />
                </Col>
                <Col xs={24} sm={8}>
                  <StatusTile
                    tone="danger"
                    label="Error"
                    value={oltValue(oltSummary.error)}
                    total={oltSummary.total}
                    icon={<WarningOutlined />}
                  />
                </Col>
              </Row>
            )}
          </DarkCard>
        </Col>

        <Col xs={24} lg={8}>
          <SystemHealthCard health={health} isLoading={healthLoading} />
        </Col>
      </Row>

      <Row gutter={[16, 16]}>
        <Col xs={24} lg={16}>
          <DarkCard title="ONT Status">
            {ontsLoading ? (
              <Skeleton active paragraph={{ rows: 3 }} title={false} />
            ) : ontSummary.total === 0 && !ontsError ? (
              <Empty
                description={
                  <span style={{ color: colors.textSecondary }}>
                    No ONTs registered yet
                  </span>
                }
              />
            ) : (
              <Row gutter={[16, 16]}>
                <Col xs={12} sm={6}>
                  <StatusTile
                    tone="success"
                    label="Online"
                    value={ontValue(ontSummary.online)}
                    total={ontSummary.total}
                  />
                </Col>
                <Col xs={12} sm={6}>
                  <StatusTile
                    tone="neutral"
                    label="Offline"
                    value={ontValue(ontSummary.offline)}
                    total={ontSummary.total}
                  />
                </Col>
                <Col xs={12} sm={6}>
                  <StatusTile
                    tone="danger"
                    label="LOS"
                    value={ontValue(ontSummary.los)}
                    total={ontSummary.total}
                    hint="Signal lost"
                  />
                </Col>
                <Col xs={12} sm={6}>
                  <StatusTile
                    tone="warning"
                    label="Dying Gasp"
                    value={ontValue(ontSummary.dyingGasp)}
                    total={ontSummary.total}
                    hint="Power lost"
                  />
                </Col>
              </Row>
            )}
          </DarkCard>
        </Col>

        <Col xs={24} lg={8}>
          <DarkCard title="Network Availability">
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
                strokeColor={availabilityColor}
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
                  : `${ontSummary.online} of ${ontSummary.total} ONTs online`}
              </div>
            </div>
          </DarkCard>
        </Col>
      </Row>
    </div>
  );
}
