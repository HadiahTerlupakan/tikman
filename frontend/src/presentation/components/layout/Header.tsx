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
    <AntHeader className="!bg-white !px-5 flex justify-between items-center border-b border-gray-200 shadow-sm" style={{ height: 56, lineHeight: '56px' }}>
      <div className="flex items-center gap-4">
        <Typography.Title level={5} className="!mb-0 !text-gray-800" style={{ fontSize: 15, fontWeight: 600 }}>
          ZTE OLT Provisioning
        </Typography.Title>
      </div>
      <Dropdown menu={{ items }} placement="bottomRight">
        <Space className="cursor-pointer hover:bg-gray-50 px-2 py-1 rounded-md transition-colors">
          <Avatar className="!bg-emerald-500" size={32}>
            {user?.username?.charAt(0).toUpperCase()}
          </Avatar>
          <div className="hidden sm:block">
            <Text className="!text-sm !text-gray-700">{user?.username}</Text>
          </div>
        </Space>
      </Dropdown>
    </AntHeader>
  );
}
