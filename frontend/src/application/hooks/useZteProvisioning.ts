import { useMutation, useQuery } from "@tanstack/react-query";
import { ZteProvisioningRepository } from "@/infrastructure/repositories";
import type { ZteGPONRegisterRequest } from "@/domain/entities";

const repository = new ZteProvisioningRepository();

export function useZteGPONRegister() {
  return useMutation({
    mutationFn: ({
      oltId,
      data,
    }: {
      oltId: string;
      data: ZteGPONRegisterRequest;
    }) => repository.register(oltId, data),
  });
}

export function useZteExistingService() {
  return useMutation({
    mutationFn: ({
      ontId,
      data,
    }: {
      ontId: string;
      data: ZteGPONRegisterRequest;
    }) => repository.configureExisting(ontId, data),
  });
}

export function useZteProvisionJob(jobId?: string) {
  return useQuery({
    queryKey: ["zte-provision-jobs", jobId],
    queryFn: () => repository.getJob(jobId as string),
    enabled: Boolean(jobId),
    refetchInterval: (query) => {
      const status = query.state.data?.status;
      return status === "pending" || status === "running" ? 2000 : false;
    },
  });
}
