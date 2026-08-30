import { Suspense, lazy } from "react";
import { createBrowserRouter, Navigate } from "react-router-dom";
import { Spin } from "antd";
import { ProtectedRoute } from "./ProtectedRoute";
import { AppLayout } from "../components/layout";
import LoginPage from "../pages/Login";
import DashboardPage from "../pages/Dashboard";
import UsersPage from "../pages/Users";
import SitesPage from "../pages/Sites";
import MapPage from "../pages/MapPage";
import OltsPage from "../pages/Olts";
import OltConfigurationPage from "../pages/OltConfigurationPage";
import OntsPage from "../pages/OntListPage";
import UnconfiguredOnusPage from "../pages/UnconfiguredOnusPage";
import ConfigTemplatesPage from "../pages/ConfigTemplatesPage";
import VpnPage from "../pages/VpnPage";
import SettingsPage from "../pages/SettingsPage";
import NotFoundPage from "../pages/NotFound";

// Graphs is the only route that pulls in recharts, so it loads on demand to keep
// the charting library out of the initial bundle.
const GraphsPage = lazy(() =>
  import("../pages/GraphsPage").then((m) => ({ default: m.GraphsPage })),
);

export const router = createBrowserRouter([
  {
    path: "/login",
    element: <LoginPage />,
  },
  {
    path: "/",
    element: <ProtectedRoute />,
    children: [
      {
        element: <AppLayout />,
        children: [
          {
            index: true,
            element: <DashboardPage />,
          },
          {
            path: "users",
            element: <UsersPage />,
          },
          {
            path: "sites",
            element: <SitesPage />,
          },
          {
            path: "map",
            element: <MapPage />,
          },
          {
            path: "olts",
            element: <OltsPage />,
          },
          {
            path: "olts/:id/configuration",
            element: <OltConfigurationPage />,
          },
          {
            path: "onts",
            element: <OntsPage />,
          },
          {
            path: "unconfigured-onus",
            element: <UnconfiguredOnusPage />,
          },
          {
            path: "config-templates",
            element: <ConfigTemplatesPage />,
          },
          {
            path: "vpn",
            element: <VpnPage />,
          },
          {
            path: "settings",
            element: <SettingsPage />,
          },
          {
            path: "graphs",
            element: (
              <Suspense
                fallback={
                  <div style={{ padding: 24, textAlign: "center" }}>
                    <Spin />
                  </div>
                }
              >
                <GraphsPage />
              </Suspense>
            ),
          },
        ],
      },
    ],
  },
  {
    path: "/404",
    element: <NotFoundPage />,
  },
  {
    path: "*",
    element: <Navigate to="/404" replace />,
  },
]);
