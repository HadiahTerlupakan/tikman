import { Layout, Dropdown, Avatar, Space, Typography, Badge } from 'antd';
import { UserOutlined, LogoutOutlined, BellOutlined } from '@ant-design/icons';
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
          <Text strong className="!text-gray-900">{user?.username}</Text>
          <br />
          <Text className="!text-xs !text-gray-500 capitalize">
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
      danger: true,
    },
  ];

  return (
    <AntHeader style={{
      height: 64,
      padding: '0 24px',
      background: '#fff',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      borderBottom: '1px solid #f0f0f0',
      position: 'fixed',
      top: 0,
      right: 0,
      left: 240,
      zIndex: 1
    }}>
      <div>
        <Text style={{ fontSize: 14, color: '#8c8c8c' }}>
          Welcome back, <span style={{ color: '#262626', fontWeight: 500 }}>{user?.username}</span>
        </Text>
      </div>
      <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
        <Badge count={0} showZero={false}>
          <div style={{ width: 36, height: 36, display: 'flex', alignItems: 'center', justifyContent: 'center', borderRadius: 8, cursor: 'pointer', transition: 'background 0.2s' }}
               onMouseEnter={(e) => e.currentTarget.style.background = '#f5f5f5'}
               onMouseLeave={(e) => e.currentTarget.style.background = 'transparent'}>
            <BellOutlined style={{ fontSize: 18, color: '#595959' }} />
          </div>
        </Badge>
        <Dropdown menu={{ items }} placement="bottomRight" trigger={['click']}>
          <Space style={{ cursor: 'pointer', padding: '8px 12px', borderRadius: 8, transition: 'background 0.2s' }}
                 onMouseEnter={(e) => e.currentTarget.style.background = '#f5f5f5'}
                 onMouseLeave={(e) => e.currentTarget.style.background = 'transparent'}>
            <Avatar style={{ background: '#1890ff' }} size={36}>
              {user?.username?.charAt(0).toUpperCase()}
            </Avatar>
            <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-start' }}>
              <Text style={{ fontSize: 14, color: '#262626', fontWeight: 500 }}>{user?.username}</Text>
              <Text style={{ fontSize: 12, color: '#8c8c8c', textTransform: 'capitalize' }}>{user?.role}</Text>
            </div>
          </Space>
        </Dropdown>
      </div>
    </AntHeader>
  );
}
