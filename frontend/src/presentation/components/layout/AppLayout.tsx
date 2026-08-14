import { Outlet, useNavigate, useLocation } from 'react-router-dom';
import { ProLayout } from '@ant-design/pro-components';
import {
  DashboardOutlined,
  EnvironmentOutlined,
  ApiOutlined,
  UserOutlined,
  LogoutOutlined,
  BellOutlined,
} from '@ant-design/icons';
import { Dropdown, Avatar, Badge } from 'antd';
import type { MenuProps } from 'antd';
import { useAuthStore } from '@/application/stores';
import { useLogout } from '@/application/hooks';
import { UserRole } from '@/domain/entities';

export function AppLayout() {
  const navigate = useNavigate();
  const location = useLocation();
  const user = useAuthStore((state) => state.user);
  const logoutMutation = useLogout();

  const handleLogout = () => {
    logoutMutation.mutate();
  };

  const userMenuItems: MenuProps['items'] = [
    {
      key: 'profile',
      icon: <UserOutlined />,
      label: `${user?.username} (${user?.role})`,
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

  const routes = [
    {
      path: '/',
      name: 'Dashboard',
      icon: <DashboardOutlined />,
    },
    {
      path: '/sites',
      name: 'Sites',
      icon: <EnvironmentOutlined />,
    },
    {
      path: '/olts',
      name: 'OLTs',
      icon: <ApiOutlined />,
    },
    ...(user?.role === UserRole.ADMIN
      ? [
          {
            path: '/users',
            name: 'Users',
            icon: <UserOutlined />,
          },
        ]
      : []),
  ];

  return (
    <ProLayout
      title="TikMan"
      logo={
        <div style={{ width: 32, height: 32, background: 'linear-gradient(135deg, #1890ff 0%, #096dd9 100%)', borderRadius: 8, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
          <svg style={{ width: 20, height: 20, color: 'white' }} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M13 10V3L4 14h7v7l9-11h-7z" />
          </svg>
        </div>
      }
      layout="mix"
      splitMenus={false}
      navTheme="light"
      fixedHeader
      fixSiderbar
      location={location}
      route={{ routes }}
      menuItemRender={(item, dom) => (
        <div onClick={() => navigate(item.path || '/')}>{dom}</div>
      )}
      avatarProps={{
        src: undefined,
        size: 'default',
        title: user?.username,
        render: () => {
          return (
            <Dropdown menu={{ items: userMenuItems }} placement="bottomRight">
              <div style={{ display: 'flex', alignItems: 'center', gap: 16, cursor: 'pointer' }}>
                <Badge count={0}>
                  <BellOutlined style={{ fontSize: 18 }} />
                </Badge>
                <Avatar style={{ backgroundColor: '#1890ff' }}>
                  {user?.username?.charAt(0).toUpperCase()}
                </Avatar>
              </div>
            </Dropdown>
          );
        },
      }}
      actionsRender={() => []}
      menuFooterRender={() => (
        <div style={{ padding: '16px', borderTop: '1px solid #f0f0f0', fontSize: 12, color: '#8c8c8c' }}>
          OLT Provisioning System
        </div>
      )}
    >
      <div style={{ padding: 24, minHeight: 'calc(100vh - 64px)' }}>
        <Outlet />
      </div>
    </ProLayout>
  );
}
