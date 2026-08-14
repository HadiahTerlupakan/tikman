import { Layout, Dropdown, Avatar, Space, Typography } from 'antd';
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
      icon: (
        <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
        </svg>
      ),
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
      icon: (
        <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
        </svg>
      ),
      label: 'Logout',
      onClick: handleLogout,
      danger: true,
    },
  ];

  return (
    <AntHeader className="!bg-white !px-6 flex justify-between items-center border-b border-slate-200 shadow-sm">
      <div>
        <Typography.Title level={5} className="!mb-0 !text-slate-700 !font-semibold">
          ZTE OLT Provisioning
        </Typography.Title>
        <Text className="!text-xs !text-slate-500">Network Management System</Text>
      </div>
      <Dropdown menu={{ items }} placement="bottomRight">
        <Space className="cursor-pointer hover:bg-slate-50 px-3 py-2 rounded-lg transition-colors">
          <Avatar className="!bg-emerald-500" size={36}>
            {user?.username?.charAt(0).toUpperCase()}
          </Avatar>
          <div className="hidden sm:block">
            <Text className="!text-sm !font-medium !text-slate-700 block leading-tight">{user?.username}</Text>
            <Text className="!text-xs !text-slate-500 capitalize">{user?.role}</Text>
          </div>
          <svg className="w-4 h-4 text-slate-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
          </svg>
        </Space>
      </Dropdown>
    </AntHeader>
  );
}
