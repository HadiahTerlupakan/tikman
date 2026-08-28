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

// Fetched rather than rebuilt in the browser: a second copy of the command
// list drifted from the one the OLT receives, and the operator was approving
// the wrong text.
export function useZteCommandPreview(
  mode: "register" | "configure",
  targetId: string | undefined,
  data: ZteGPONRegisterRequest | undefined,
  enabled: boolean,
) {
  return useQuery({
    queryKey: ["zte-command-preview", mode, targetId, data],
    queryFn: () =>
      mode === "register"
        ? repository.previewRegister(
            targetId as string,
            data as ZteGPONRegisterRequest,
          )
        : repository.previewConfigure(
            targetId as string,
            data as ZteGPONRegisterRequest,
          ),
    enabled: enabled && Boolean(targetId) && Boolean(data),
    retry: false,
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
