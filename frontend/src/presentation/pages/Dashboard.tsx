import { Row, Col, Typography, Card, Statistic } from 'antd';
import { UserOutlined, EnvironmentOutlined, ApiOutlined, ArrowUpOutlined } from '@ant-design/icons';
import { useUsers, useSites, useOlts } from '@/application/hooks';
import { useAuthStore } from '@/application/stores';
import { UserRole, OltStatus } from '@/domain/entities';

const { Title, Text } = Typography;

export default function DashboardPage() {
  const user = useAuthStore((state) => state.user);
  const { data: users, isLoading: usersLoading } = useUsers();
  const { data: sites, isLoading: sitesLoading } = useSites();
  const { data: olts, isLoading: oltsLoading } = useOlts();

  const activeOlts = olts?.filter(olt => olt.status === OltStatus.ONLINE).length || 0;
  const offlineOlts = olts?.filter(olt => olt.status === OltStatus.OFFLINE).length || 0;
  const errorOlts = olts?.filter(olt => olt.status === OltStatus.ERROR).length || 0;

  return (
    <div>
      {/* Page Header */}
      <div style={{ marginBottom: 24 }}>
        <Title level={4} style={{ margin: 0, marginBottom: 8 }}>Dashboard Overview</Title>
        <Text type="secondary">Monitor your OLT provisioning system in real-time</Text>
      </div>

      {/* Stats Cards */}
      <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
        {user?.role === UserRole.ADMIN && (
          <Col xs={24} sm={12} xl={6}>
            <Card bordered={false}>
              <Statistic
                title="Total Users"
                value={users?.length || 0}
                loading={usersLoading}
                prefix={<UserOutlined />}
                valueStyle={{ color: '#1890ff' }}
              />
            </Card>
          </Col>
        )}
        <Col xs={24} sm={12} xl={user?.role === UserRole.ADMIN ? 6 : 8}>
          <Card bordered={false}>
            <Statistic
              title="Total Sites"
              value={sites?.length || 0}
              loading={sitesLoading}
              prefix={<EnvironmentOutlined />}
              valueStyle={{ color: '#52c41a' }}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} xl={user?.role === UserRole.ADMIN ? 6 : 8}>
          <Card bordered={false}>
            <Statistic
              title="Total OLTs"
              value={olts?.length || 0}
              loading={oltsLoading}
              prefix={<ApiOutlined />}
              valueStyle={{ color: '#722ed1' }}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} xl={user?.role === UserRole.ADMIN ? 6 : 8}>
          <Card bordered={false}>
            <Statistic
              title="Online OLTs"
              value={activeOlts}
              loading={oltsLoading}
              suffix={`/ ${olts?.length || 0}`}
              valueStyle={{ color: '#52c41a' }}
              prefix={<ArrowUpOutlined />}
            />
            <Text type="secondary" style={{ fontSize: 12, marginTop: 8, display: 'block' }}>
              {olts?.length ? `${Math.round((activeOlts / olts.length) * 100)}% uptime` : 'No data'}
            </Text>
          </Card>
        </Col>
      </Row>

      {/* OLT Status & System Health */}
      <Row gutter={[16, 16]}>
        <Col xs={24} lg={16}>
          <Card
            title="OLT Status Distribution"
            bordered={false}
          >
            <Row gutter={[16, 16]}>
              <Col xs={24} sm={8}>
                <Card style={{ textAlign: 'center', backgroundColor: '#f6ffed', border: '1px solid #b7eb8f' }}>
                  <Statistic
                    value={activeOlts}
                    valueStyle={{ color: '#52c41a', fontSize: 32 }}
                  />
                  <Text style={{ color: '#52c41a', fontWeight: 500 }}>Online</Text>
                  <div style={{ fontSize: 12, color: '#52c41a', marginTop: 4 }}>Operating normally</div>
                </Card>
              </Col>
              <Col xs={24} sm={8}>
                <Card style={{ textAlign: 'center', backgroundColor: '#fafafa', border: '1px solid #d9d9d9' }}>
                  <Statistic
                    value={offlineOlts}
                    valueStyle={{ color: '#8c8c8c', fontSize: 32 }}
                  />
                  <Text style={{ color: '#8c8c8c', fontWeight: 500 }}>Offline</Text>
                  <div style={{ fontSize: 12, color: '#8c8c8c', marginTop: 4 }}>Not responding</div>
                </Card>
              </Col>
              <Col xs={24} sm={8}>
                <Card style={{ textAlign: 'center', backgroundColor: '#fff1f0', border: '1px solid #ffccc7' }}>
                  <Statistic
                    value={errorOlts}
                    valueStyle={{ color: '#ff4d4f', fontSize: 32 }}
                  />
                  <Text style={{ color: '#ff4d4f', fontWeight: 500 }}>Error</Text>
                  <div style={{ fontSize: 12, color: '#ff4d4f', marginTop: 4 }}>Needs attention</div>
                </Card>
              </Col>
            </Row>
          </Card>
        </Col>

        <Col xs={24} lg={8}>
          <Card
            title="System Health"
            bordered={false}
          >
            <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
              <div style={{ padding: 12, backgroundColor: '#f6ffed', borderRadius: 8, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <div style={{ width: 8, height: 8, backgroundColor: '#52c41a', borderRadius: '50%' }}></div>
                  <Text>API Server</Text>
                </div>
                <Text style={{ color: '#52c41a', fontWeight: 500 }}>Healthy</Text>
              </div>
              <div style={{ padding: 12, backgroundColor: '#f6ffed', borderRadius: 8, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <div style={{ width: 8, height: 8, backgroundColor: '#52c41a', borderRadius: '50%' }}></div>
                  <Text>Database</Text>
                </div>
                <Text style={{ color: '#52c41a', fontWeight: 500 }}>Connected</Text>
              </div>
              <div style={{ padding: 12, backgroundColor: '#f6ffed', borderRadius: 8, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <div style={{ width: 8, height: 8, backgroundColor: '#52c41a', borderRadius: '50%' }}></div>
                  <Text>Redis Cache</Text>
                </div>
                <Text style={{ color: '#52c41a', fontWeight: 500 }}>Active</Text>
              </div>
            </div>
          </Card>
        </Col>
      </Row>
    </div>
  );
}
