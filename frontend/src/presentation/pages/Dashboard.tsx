import { Row, Col, Typography, Card, Statistic, Progress } from 'antd';
import { UserOutlined, EnvironmentOutlined, ApiOutlined, CheckCircleOutlined } from '@ant-design/icons';
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
  const oltHealthPercentage = olts?.length ? Math.round((activeOlts / olts.length) * 100) : 0;

  return (
    <div className="max-w-7xl space-y-6">
      {/* Welcome Section */}
      <div className="bg-gradient-to-r from-sky-500 to-sky-600 rounded-xl p-6 text-white shadow-lg">
        <div className="flex items-center justify-between">
          <div>
            <Title level={2} className="!text-white !mb-2">
              Selamat Datang, {user?.username}!
            </Title>
            <Text className="text-sky-50 text-base">
              {user?.role === UserRole.ADMIN && 'Administrator Dashboard - Kelola sistem OLT provisioning'}
              {user?.role === UserRole.TECHNICIAN && 'Technician Dashboard - Monitor dan kelola perangkat OLT'}
              {user?.role === UserRole.VIEWER && 'Viewer Dashboard - Lihat status sistem dan perangkat'}
            </Text>
          </div>
          <div className="hidden md:block">
            <CheckCircleOutlined className="text-6xl text-white/20" />
          </div>
        </div>
      </div>

      {/* Stats Cards */}
      <Row gutter={[16, 16]}>
        {user?.role === UserRole.ADMIN && (
          <Col xs={24} sm={12} lg={6}>
            <Card loading={usersLoading} className="hover:shadow-lg transition-shadow">
              <Statistic
                title={<span className="text-slate-600 font-medium">Total Pengguna</span>}
                value={users?.length || 0}
                prefix={<UserOutlined className="text-sky-500" />}
                valueStyle={{ color: '#0F172A', fontWeight: 600 }}
              />
            </Card>
          </Col>
        )}
        <Col xs={24} sm={12} lg={user?.role === UserRole.ADMIN ? 6 : 8}>
          <Card loading={sitesLoading} className="hover:shadow-lg transition-shadow">
            <Statistic
              title={<span className="text-slate-600 font-medium">Total Site</span>}
              value={sites?.length || 0}
              prefix={<EnvironmentOutlined className="text-emerald-500" />}
              valueStyle={{ color: '#0F172A', fontWeight: 600 }}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={user?.role === UserRole.ADMIN ? 6 : 8}>
          <Card loading={oltsLoading} className="hover:shadow-lg transition-shadow">
            <Statistic
              title={<span className="text-slate-600 font-medium">Total OLT</span>}
              value={olts?.length || 0}
              prefix={<ApiOutlined className="text-blue-500" />}
              valueStyle={{ color: '#0F172A', fontWeight: 600 }}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={user?.role === UserRole.ADMIN ? 6 : 8}>
          <Card loading={oltsLoading} className="hover:shadow-lg transition-shadow">
            <Statistic
              title={<span className="text-slate-600 font-medium">OLT Aktif</span>}
              value={activeOlts}
              suffix={`/ ${olts?.length || 0}`}
              prefix={<CheckCircleOutlined className="text-green-500" />}
              valueStyle={{ color: '#0F172A', fontWeight: 600 }}
            />
            <Progress
              percent={oltHealthPercentage}
              strokeColor={oltHealthPercentage >= 80 ? '#10B981' : oltHealthPercentage >= 50 ? '#F59E0B' : '#EF4444'}
              showInfo={false}
              className="mt-2"
            />
          </Card>
        </Col>
      </Row>

      {/* System Status & Quick Info */}
      <Row gutter={[16, 16]}>
        <Col xs={24} lg={12}>
          <Card
            title={<span className="text-slate-800 font-semibold">Status Sistem</span>}
            className="h-full"
          >
            <div className="space-y-4">
              <div className="flex items-center justify-between p-3 bg-green-50 rounded-lg border border-green-200">
                <div className="flex items-center gap-3">
                  <div className="w-2 h-2 bg-green-500 rounded-full animate-pulse"></div>
                  <Text className="font-medium">Backend API</Text>
                </div>
                <Text className="text-green-600 font-semibold">Online</Text>
              </div>
              <div className="flex items-center justify-between p-3 bg-green-50 rounded-lg border border-green-200">
                <div className="flex items-center gap-3">
                  <div className="w-2 h-2 bg-green-500 rounded-full animate-pulse"></div>
                  <Text className="font-medium">Database</Text>
                </div>
                <Text className="text-green-600 font-semibold">Connected</Text>
              </div>
              <div className="flex items-center justify-between p-3 bg-green-50 rounded-lg border border-green-200">
                <div className="flex items-center gap-3">
                  <div className="w-2 h-2 bg-green-500 rounded-full animate-pulse"></div>
                  <Text className="font-medium">Redis Cache</Text>
                </div>
                <Text className="text-green-600 font-semibold">Active</Text>
              </div>
            </div>
          </Card>
        </Col>
        <Col xs={24} lg={12}>
          <Card
            title={<span className="text-slate-800 font-semibold">Ringkasan OLT</span>}
            className="h-full"
          >
            <div className="space-y-4">
              <div className="flex items-center justify-between p-3 bg-slate-50 rounded-lg">
                <Text className="text-slate-600">Status Aktif</Text>
                <Text className="font-semibold text-green-600">{activeOlts} OLT</Text>
              </div>
              <div className="flex items-center justify-between p-3 bg-slate-50 rounded-lg">
                <Text className="text-slate-600">Status Tidak Aktif</Text>
                <Text className="font-semibold text-slate-600">
                  {(olts?.length || 0) - activeOlts} OLT
                </Text>
              </div>
              <div className="flex items-center justify-between p-3 bg-slate-50 rounded-lg">
                <Text className="text-slate-600">Health Percentage</Text>
                <Text className="font-semibold text-sky-600">{oltHealthPercentage}%</Text>
              </div>
            </div>
          </Card>
        </Col>
      </Row>
    </div>
  );
}
