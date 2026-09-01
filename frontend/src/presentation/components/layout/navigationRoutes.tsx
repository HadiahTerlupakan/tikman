import {
  DashboardOutlined,
  EnvironmentOutlined,
  ApiOutlined,
  UserOutlined,
  MonitorOutlined,
  BarChartOutlined,
  QuestionCircleOutlined,
  WarningOutlined,
  FileTextOutlined,
  CloudServerOutlined,
  SettingOutlined,
  GlobalOutlined,
} from "@ant-design/icons";
import type { ReactNode } from "react";
import { UserRole } from "@/domain/entities";

export interface NavigationRoute {
  path: string;
  name: string;
  icon: ReactNode;
}

// The menu lives outside AppLayout so it can be asserted on directly. A page
// that is reachable by URL but missing here is invisible to the operator, and
// that is not something rendering the layout in a test reliably catches.
export function buildNavigationRoutes(role?: UserRole): NavigationRoute[] {
  return [
    { path: "/", name: "Dashboard", icon: <DashboardOutlined /> },
    { path: "/sites", name: "Sites", icon: <EnvironmentOutlined /> },
    { path: "/map", name: "Map", icon: <GlobalOutlined /> },
    { path: "/olts", name: "OLTs", icon: <ApiOutlined /> },
    { path: "/onts", name: "ONT Monitoring", icon: <MonitorOutlined /> },
    {
      path: "/onts/troubled",
      name: "Pelanggan Bermasalah",
      icon: <WarningOutlined />,
    },
    {
      path: "/unconfigured-onus",
      name: "Unconfigured ONU",
      icon: <QuestionCircleOutlined />,
    },
    {
      path: "/config-templates",
      name: "Config Templates",
      icon: <FileTextOutlined />,
    },
    { path: "/graphs", name: "Graphs", icon: <BarChartOutlined /> },
    { path: "/vpn", name: "VPN", icon: <CloudServerOutlined /> },
    ...(role === UserRole.ADMIN
      ? [
          { path: "/users", name: "Users", icon: <UserOutlined /> },
          { path: "/settings", name: "Settings", icon: <SettingOutlined /> },
        ]
      : []),
  ];
}
