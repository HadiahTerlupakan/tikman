import { apiClient } from "../http/apiClient";
import { API_ENDPOINTS } from "../http/endpoints";
import type {
  IAuthRepository,
  LoginCredentials,
  LoginResponse,
} from "@/domain/repositories";
import type { User } from "@/domain/entities";

export class AuthRepository implements IAuthRepository {
  async login(credentials: LoginCredentials): Promise<LoginResponse> {
    const response = await apiClient.post(
      API_ENDPOINTS.AUTH_LOGIN,
      credentials,
    );
    return response.data;
  }

  async logout(): Promise<void> {
    await apiClient.post(API_ENDPOINTS.AUTH_LOGOUT);
  }

  async getCurrentUser(): Promise<User> {
    const response = await apiClient.get(API_ENDPOINTS.AUTH_ME);
    return response.data;
  }
}
