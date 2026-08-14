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
          <Text strong className="!text-slate-700">{user?.username}</Text>
          <br />
          <Text className="!text-xs !text-slate-500 capitalize">
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
    <AntHeader className="!bg-white !px-6 flex justify-between items-center border-b border-slate-200">
      <Typography.Title level={5} className="!mb-0 !text-slate-800">
        ZTE OLT Provisioning
      </Typography.Title>
      <Dropdown menu={{ items }} placement="bottomRight">
        <Space className="cursor-pointer">
          <Avatar className="!bg-emerald-500" size="default">
            {user?.username?.charAt(0).toUpperCase()}
          </Avatar>
          <Text className="!text-sm !text-slate-700">{user?.username}</Text>
        </Space>
      </Dropdown>
    </AntHeader>
  );
}
