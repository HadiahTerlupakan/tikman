import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { SettingRepository } from "@/infrastructure/repositories";
import { GOOGLE_MAPS_API_KEY } from "@/domain/entities";

const settingRepository = new SettingRepository();

// Credentials change rarely, and a page that needs the key reads it once on
// entry. Polling it would be traffic for nothing.
const BROWSER_SETTINGS_STALE_TIME = 300_000;

export function useSettings() {
  return useQuery({
    queryKey: ["settings"],
    queryFn: () => settingRepository.list(),
  });
}

export function useSaveSetting() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ name, value }: { name: string; value: string }) =>
      settingRepository.save(name, value),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["settings"] });
      queryClient.invalidateQueries({ queryKey: ["settings", "browser"] });
    },
  });
}

export function useDeleteSetting() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (name: string) => settingRepository.remove(name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["settings"] });
      queryClient.invalidateQueries({ queryKey: ["settings", "browser"] });
    },
  });
}

export function useBrowserSettings() {
  return useQuery({
    queryKey: ["settings", "browser"],
    queryFn: () => settingRepository.browser(),
    staleTime: BROWSER_SETTINGS_STALE_TIME,
  });
}

/** The Maps key, or undefined when none is configured. */
export function useGoogleMapsKey(): { key?: string; isLoading: boolean } {
  const { data, isLoading } = useBrowserSettings();
  return { key: data?.[GOOGLE_MAPS_API_KEY] || undefined, isLoading };
}
