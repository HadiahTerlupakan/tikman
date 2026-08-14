import type { User } from '../entities';

export interface LoginCredentials {
  username: string;
  password: string;
}

export interface LoginResponse {
  user: User;
  token: string;
}

export interface IAuthRepository {
  login(credentials: LoginCredentials): Promise<LoginResponse>;
  logout(): Promise<void>;
  getCurrentUser(): Promise<User>;
}
