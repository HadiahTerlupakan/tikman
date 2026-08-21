import { createBrowserRouter, Navigate } from "react-router-dom";
import { ProtectedRoute } from "./ProtectedRoute";
import { AppLayout } from "../components/layout";
import LoginPage from "../pages/Login";
import DashboardPage from "../pages/Dashboard";
import UsersPage from "../pages/Users";
import SitesPage from "../pages/Sites";
import OltsPage from "../pages/Olts";
import OntsPage from "../pages/OntListPage";
import { GraphsPage } from "../pages/GraphsPage";
import NotFoundPage from "../pages/NotFound";

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
            path: "olts",
            element: <OltsPage />,
          },
          {
            path: "onts",
            element: <OntsPage />,
          },
          {
            path: "graphs",
            element: <GraphsPage />,
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
