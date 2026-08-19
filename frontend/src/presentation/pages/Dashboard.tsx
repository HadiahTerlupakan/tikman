import { Row, Col, Card, Statistic } from "antd";
import {
  UserOutlined,
  EnvironmentOutlined,
  ApiOutlined,
  ArrowUpOutlined,
} from "@ant-design/icons";
import { useUsers, useSites, useOlts } from "@/application/hooks";
import { useAuthStore } from "@/application/stores";
import { UserRole, OltStatus } from "@/domain/entities";
import { PageHeader, DarkCard } from "../components/common";

export default function DashboardPage() {
  const user = useAuthStore((state) => state.user);
  const { data: users, isLoading: usersLoading } = useUsers();
  const { data: sites, isLoading: sitesLoading } = useSites();
  const { data: olts, isLoading: oltsLoading } = useOlts();

  const activeOlts =
    olts?.filter((olt) => olt.status === OltStatus.ONLINE).length || 0;
  const offlineOlts =
    olts?.filter((olt) => olt.status === OltStatus.OFFLINE).length || 0;
  const errorOlts =
    olts?.filter((olt) => olt.status === OltStatus.ERROR).length || 0;

  return (
    <div>
      <PageHeader
        title="Dashboard Overview"
        description="Monitor your OLT provisioning system in real-time"
      />

      {/* Stats Cards */}
      <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
        {user?.role === UserRole.ADMIN && (
          <Col xs={24} sm={12} xl={6}>
            <DarkCard>
              <Statistic
                title={<span style={{ color: "#a1a1aa" }}>Total Users</span>}
                value={users?.length || 0}
                loading={usersLoading}
                prefix={<UserOutlined />}
                valueStyle={{ color: "#3ecf8e" }}
              />
            </DarkCard>
          </Col>
        )}
        <Col xs={24} sm={12} xl={user?.role === UserRole.ADMIN ? 6 : 8}>
          <DarkCard>
            <Statistic
              title={<span style={{ color: "#a1a1aa" }}>Total Sites</span>}
              value={sites?.length || 0}
              loading={sitesLoading}
              prefix={<EnvironmentOutlined />}
              valueStyle={{ color: "#3ecf8e" }}
            />
          </DarkCard>
        </Col>
        <Col xs={24} sm={12} xl={user?.role === UserRole.ADMIN ? 6 : 8}>
          <DarkCard>
            <Statistic
              title={<span style={{ color: "#a1a1aa" }}>Total OLTs</span>}
              value={olts?.length || 0}
              loading={oltsLoading}
              prefix={<ApiOutlined />}
              valueStyle={{ color: "#3ecf8e" }}
            />
          </DarkCard>
        </Col>
        <Col xs={24} sm={12} xl={user?.role === UserRole.ADMIN ? 6 : 8}>
          <DarkCard>
            <Statistic
              title={<span style={{ color: "#a1a1aa" }}>Online OLTs</span>}
              value={activeOlts}
              loading={oltsLoading}
              suffix={
                <span style={{ color: "#71717a" }}>/ {olts?.length || 0}</span>
              }
              valueStyle={{ color: "#3ecf8e" }}
              prefix={<ArrowUpOutlined />}
            />
            <div style={{ fontSize: 12, marginTop: 8, color: "#71717a" }}>
              {olts?.length
                ? `${Math.round((activeOlts / olts.length) * 100)}% uptime`
                : "No data"}
            </div>
          </DarkCard>
        </Col>
      </Row>

      {/* OLT Status & System Health */}
      <Row gutter={[16, 16]}>
        <Col xs={24} lg={16}>
          <DarkCard title="OLT Status Distribution">
            <Row gutter={[16, 16]}>
              <Col xs={24} sm={8}>
                <Card
                  style={{
                    textAlign: "center",
                    background: "#14532d",
                    border: "1px solid #15803d",
                  }}
                >
                  <Statistic
                    value={activeOlts}
                    valueStyle={{ color: "#3ecf8e", fontSize: 32 }}
                  />
                  <div
                    style={{ color: "#3ecf8e", fontWeight: 500, marginTop: 8 }}
                  >
                    Online
                  </div>
                  <div style={{ fontSize: 12, color: "#4ade80", marginTop: 4 }}>
                    Operating normally
                  </div>
                </Card>
              </Col>
              <Col xs={24} sm={8}>
                <Card
                  style={{
                    textAlign: "center",
                    background: "#18181b",
                    border: "1px solid #27272a",
                  }}
                >
                  <Statistic
                    value={offlineOlts}
                    valueStyle={{ color: "#a1a1aa", fontSize: 32 }}
                  />
                  <div
                    style={{ color: "#a1a1aa", fontWeight: 500, marginTop: 8 }}
                  >
                    Offline
                  </div>
                  <div style={{ fontSize: 12, color: "#71717a", marginTop: 4 }}>
                    Not responding
                  </div>
                </Card>
              </Col>
              <Col xs={24} sm={8}>
                <Card
                  style={{
                    textAlign: "center",
                    background: "#450a0a",
                    border: "1px solid #dc2626",
                  }}
                >
                  <Statistic
                    value={errorOlts}
                    valueStyle={{ color: "#ef4444", fontSize: 32 }}
                  />
                  <div
                    style={{ color: "#ef4444", fontWeight: 500, marginTop: 8 }}
                  >
                    Error
                  </div>
                  <div style={{ fontSize: 12, color: "#f87171", marginTop: 4 }}>
                    Needs attention
                  </div>
                </Card>
              </Col>
            </Row>
          </DarkCard>
        </Col>

        <Col xs={24} lg={12}>
          <DarkCard title="System Health">
            <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
              {[
                { label: "API Server", status: "Healthy" },
                { label: "Database", status: "Connected" },
                { label: "Redis Cache", status: "Active" },
              ].map((item) => (
                <div
                  key={item.label}
                  style={{
                    padding: 12,
                    background: "#14532d",
                    border: "1px solid #15803d",
                    borderRadius: 8,
                    display: "flex",
                    justifyContent: "space-between",
                    alignItems: "center",
                  }}
                >
                  <div
                    style={{ display: "flex", alignItems: "center", gap: 8 }}
                  >
                    <div
                      style={{
                        width: 8,
                        height: 8,
                        backgroundColor: "#3ecf8e",
                        borderRadius: "50%",
                      }}
                    ></div>
                    <span style={{ color: "#e5e5e5" }}>{item.label}</span>
                  </div>
                  <span style={{ color: "#3ecf8e", fontWeight: 500 }}>
                    {item.status}
                  </span>
                </div>
              ))}
            </div>
          </DarkCard>
        </Col>
      </Row>
    </div>
  );
}
