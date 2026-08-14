import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { AuthRepository } from "@/infrastructure/repositories";
import { useAuthStore } from "../stores";
import type { LoginCredentials } from "@/domain/repositories";

const authRepository = new AuthRepository();

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
    mutationFn: () => authRepository.logout(),
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
