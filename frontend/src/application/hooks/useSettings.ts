import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { SettingRepository } from "@/infrastructure/repositories";
import { GOOGLE_MAPS_API_KEY, GOOGLE_MAPS_MAP_ID } from "@/domain/entities";

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
      // Prefix-matching, so this also covers ["settings", "browser"].
      queryClient.invalidateQueries({ queryKey: ["settings"] });
    },
  });
}

export function useDeleteSetting() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (name: string) => settingRepository.remove(name),
    onSuccess: () => {
      // Prefix-matching, so this also covers ["settings", "browser"].
      queryClient.invalidateQueries({ queryKey: ["settings"] });
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

/**
 * What it takes to draw a Google map here: the key that authorises the script,
 * and the Cloud map ID the pins belong to. Either being unset is a state the
 * caller has to handle — without the key nothing loads, and without the map ID
 * the advanced markers render nothing.
 */
export function useGoogleMapsKey(): {
  key?: string;
  mapId?: string;
  isLoading: boolean;
} {
  const { data, isLoading } = useBrowserSettings();
  return {
    key: data?.[GOOGLE_MAPS_API_KEY] || undefined,
    mapId: data?.[GOOGLE_MAPS_MAP_ID] || undefined,
    isLoading,
  };
}
