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
    <Sider width={240} className="!bg-slate-900">
      <div className="h-16 flex items-center px-5 border-b border-slate-800">
        <span className="text-base font-semibold text-white">TikMan</span>
      </div>
      <Menu
        mode="inline"
        selectedKeys={[selectedKey]}
        items={items}
        className="!bg-transparent !border-none mt-2"
      />
    </Sider>
  );
}
