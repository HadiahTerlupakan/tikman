import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { AuthRepository, PushRepository } from "@/infrastructure/repositories";
import { refreshTokenIfGranted } from "@/infrastructure/firebase/messaging";
import { useAuthStore } from "../stores";
import type { LoginCredentials } from "@/domain/repositories";

const authRepository = new AuthRepository();
const pushRepository = new PushRepository();

/** Drops this device's push registration on the way out. A notification body
 * carries the customer's name and their words, so a logged-out phone that keeps
 * receiving them is showing another team's inbox to whoever holds it. Best
 * effort by design: a device that cannot reach the API must still log out. */
async function forgetPushDevice(): Promise<void> {
  try {
    const token = await refreshTokenIfGranted();
    if (token) await pushRepository.unsubscribe(token);
  } catch (error) {
    console.warn("Could not unsubscribe this device from push", error);
  }
}

export function useLogin() {
  const queryClient = useQueryClient();
  const setUser = useAuthStore((state) => state.setUser);

  return useMutation({
    mutationFn: (credentials: LoginCredentials) =>
      authRepository.login(credentials),
    onSuccess: (data) => {
      setUser(data.user);
      queryClient.invalidateQueries({ queryKey: ["auth", "me"] });
    },
  });
}

export function useLogout() {
  const queryClient = useQueryClient();
  const logout = useAuthStore((state) => state.logout);

  return useMutation({
    // Before the logout call, not after: DELETE /push/subscribe is scoped to
    // the authenticated caller, so it needs the session still standing.
    mutationFn: async () => {
      await forgetPushDevice();
      return authRepository.logout();
    },
    onSuccess: () => {
      logout();
      queryClient.clear();
    },
  });
}

export function useCurrentUser() {
  const setUser = useAuthStore((state) => state.setUser);

  return useQuery({
    queryKey: ["auth", "me"],
    queryFn: async () => {
      const user = await authRepository.getCurrentUser();
      setUser(user);
      return user;
    },
    retry: false,
  });
}
