import { RouterProvider } from "react-router-dom";
import { QueryClientProvider } from "@tanstack/react-query";
import { ConfigProvider } from "antd";
import enUS from "antd/locale/en_US";
import { queryClient } from "@/shared/config/queryClient";
import { theme } from "@/shared/theme";
import { router } from "./presentation/routes";

export default function App() {
  return (
    <ConfigProvider theme={theme} locale={enUS}>
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </ConfigProvider>
  );
}
