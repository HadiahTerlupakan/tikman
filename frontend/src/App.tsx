import { RouterProvider } from 'react-router-dom';
import { QueryClientProvider } from '@tanstack/react-query';
import { ConfigProvider } from 'antd';
import enUS from 'antd/locale/en_US';
import { queryClient } from '@/shared/config/queryClient';
import { theme } from '@/shared/theme';
import { router } from './presentation/routes';
import '@/shared/styles/background-pattern.css';

export default function App() {
  return (
    <ConfigProvider theme={theme} locale={enUS}>
      <QueryClientProvider client={queryClient}>
        <div className="app-background-pattern" />
        <div className="app-content-wrapper">
          <RouterProvider router={router} />
        </div>
      </QueryClientProvider>
    </ConfigProvider>
  );
}
