import { useMutation, useQuery } from "@tanstack/react-query";
import { apiClient } from "@/infrastructure/http/apiClient";
import { API_ENDPOINTS } from "@/infrastructure/http/endpoints";
import type {
  ProvisionRequest,
  ProvisionJob,
} from "@/domain/entities/Provisioning";

export function useProvisionOnt() {
  return useMutation({
    mutationFn: ({ ontId, data }: { ontId: string; data: ProvisionRequest }) =>
      apiClient.post<{ job_id: string; status: string; message: string }>(
        API_ENDPOINTS.ONT_PROVISION(ontId),
        data,
      ),
  });
}

export function useProvisionJob(jobId?: string) {
  return useQuery({
    queryKey: ["provision-jobs", jobId],
    queryFn: async () => {
      const response = await apiClient.get<{ data: ProvisionJob }>(
        API_ENDPOINTS.PROVISION_JOB_BY_ID(jobId!),
      );
      return response.data.data;
    },
    enabled: !!jobId,
    refetchInterval: (query) => {
      const job = query.state.data;
      if (!job || !["pending", "running"].includes(job.status as string)) {
        return false;
      }
      return 2000;
    },
  });
}

export function useProvisionJobsByONT(ontId?: string) {
  return useQuery({
    queryKey: ["provision-jobs", "ont", ontId],
    queryFn: async () => {
      const response = await apiClient.get<{
        data: ProvisionJob[];
        total: number;
      }>(API_ENDPOINTS.ONT_PROVISION_JOBS(ontId!));
      return response.data;
    },
    enabled: !!ontId,
  });
}
