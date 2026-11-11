import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { GraphReportRepository } from "@/infrastructure/repositories";
import type {
  GraphReportInput,
  GraphReportPreviewInput,
} from "@/domain/entities";

const graphReportRepository = new GraphReportRepository();

const graphReportKeys = {
  list: ["graph-reports", "list"] as const,
  devices: ["graph-reports", "librenms", "devices"] as const,
  ports: (deviceId?: number) =>
    ["graph-reports", "librenms", "ports", deviceId] as const,
  graph: (deviceId?: number, ifName?: string, label?: string) =>
    ["graph-reports", "librenms", "graph", deviceId, ifName, label] as const,
};

export function useGraphReports() {
  return useQuery({
    queryKey: graphReportKeys.list,
    queryFn: () => graphReportRepository.list(),
  });
}

function useInvalidateGraphReports() {
  const queryClient = useQueryClient();
  return () =>
    queryClient.invalidateQueries({ queryKey: graphReportKeys.list });
}

export function useSaveGraphReport() {
  const invalidate = useInvalidateGraphReports();
  return useMutation({
    mutationFn: ({ id, input }: { id?: string; input: GraphReportInput }) =>
      id
        ? graphReportRepository.update(id, input)
        : graphReportRepository.create(input),
    onSuccess: invalidate,
  });
}

export function useDeleteGraphReport() {
  const invalidate = useInvalidateGraphReports();
  return useMutation({
    mutationFn: (id: string) => graphReportRepository.remove(id),
    onSuccess: invalidate,
  });
}

/** A failed send also records its error on the report, so both outcomes
 * refetch the list. */
export function useSendGraphReport() {
  const invalidate = useInvalidateGraphReports();
  return useMutation({
    mutationFn: (id: string) => graphReportRepository.send(id),
    onSettled: invalidate,
  });
}

// Not retried: the usual failure is LibreNMS being unset or unreachable, and
// three retries only hold back the message that says so.
export function useLibrenmsDevices(enabled: boolean) {
  return useQuery({
    queryKey: graphReportKeys.devices,
    queryFn: () => graphReportRepository.devices(),
    enabled,
    retry: false,
  });
}

export function useLibrenmsPorts(deviceId?: number) {
  return useQuery({
    queryKey: graphReportKeys.ports(deviceId),
    queryFn: () => graphReportRepository.ports(deviceId as number),
    enabled: deviceId !== undefined,
    retry: false,
  });
}

/** The last 24 hours as LibreNMS draws them; a refetch moves the window. */
export function useLibrenmsGraph(
  deviceId?: number,
  ifName?: string,
  label?: string,
) {
  return useQuery({
    queryKey: graphReportKeys.graph(deviceId, ifName, label),
    queryFn: () =>
      graphReportRepository.graph(deviceId as number, ifName!, label ?? ""),
    enabled: deviceId !== undefined && !!ifName,
    retry: false,
    gcTime: 0,
  });
}

/** A mutation rather than a query: it previews unsaved form values, and each
 * click should fetch the window as it is now. */
export function useGraphReportPreview() {
  return useMutation({
    mutationFn: (input: GraphReportPreviewInput) =>
      graphReportRepository.preview(input),
  });
}
