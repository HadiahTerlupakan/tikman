import { Layout, Dropdown, Avatar, Space, Typography } from 'antd';
import { UserOutlined, LogoutOutlined } from '@ant-design/icons';
import { useAuthStore } from '@/application/stores';
import { useLogout } from '@/application/hooks';
import type { MenuProps } from 'antd';

const { Header: AntHeader } = Layout;
const { Text } = Typography;

export function Header() {
  const user = useAuthStore((state) => state.user);
  const logoutMutation = useLogout();

  const handleLogout = () => {
    logoutMutation.mutate();
  };

  const items: MenuProps['items'] = [
    {
      key: 'profile',
      icon: <UserOutlined />,
      label: (
        <div>
          <Text strong>{user?.username}</Text>
          <br />
          <Text type="secondary" style={{ fontSize: '12px' }}>
            {user?.role}
          </Text>
        </div>
      ),
    },
    { type: 'divider' },
    {
      key: 'logout',
      icon: <LogoutOutlined />,
      label: 'Logout',
      onClick: handleLogout,
    },
  ];

  return (
    <AntHeader style={{ background: '#fff', padding: '0 24px', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
      <Typography.Title level={4} style={{ margin: 0 }}>
        TikMan OLT Provisioning
      </Typography.Title>
      <Dropdown menu={{ items }} placement="bottomRight">
        <Space style={{ cursor: 'pointer' }}>
          <Avatar icon={<UserOutlined />} />
          <Text>{user?.username}</Text>
        </Space>
      </Dropdown>
    </AntHeader>
  );
}
