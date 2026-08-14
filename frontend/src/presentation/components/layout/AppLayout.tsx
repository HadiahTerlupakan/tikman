import { Layout } from 'antd';
import { Outlet } from 'react-router-dom';
import { Sidebar } from './Sidebar';
import { Header } from './Header';

const { Content } = Layout;

export function AppLayout() {
  return (
    <Layout className="min-h-screen" style={{ background: '#f5f5f5' }}>
      <Sidebar />
      <Layout>
        <Header />
        <Content className="p-6">
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  );
}
