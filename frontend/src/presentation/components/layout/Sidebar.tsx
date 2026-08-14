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
    <Sider width={240} style={{ background: 'linear-gradient(180deg, #0F172A 0%, #1E293B 100%)' }}>
      <div className="h-16 flex items-center px-6 border-b border-white/10">
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 bg-gradient-to-br from-sky-400 to-sky-600 rounded-lg flex items-center justify-center shadow-lg">
            <svg className="w-5 h-5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M13 10V3L4 14h7v7l9-11h-7z" />
            </svg>
          </div>
          <span className="text-base font-bold text-white tracking-tight">TikMan</span>
        </div>
      </div>
      <div className="p-3 mt-3">
        <Menu
          mode="inline"
          selectedKeys={[selectedKey]}
          items={items}
          className="!bg-transparent !border-none"
          style={{
            fontSize: '14px',
          }}
        />
      </div>
    </Sider>
  );
}
