import type { User } from "../entities";

export interface LoginCredentials {
  username: string;
  password: string;
}

/** The session itself arrives as an HttpOnly cookie, never in this body. */
export interface LoginResponse {
  user: User;
}

export interface IAuthRepository {
  login(credentials: LoginCredentials): Promise<LoginResponse>;
  logout(): Promise<void>;
  getCurrentUser(): Promise<User>;
}
