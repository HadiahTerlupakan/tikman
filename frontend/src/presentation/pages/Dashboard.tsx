import { Row, Col, Typography, Card, Statistic, Tag } from 'antd';
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
    <div className="space-y-6">
      {/* Page Header */}
      <div>
        <Title level={3} className="!mb-2">Dashboard Overview</Title>
        <Text type="secondary">Monitor your OLT provisioning system in real-time</Text>
      </div>

      {/* Stats Cards */}
      <Row gutter={[16, 16]}>
        {user?.role === UserRole.ADMIN && (
          <Col xs={24} sm={12} xl={6}>
            <Card loading={usersLoading} bordered={false} className="shadow-sm hover:shadow-md transition-shadow">
              <Statistic
                title="Total Users"
                value={users?.length || 0}
                prefix={<UserOutlined />}
                valueStyle={{ color: '#1890ff' }}
              />
              <div className="mt-2">
                <Tag color="blue">Active</Tag>
              </div>
            </Card>
          </Col>
        )}
        <Col xs={24} sm={12} xl={user?.role === UserRole.ADMIN ? 6 : 8}>
          <Card loading={sitesLoading} bordered={false} className="shadow-sm hover:shadow-md transition-shadow">
            <Statistic
              title="Total Sites"
              value={sites?.length || 0}
              prefix={<EnvironmentOutlined />}
              valueStyle={{ color: '#52c41a' }}
            />
            <div className="mt-2">
              <Text type="secondary" className="text-xs">Locations monitored</Text>
            </div>
          </Card>
        </Col>
        <Col xs={24} sm={12} xl={user?.role === UserRole.ADMIN ? 6 : 8}>
          <Card loading={oltsLoading} bordered={false} className="shadow-sm hover:shadow-md transition-shadow">
            <Statistic
              title="Total OLTs"
              value={olts?.length || 0}
              prefix={<ApiOutlined />}
              valueStyle={{ color: '#722ed1' }}
            />
            <div className="mt-2">
              <Text type="secondary" className="text-xs">Devices registered</Text>
            </div>
          </Card>
        </Col>
        <Col xs={24} sm={12} xl={user?.role === UserRole.ADMIN ? 6 : 8}>
          <Card loading={oltsLoading} bordered={false} className="shadow-sm hover:shadow-md transition-shadow">
            <Statistic
              title="Online OLTs"
              value={activeOlts}
              suffix={`/ ${olts?.length || 0}`}
              valueStyle={{ color: '#52c41a' }}
              prefix={<ArrowUpOutlined />}
            />
            <div className="mt-2">
              <Text type="secondary" className="text-xs">
                {olts?.length ? `${Math.round((activeOlts / olts.length) * 100)}% uptime` : 'No data'}
              </Text>
            </div>
          </Card>
        </Col>
      </Row>

      {/* OLT Status Details */}
      <Row gutter={[16, 16]}>
        <Col xs={24} lg={16}>
          <Card
            title="OLT Status Distribution"
            bordered={false}
            className="shadow-sm"
            extra={<Tag color="blue">Live</Tag>}
          >
            <Row gutter={[16, 16]}>
              <Col xs={24} sm={8}>
                <div className="text-center p-4 bg-green-50 rounded-lg border border-green-200">
                  <div className="text-3xl font-bold text-green-600">{activeOlts}</div>
                  <div className="text-sm text-green-700 mt-1 font-medium">Online</div>
                  <div className="text-xs text-green-600 mt-1">Operating normally</div>
                </div>
              </Col>
              <Col xs={24} sm={8}>
                <div className="text-center p-4 bg-gray-50 rounded-lg border border-gray-200">
                  <div className="text-3xl font-bold text-gray-600">{offlineOlts}</div>
                  <div className="text-sm text-gray-700 mt-1 font-medium">Offline</div>
                  <div className="text-xs text-gray-600 mt-1">Not responding</div>
                </div>
              </Col>
              <Col xs={24} sm={8}>
                <div className="text-center p-4 bg-red-50 rounded-lg border border-red-200">
                  <div className="text-3xl font-bold text-red-600">{errorOlts}</div>
                  <div className="text-sm text-red-700 mt-1 font-medium">Error</div>
                  <div className="text-xs text-red-600 mt-1">Needs attention</div>
                </div>
              </Col>
            </Row>
          </Card>
        </Col>
        <Col xs={24} lg={8}>
          <Card
            title="System Health"
            bordered={false}
            className="shadow-sm"
          >
            <div className="space-y-3">
              <div className="flex items-center justify-between p-3 bg-green-50 rounded-lg">
                <div className="flex items-center gap-2">
                  <div className="w-2 h-2 bg-green-500 rounded-full"></div>
                  <Text className="text-sm">API Server</Text>
                </div>
                <Tag color="success">Healthy</Tag>
              </div>
              <div className="flex items-center justify-between p-3 bg-green-50 rounded-lg">
                <div className="flex items-center gap-2">
                  <div className="w-2 h-2 bg-green-500 rounded-full"></div>
                  <Text className="text-sm">Database</Text>
                </div>
                <Tag color="success">Connected</Tag>
              </div>
              <div className="flex items-center justify-between p-3 bg-green-50 rounded-lg">
                <div className="flex items-center gap-2">
                  <div className="w-2 h-2 bg-green-500 rounded-full"></div>
                  <Text className="text-sm">Redis Cache</Text>
                </div>
                <Tag color="success">Active</Tag>
              </div>
            </div>
          </Card>
        </Col>
      </Row>

      {/* Quick Actions */}
      <Row gutter={[16, 16]}>
        <Col xs={24}>
          <Card
            title="Quick Actions"
            bordered={false}
            className="shadow-sm"
          >
            <Row gutter={[16, 16]}>
              <Col xs={24} sm={8}>
                <div className="p-4 border border-gray-200 rounded-lg hover:border-blue-400 hover:shadow-sm transition-all cursor-pointer">
                  <EnvironmentOutlined className="text-2xl text-blue-500 mb-2" />
                  <div className="font-medium text-gray-800">Manage Sites</div>
                  <div className="text-xs text-gray-500 mt-1">Add or configure site locations</div>
                </div>
              </Col>
              <Col xs={24} sm={8}>
                <div className="p-4 border border-gray-200 rounded-lg hover:border-blue-400 hover:shadow-sm transition-all cursor-pointer">
                  <ApiOutlined className="text-2xl text-purple-500 mb-2" />
                  <div className="font-medium text-gray-800">Manage OLTs</div>
                  <div className="text-xs text-gray-500 mt-1">Configure OLT devices</div>
                </div>
              </Col>
              {user?.role === UserRole.ADMIN && (
                <Col xs={24} sm={8}>
                  <div className="p-4 border border-gray-200 rounded-lg hover:border-blue-400 hover:shadow-sm transition-all cursor-pointer">
                    <UserOutlined className="text-2xl text-green-500 mb-2" />
                    <div className="font-medium text-gray-800">Manage Users</div>
                    <div className="text-xs text-gray-500 mt-1">Add or edit user accounts</div>
                  </div>
                </Col>
              )}
            </Row>
          </Card>
        </Col>
      </Row>
    </div>
  );
}
