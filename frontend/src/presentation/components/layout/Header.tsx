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
    <AntHeader className="!bg-white !px-6 flex justify-between items-center" style={{ height: 64, lineHeight: '64px', borderBottom: '1px solid #f0f0f0' }}>
      <div className="flex items-center gap-4">
        <Text className="text-gray-400 text-sm">
          Welcome back, <span className="text-gray-900 font-medium">{user?.username}</span>
        </Text>
      </div>
      <div className="flex items-center gap-4">
        <Badge count={0} showZero={false}>
          <div className="w-9 h-9 flex items-center justify-center rounded-lg hover:bg-gray-100 cursor-pointer transition-colors">
            <BellOutlined className="text-gray-600 text-lg" />
          </div>
        </Badge>
        <Dropdown menu={{ items }} placement="bottomRight" trigger={['click']}>
          <Space className="cursor-pointer hover:bg-gray-50 px-3 py-2 rounded-lg transition-colors">
            <Avatar className="!bg-blue-500" size={36}>
              {user?.username?.charAt(0).toUpperCase()}
            </Avatar>
            <div className="hidden sm:block">
              <div className="flex flex-col items-start">
                <Text className="!text-sm !text-gray-800 font-medium">{user?.username}</Text>
                <Text className="!text-xs !text-gray-500 capitalize">{user?.role}</Text>
              </div>
            </div>
          </Space>
        </Dropdown>
      </div>
    </AntHeader>
  );
}
