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
    <AntHeader className="!bg-white !px-6 flex justify-between items-center shadow-sm" style={{ height: 64, lineHeight: '64px', borderBottom: '1px solid #e5e7eb' }}>
      <div className="flex items-center gap-4">
        <Typography.Title level={4} className="!mb-0 !text-slate-800" style={{ fontSize: 16, fontWeight: 600 }}>
          ZTE OLT Provisioning System
        </Typography.Title>
      </div>
      <Dropdown menu={{ items }} placement="bottomRight">
        <Space className="cursor-pointer hover:bg-slate-50 px-3 py-2 rounded-lg transition-colors">
          <Avatar className="!bg-sky-500" size={36}>
            {user?.username?.charAt(0).toUpperCase()}
          </Avatar>
          <div className="hidden sm:block">
            <div className="flex flex-col items-start">
              <Text className="!text-sm !text-slate-800 font-medium">{user?.username}</Text>
              <Text className="!text-xs !text-slate-500 capitalize">{user?.role}</Text>
            </div>
          </div>
        </Space>
      </Dropdown>
    </AntHeader>
  );
}
