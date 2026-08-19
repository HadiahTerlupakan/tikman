import { useQuery } from "@tanstack/react-query";
import { OntRepository } from "@/infrastructure/repositories";

const ontRepository = new OntRepository();

export function useOntEvents(id: string, limit = 50, offset = 0) {
  return useQuery({
    queryKey: ["onts", id, "events", limit, offset],
    queryFn: () => ontRepository.getEvents(id, limit, offset),
    enabled: !!id,
  });
}

export function useOntAvailability(id: string, days = 7) {
  return useQuery({
    queryKey: ["onts", id, "availability", days],
    queryFn: () => ontRepository.getAvailability(id, days),
    enabled: !!id,
    refetchInterval: 300000,
  });
}
