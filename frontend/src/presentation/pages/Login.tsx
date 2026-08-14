import { useEffect } from "react";
import { useNavigate } from "react-router-dom";
import { Form, Input, Button, Typography, Alert, Card } from "antd";
import { UserOutlined, LockOutlined } from "@ant-design/icons";
import { useLogin } from "@/application/hooks";
import { useAuthStore } from "@/application/stores";
import type { LoginCredentials } from "@/domain/repositories";

const { Title, Text } = Typography;

export default function LoginPage() {
  const navigate = useNavigate();
  const isAuthenticated = useAuthStore((state) => state.isAuthenticated);
  const loginMutation = useLogin();

  useEffect(() => {
    if (isAuthenticated) {
      navigate("/", { replace: true });
    }
  }, [isAuthenticated, navigate]);

  const handleSubmit = (values: LoginCredentials) => {
    loginMutation.mutate(values, {
      onSuccess: () => {
        navigate("/", { replace: true });
      },
    });
  };

  return (
    <div
      style={{
        minHeight: "100vh",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        background: "#0a0a0a",
        padding: "24px",
      }}
    >
      <Card
        style={{
          width: "100%",
          maxWidth: "420px",
          background: "#18181b",
          border: "1px solid #27272a",
        }}
        bordered={false}
      >
        <div style={{ textAlign: "center", marginBottom: 32 }}>
          <div
            style={{
              width: 48,
              height: 48,
              background: "linear-gradient(135deg, #3ecf8e 0%, #2fb574 100%)",
              borderRadius: 12,
              display: "inline-flex",
              alignItems: "center",
              justifyContent: "center",
              marginBottom: 16,
            }}
          >
            <svg
              style={{ width: 28, height: 28, color: "white" }}
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              strokeWidth={2.5}
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d="M13 10V3L4 14h7v7l9-11h-7z"
              />
            </svg>
          </div>
          <Title level={3} style={{ margin: 0, color: "#ffffff" }}>
            TikMan
          </Title>
          <Text style={{ color: "#a1a1aa" }}>ZTE OLT Provisioning System</Text>
        </div>

        {loginMutation.isError && (
          <Alert
            message="Login Failed"
            description="Invalid username or password. Please try again."
            type="error"
            showIcon
            closable
            style={{ marginBottom: 24 }}
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
            label={<span style={{ color: "#e5e5e5" }}>Username</span>}
            rules={[{ required: true, message: "Please enter username" }]}
          >
            <Input
              prefix={<UserOutlined style={{ color: "#a1a1aa" }} />}
              placeholder="Enter username"
              size="large"
            />
          </Form.Item>

          <Form.Item
            name="password"
            label={<span style={{ color: "#e5e5e5" }}>Password</span>}
            rules={[{ required: true, message: "Please enter password" }]}
          >
            <Input.Password
              prefix={<LockOutlined style={{ color: "#a1a1aa" }} />}
              placeholder="Enter password"
              size="large"
            />
          </Form.Item>

          <Form.Item style={{ marginBottom: 0 }}>
            <Button
              type="primary"
              htmlType="submit"
              size="large"
              block
              loading={loginMutation.isPending}
            >
              {loginMutation.isPending ? "Signing in..." : "Sign In"}
            </Button>
          </Form.Item>
        </Form>

        <div
          style={{
            marginTop: 24,
            paddingTop: 24,
            borderTop: "1px solid #27272a",
            textAlign: "center",
          }}
        >
          <Text style={{ fontSize: 12, color: "#71717a" }}>
            Need help? Contact your system administrator
          </Text>
        </div>
      </Card>
    </div>
  );
}
