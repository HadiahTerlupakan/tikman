import { Layout } from 'antd';
import { Outlet } from 'react-router-dom';
import { Sidebar } from './Sidebar';
import { Header } from './Header';

const { Content } = Layout;

export function AppLayout() {
  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sidebar />
      <Layout>
        <Header />
        <Content style={{ margin: '24px', background: '#fff', padding: 24 }}>
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  );
}
