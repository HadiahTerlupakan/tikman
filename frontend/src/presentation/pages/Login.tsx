import { useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { Form, Input, Button, Typography, Alert } from 'antd';
import { UserOutlined, LockOutlined } from '@ant-design/icons';
import { useLogin } from '@/application/hooks';
import { useAuthStore } from '@/application/stores';
import type { LoginCredentials } from '@/domain/repositories';

const { Title, Text } = Typography;

export default function LoginPage() {
  const navigate = useNavigate();
  const isAuthenticated = useAuthStore((state) => state.isAuthenticated);
  const loginMutation = useLogin();

  useEffect(() => {
    if (isAuthenticated) {
      navigate('/', { replace: true });
    }
  }, [isAuthenticated, navigate]);

  const handleSubmit = (values: LoginCredentials) => {
    loginMutation.mutate(values, {
      onSuccess: () => {
        navigate('/', { replace: true });
      },
    });
  };

  return (
    <div className="min-h-[100dvh] flex">
      {/* Left Panel - Brand */}
      <div className="hidden lg:flex lg:w-1/2 bg-gradient-to-br from-slate-900 via-slate-800 to-slate-900 relative overflow-hidden">
        {/* Subtle grid pattern */}
        <div className="absolute inset-0 opacity-[0.03]" style={{
          backgroundImage: `
            linear-gradient(rgba(255,255,255,0.1) 1px, transparent 1px),
            linear-gradient(90deg, rgba(255,255,255,0.1) 1px, transparent 1px)
          `,
          backgroundSize: '48px 48px'
        }} />

        {/* Gradient orb */}
        <div className="absolute top-1/4 -left-32 w-96 h-96 bg-emerald-500/20 rounded-full blur-3xl" />
        <div className="absolute bottom-1/4 -right-32 w-96 h-96 bg-blue-500/20 rounded-full blur-3xl" />

        <div className="relative z-10 flex flex-col justify-between p-12 w-full">
          <div>
            <div className="flex items-center gap-3 mb-12">
              <div className="w-10 h-10 bg-emerald-500 rounded-lg flex items-center justify-center">
                <svg className="w-6 h-6 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 10V3L4 14h7v7l9-11h-7z" />
                </svg>
              </div>
              <span className="text-2xl font-semibold text-white tracking-tight">TikMan</span>
            </div>

            <div className="max-w-md">
              <h1 className="text-4xl font-bold text-white mb-4 leading-tight">
                Network provisioning made simple
              </h1>
              <p className="text-slate-300 text-lg leading-relaxed">
                Manage ZTE OLT devices, configure subscribers, and monitor your fiber network from one unified platform.
              </p>
            </div>
          </div>

          <div className="max-w-md">
            <div className="flex items-start gap-3 p-4 bg-white/5 backdrop-blur-sm border border-white/10 rounded-lg">
              <div className="w-8 h-8 bg-emerald-500/20 rounded-full flex items-center justify-center flex-shrink-0 mt-0.5">
                <svg className="w-4 h-4 text-emerald-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                </svg>
              </div>
              <div>
                <p className="text-sm font-medium text-white mb-1">Role-based access control</p>
                <p className="text-sm text-slate-400">Admin, Technician, and Viewer roles with granular permissions</p>
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Right Panel - Form */}
      <div className="flex-1 flex items-center justify-center p-6 lg:p-12 bg-white">
        <div className="w-full max-w-md">
          <div className="mb-8 lg:hidden">
            <div className="flex items-center gap-2 mb-6">
              <div className="w-8 h-8 bg-emerald-500 rounded-lg flex items-center justify-center">
                <svg className="w-5 h-5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 10V3L4 14h7v7l9-11h-7z" />
                </svg>
              </div>
              <span className="text-xl font-semibold text-slate-900 tracking-tight">TikMan</span>
            </div>
          </div>

          <div className="mb-8">
            <Title level={2} className="!mb-2 !text-slate-900">Masuk ke Sistem</Title>
            <Text className="text-slate-600">Gunakan kredensial akun Anda untuk melanjutkan</Text>
          </div>

          {loginMutation.isError && (
            <Alert
              message="Login Gagal"
              description="Username atau password salah. Silakan coba lagi."
              type="error"
              showIcon
              closable
              className="mb-6"
            />
          )}

          <Form
            name="login"
            onFinish={handleSubmit}
            autoComplete="off"
            layout="vertical"
            requiredMark={false}
          >
            <Form.Item
              name="username"
              label={<span className="text-sm font-medium text-slate-700">Username</span>}
              rules={[{ required: true, message: 'Username harus diisi' }]}
            >
              <Input
                prefix={<UserOutlined className="text-slate-400" />}
                placeholder="Masukkan username"
                size="large"
                className="rounded-lg"
              />
            </Form.Item>

            <Form.Item
              name="password"
              label={<span className="text-sm font-medium text-slate-700">Password</span>}
              rules={[{ required: true, message: 'Password harus diisi' }]}
            >
              <Input.Password
                prefix={<LockOutlined className="text-slate-400" />}
                placeholder="Masukkan password"
                size="large"
                className="rounded-lg"
              />
            </Form.Item>

            <Form.Item className="mb-0">
              <Button
                type="primary"
                htmlType="submit"
                size="large"
                block
                loading={loginMutation.isPending}
                className="!bg-emerald-600 hover:!bg-emerald-700 !border-emerald-600 hover:!border-emerald-700 !h-12 !rounded-lg !font-medium !shadow-sm"
              >
                {loginMutation.isPending ? 'Memproses...' : 'Masuk'}
              </Button>
            </Form.Item>
          </Form>

          <div className="mt-8 pt-8 border-t border-slate-200">
            <p className="text-sm text-slate-500 text-center">
              Butuh bantuan? Hubungi administrator sistem Anda
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}
