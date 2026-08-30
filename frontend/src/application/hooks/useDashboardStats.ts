import { useQuery } from "@tanstack/react-query";
import { DashboardRepository } from "@/infrastructure/repositories";

const dashboardRepository = new DashboardRepository();

// Matches the ONT list's old cadence, so the overview stays as live as it was.
const DASHBOARD_POLL_INTERVAL = 15000;

export function useDashboardStats() {
  return useQuery({
    queryKey: ["dashboard", "stats"],
    queryFn: () => dashboardRepository.getStats(),
    refetchInterval: DASHBOARD_POLL_INTERVAL,
    refetchIntervalInBackground: true,
    refetchOnWindowFocus: true,
    staleTime: 5000,
    retry: 3,
  });
}
