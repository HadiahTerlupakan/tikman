import { Row, Col, Typography } from 'antd';
import { UserOutlined, EnvironmentOutlined, ApiOutlined } from '@ant-design/icons';
import { useUsers, useSites, useOlts } from '@/application/hooks';
import { StatsCard } from '../components/dashboard';
import { useAuthStore } from '@/application/stores';
import { UserRole } from '@/domain/entities';

const { Title } = Typography;

export default function DashboardPage() {
  const user = useAuthStore((state) => state.user);
  const { data: users, isLoading: usersLoading } = useUsers();
  const { data: sites, isLoading: sitesLoading } = useSites();
  const { data: olts, isLoading: oltsLoading } = useOlts();

  return (
    <div>
      <Title level={2}>Dashboard</Title>
      <Typography.Paragraph type="secondary">
        Selamat datang, {user?.username}!
      </Typography.Paragraph>

      <Row gutter={[16, 16]} style={{ marginTop: 24 }}>
        {user?.role === UserRole.ADMIN && (
          <Col xs={24} sm={12} lg={8}>
            <StatsCard
              title="Total Pengguna"
              value={users?.length || 0}
              icon={<UserOutlined />}
              loading={usersLoading}
            />
          </Col>
        )}
        <Col xs={24} sm={12} lg={8}>
          <StatsCard
            title="Total Site"
            value={sites?.length || 0}
            icon={<EnvironmentOutlined />}
            loading={sitesLoading}
          />
        </Col>
        <Col xs={24} sm={12} lg={8}>
          <StatsCard
            title="Total OLT"
            value={olts?.length || 0}
            icon={<ApiOutlined />}
            loading={oltsLoading}
          />
        </Col>
      </Row>
    </div>
  );
}
