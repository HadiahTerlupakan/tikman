import { Row, Col, Typography } from 'antd';
import { UserOutlined, EnvironmentOutlined, ApiOutlined } from '@ant-design/icons';
import { useUsers, useSites, useOlts } from '@/application/hooks';
import { StatsCard } from '../components/dashboard';
import { useAuthStore } from '@/application/stores';
import { UserRole } from '@/domain/entities';

const { Title, Text } = Typography;

export default function DashboardPage() {
  const user = useAuthStore((state) => state.user);
  const { data: users, isLoading: usersLoading } = useUsers();
  const { data: sites, isLoading: sitesLoading } = useSites();
  const { data: olts, isLoading: oltsLoading } = useOlts();

  return (
    <div className="max-w-7xl">
      <div className="mb-6">
        <Title level={3} className="!mb-1 !text-gray-900">Dashboard</Title>
        <Text className="text-gray-600">Selamat datang, {user?.username}!</Text>
      </div>

      <Row gutter={[16, 16]}>
        {user?.role === UserRole.ADMIN && (
          <Col xs={24} sm={12} lg={8}>
            <StatsCard
              title="Total Pengguna"
              value={users?.length || 0}
              icon={<UserOutlined style={{ fontSize: 20 }} />}
              loading={usersLoading}
            />
          </Col>
        )}
        <Col xs={24} sm={12} lg={8}>
          <StatsCard
            title="Total Site"
            value={sites?.length || 0}
            icon={<EnvironmentOutlined style={{ fontSize: 20 }} />}
            loading={sitesLoading}
          />
        </Col>
        <Col xs={24} sm={12} lg={8}>
          <StatsCard
            title="Total OLT"
            value={olts?.length || 0}
            icon={<ApiOutlined style={{ fontSize: 20 }} />}
            loading={oltsLoading}
          />
        </Col>
      </Row>
    </div>
  );
}
