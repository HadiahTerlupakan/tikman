import { Layout, Menu } from 'antd';
import { useNavigate, useLocation } from 'react-router-dom';
import {
  DashboardOutlined,
  EnvironmentOutlined,
  ApiOutlined,
  UserOutlined,
} from '@ant-design/icons';
import { useAuthStore } from '@/application/stores';
import { UserRole } from '@/domain/entities';
import type { MenuProps } from 'antd';

const { Sider } = Layout;

export function Sidebar() {
  const navigate = useNavigate();
  const location = useLocation();
  const user = useAuthStore((state) => state.user);

  const items: MenuProps['items'] = [
    {
      key: '/',
      icon: <DashboardOutlined />,
      label: 'Dashboard',
      onClick: () => navigate('/'),
    },
    {
      key: '/sites',
      icon: <EnvironmentOutlined />,
      label: 'Sites',
      onClick: () => navigate('/sites'),
    },
    {
      key: '/olts',
      icon: <ApiOutlined />,
      label: 'OLTs',
      onClick: () => navigate('/olts'),
    },
  ];

  if (user?.role === UserRole.ADMIN) {
    items.push({
      key: '/users',
      icon: <UserOutlined />,
      label: 'Users',
      onClick: () => navigate('/users'),
    });
  }

  const selectedKey = items.find((item) => location.pathname === item?.key)?.key as string || '/';

  return (
    <Sider
      width={240}
      style={{
        overflow: 'auto',
        height: '100vh',
        position: 'fixed',
        left: 0,
        top: 0,
        bottom: 0,
        background: '#001529',
      }}
    >
      <div style={{ height: 64, display: 'flex', alignItems: 'center', padding: '0 24px', borderBottom: '1px solid rgba(255,255,255,0.1)' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          <div style={{ width: 32, height: 32, background: 'linear-gradient(135deg, #1890ff 0%, #096dd9 100%)', borderRadius: 8, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <svg style={{ width: 20, height: 20, color: 'white' }} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M13 10V3L4 14h7v7l9-11h-7z" />
            </svg>
          </div>
          <div style={{ display: 'flex', flexDirection: 'column' }}>
            <span style={{ fontSize: 14, fontWeight: 600, color: 'white' }}>TikMan</span>
            <span style={{ fontSize: 11, color: 'rgba(255,255,255,0.5)' }}>OLT Provisioning</span>
          </div>
        </div>
      </div>
      <Menu
        theme="dark"
        mode="inline"
        selectedKeys={[selectedKey]}
        items={items}
        style={{ marginTop: 16, border: 'none' }}
      />
    </Sider>
  );
}
