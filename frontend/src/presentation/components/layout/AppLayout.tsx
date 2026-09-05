import { Outlet, useNavigate, useLocation } from "react-router-dom";
import { ProLayout } from "@ant-design/pro-components";
import { UserOutlined, LogoutOutlined } from "@ant-design/icons";
import { Dropdown, Avatar, App, Grid } from "antd";
import type { MenuProps } from "antd";
import { useEffect } from "react";
import { useAuthStore } from "@/application/stores";
import {
  useLogout,
  useCsConversations,
  usePushNotifications,
} from "@/application/hooks";
import {
  useCsStream,
  type CsTypingStatus,
  type WaStreamStatus,
} from "@/application/hooks/useCsStream";
import { UserRole } from "@/domain/entities";
import { NotificationBell } from "@/presentation/components/cs/NotificationBell";
import {
  listenForForegroundMessages,
  registerForPush,
  showLocalNotification,
  startPushRegistration,
} from "@/infrastructure/firebase/messaging";
import { PushRepository } from "@/infrastructure/repositories";
import { playNotificationChime } from "@/infrastructure/notificationSound";
import { PushPermissionPrompt } from "@/presentation/components/cs/PushPermissionPrompt";
import { AppFooter } from "./AppFooter";
import { buildNavigationRoutes } from "./navigationRoutes";
import { FOOTER_HEIGHT, HEADER_HEIGHT, layoutPadding } from "./layoutPadding";

const pushRepository = new PushRepository();

// The three roles that can open /api/v1/cs/* at all — everything push- and
// badge-related is inert for anyone else, the same gate the backend enforces.
const CS_ROLES: UserRole[] = [UserRole.ADMIN, UserRole.CS, UserRole.TECHNICIAN];

/** Attaches this device's FID lifecycle handlers and, where permission was
 * already granted on a previous visit, re-registers silently so the current
 * FID is re-delivered through them. Returns the detach function.
 *
 * The register call has to come after the handlers are attached: the SDK
 * throws `invalid-on-registered-handler` otherwise, and the FID is only ever
 * delivered through `onRegistered` — `register()` itself resolves with
 * nothing. */
async function trackPushRegistration(): Promise<() => void> {
  const detach = await startPushRegistration({
    onRegistered: (fid) => {
      void pushRepository
        .subscribe(fid)
        .catch((error) => console.warn("Push subscribe failed", error));
    },
    onUnregistered: (fid) => {
      void pushRepository
        .unsubscribe(fid)
        .catch((error) => console.warn("Push unsubscribe failed", error));
    },
  });

  if (
    typeof Notification !== "undefined" &&
    Notification.permission === "granted"
  ) {
    // Silent re-registration is how most devices ever reach the backend, so a
    // failure here is the difference between push working and push quietly
    // never arriving. Nothing to retry — say so and move on.
    void registerForPush().catch((error) =>
      console.warn("Push re-registration failed", error),
    );
  }

  return detach;
}

/** What CsInboxPage reads back via useOutletContext, since the stream that
 * feeds the navbar badge has to run here, not on that page. */
export interface AppLayoutContext {
  csStream: WaStreamStatus;
  /** Which threads have a customer writing in them right now. */
  csTyping: CsTypingStatus;
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
  const push = usePushNotifications();

  // Runs once per app-shell mount, not per click. This is the only place that
  // tells the backend about this device: the FID arrives asynchronously
  // through onRegistered, whoever triggered the registration. The foreground
  // listener has to be live on every page too — a push can arrive while
  // looking at the OLT map, not only while CS Inbox is open.
  useEffect(() => {
    if (!canUseCs) return;
    // A cleanup can outrun the promise that produces the unsubscribe (e.g.
    // StrictMode's synchronous mount→cleanup→re-mount in dev, or canUseCs
    // flipping faster than Firebase resolves) — without this flag, the
    // listener that arrives after cleanup would never be torn down.
    let cancelled = false;
    let stopRegistration: (() => void) | undefined;
    let unsubscribe: (() => void) | undefined;

    trackPushRegistration()
      .then((detach) => {
        if (cancelled) {
          detach();
          return;
        }
        stopRegistration = detach;
      })
      .catch((error) =>
        console.warn("Could not listen for push registration", error),
      );

    listenForForegroundMessages((title, body) => {
      // The OS tone belongs to notifications it renders itself; one shown from
      // an open tab is silent unless the page makes a sound of its own.
      void playNotificationChime();
      void showLocalNotification(title, body).catch((error) =>
        console.warn("Could not show a foreground notification", error),
      );
    })
      .then((unsub) => {
        if (cancelled) {
          unsub();
          return;
        }
        unsubscribe = unsub;
      })
      .catch((error) =>
        console.warn("Could not listen for foreground pushes", error),
      );

    return () => {
      cancelled = true;
      stopRegistration?.();
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
              heightLayoutHeader: HEADER_HEIGHT,
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
                <div style={{ display: "flex", alignItems: "center", gap: 16 }}>
                  {/* Outside the Dropdown deliberately: while the count was
                      hardcoded to zero the bell sat inside the avatar's trigger
                      and nobody noticed, but a bell showing a number invites a
                      click, and that click opened the user menu. It now goes
                      where the number points — the threads waiting on a reply. */}
                  {canUseCs && (
                    <NotificationBell
                      conversations={awaitingQuery.data ?? []}
                      onOpen={(id) =>
                        navigate(`/cs?view=belum-dibalas&conversation=${id}`)
                      }
                      onSeeAll={() => navigate("/cs?view=belum-dibalas")}
                    />
                  )}
                  <Dropdown
                    menu={{ items: userMenuItems }}
                    placement="bottomRight"
                  >
                    <Avatar
                      style={{
                        backgroundColor: "#3ecf8e",
                        cursor: "pointer",
                      }}
                    >
                      {user?.initials ||
                        user?.username?.charAt(0).toUpperCase()}
                    </Avatar>
                  </Dropdown>
                </div>
              );
            },
          }}
          actionsRender={() => []}
          footerRender={() => <AppFooter />}
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
              minHeight: `calc(100vh - ${HEADER_HEIGHT + FOOTER_HEIGHT}px)`,
              background: "transparent",
            }}
          >
            {canUseCs && (
              <PushPermissionPrompt
                permission={push.permission}
                requesting={push.requesting}
                onEnable={push.enable}
              />
            )}
            <Outlet
              context={
                {
                  csStream: stream.accounts,
                  csTyping: stream.typing,
                } satisfies AppLayoutContext
              }
            />
          </div>
        </ProLayout>
      </App>
    </div>
  );
}
