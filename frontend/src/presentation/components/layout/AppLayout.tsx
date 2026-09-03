import { Outlet, useNavigate, useLocation } from "react-router-dom";
import { ProLayout } from "@ant-design/pro-components";
import { UserOutlined, LogoutOutlined, BellOutlined } from "@ant-design/icons";
import { Dropdown, Avatar, Badge, App, Grid } from "antd";
import type { MenuProps } from "antd";
import { useEffect } from "react";
import { useAuthStore } from "@/application/stores";
import { useLogout, useCsConversations } from "@/application/hooks";
import {
  useCsStream,
  type WaStreamStatus,
} from "@/application/hooks/useCsStream";
import { UserRole } from "@/domain/entities";
import {
  listenForForegroundMessages,
  refreshTokenIfGranted,
} from "@/infrastructure/firebase/messaging";
import { PushRepository } from "@/infrastructure/repositories";
import { buildNavigationRoutes } from "./navigationRoutes";
import { layoutPadding } from "./layoutPadding";

const pushRepository = new PushRepository();

// The three roles that can open /api/v1/cs/* at all — everything push- and
// badge-related is inert for anyone else, the same gate the backend enforces.
const CS_ROLES: UserRole[] = [UserRole.ADMIN, UserRole.CS, UserRole.TECHNICIAN];

/** What CsInboxPage reads back via useOutletContext, since the stream that
 * feeds the navbar badge has to run here, not on that page. */
export interface AppLayoutContext {
  csStream: WaStreamStatus;
}

export function AppLayout() {
  const navigate = useNavigate();
  const location = useLocation();
  const user = useAuthStore((state) => state.user);
  const logoutMutation = useLogout();
  const canUseCs = !!user && CS_ROLES.includes(user.role);

  const stream = useCsStream(canUseCs);
  const awaitingQuery = useCsConversations(
    { awaitingReply: true },
    { enabled: canUseCs },
  );
  const awaitingCount = awaitingQuery.data?.length ?? 0;

  // Runs once per app-shell mount, not per click: a CS who already granted
  // permission on a previous visit gets their token silently refreshed, and
  // the foreground listener has to be live on every page — a push can arrive
  // while looking at the OLT map, not only while CS Inbox is open.
  useEffect(() => {
    if (!canUseCs) return;
    // A cleanup can outrun the promise that produces the unsubscribe (e.g.
    // StrictMode's synchronous mount→cleanup→re-mount in dev, or canUseCs
    // flipping faster than Firebase resolves) — without this flag, the
    // listener that arrives after cleanup would never be torn down.
    let cancelled = false;
    let unsubscribe: (() => void) | undefined;

    refreshTokenIfGranted().then((token) => {
      if (token) void pushRepository.subscribe(token);
    });

    listenForForegroundMessages((title, body) => {
      new Notification(title, { body });
    }).then((unsub) => {
      if (cancelled) {
        unsub();
        return;
      }
      unsubscribe = unsub;
    });

    return () => {
      cancelled = true;
      unsubscribe?.();
    };
  }, [canUseCs]);

  const handleLogout = () => {
    logoutMutation.mutate();
  };

  const userMenuItems: MenuProps["items"] = [
    {
      key: "profile",
      icon: <UserOutlined />,
      label: `${user?.username} (${user?.role})`,
    },
    { type: "divider" },
    {
      key: "logout",
      icon: <LogoutOutlined />,
      label: "Logout",
      onClick: handleLogout,
      danger: true,
    },
  ];

  const routes = buildNavigationRoutes(user?.role);
  const padding = layoutPadding(Grid.useBreakpoint());

  return (
    <div
      style={{
        background: "#0a0a0a",
        minHeight: "100vh",
        backgroundImage: `
        linear-gradient(rgba(39, 39, 42, 0.3) 1px, transparent 1px),
        linear-gradient(90deg, rgba(39, 39, 42, 0.3) 1px, transparent 1px)
      `,
        backgroundSize: "20px 20px",
      }}
    >
      <App>
        <ProLayout
          title="TikMan"
          logo={
            <div
              style={{
                width: 32,
                height: 32,
                background: "linear-gradient(135deg, #3ecf8e 0%, #2fb574 100%)",
                borderRadius: 8,
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
              }}
            >
              <svg
                style={{ width: 20, height: 20, color: "white" }}
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
          }
          layout="mix"
          splitMenus={false}
          navTheme="realDark"
          fixedHeader
          fixSiderbar
          location={location}
          route={{ routes }}
          siderWidth={256}
          contentStyle={{ paddingInline: padding.contentInline }}
          token={{
            bgLayout: "transparent",
            sider: {
              colorMenuBackground: "#18181b",
              colorBgMenuItemSelected: "rgba(62, 207, 142, 0.1)",
              colorTextMenuSelected: "#3ecf8e",
              colorTextMenu: "#a1a1aa",
              colorTextMenuItemHover: "#ffffff",
            },
            header: {
              colorBgHeader: "#18181b",
              colorHeaderTitle: "#ffffff",
              colorTextMenu: "#a1a1aa",
              colorTextMenuSelected: "#ffffff",
              heightLayoutHeader: 56,
            },
          }}
          menuItemRender={(item, dom) => (
            <div onClick={() => navigate(item.path || "/")}>{dom}</div>
          )}
          avatarProps={{
            src: undefined,
            size: "default",
            title: user?.username,
            render: () => {
              return (
                <Dropdown
                  menu={{ items: userMenuItems }}
                  placement="bottomRight"
                >
                  <div
                    style={{
                      display: "flex",
                      alignItems: "center",
                      gap: 16,
                      cursor: "pointer",
                    }}
                  >
                    {canUseCs && (
                      <Badge count={awaitingCount}>
                        <BellOutlined
                          style={{ fontSize: 18, color: "#a1a1aa" }}
                        />
                      </Badge>
                    )}
                    <Avatar style={{ backgroundColor: "#3ecf8e" }}>
                      {user?.initials ||
                        user?.username?.charAt(0).toUpperCase()}
                    </Avatar>
                  </div>
                </Dropdown>
              );
            },
          }}
          actionsRender={() => []}
          menuFooterRender={() => (
            <div
              style={{
                padding: "16px",
                borderTop: "1px solid #27272a",
                fontSize: 12,
                color: "#71717a",
              }}
            >
              OLT Provisioning System
            </div>
          )}
        >
          <div
            style={{
              padding: padding.page,
              minHeight: "calc(100vh - 56px)",
              background: "transparent",
            }}
          >
            <Outlet context={{ csStream: stream } satisfies AppLayoutContext} />
          </div>
        </ProLayout>
      </App>
    </div>
  );
}
