export enum UserRole {
  ADMIN = 'admin',
  TECHNICIAN = 'technician',
  VIEWER = 'viewer',
}

export interface User {
  id: string;
  username: string;
  email: string;
  role: UserRole;
  createdAt: string;
  updatedAt: string;
}

export interface CreateUserDto {
  username: string;
  email: string;
  password: string;
  role: UserRole;
}

export interface UpdateUserDto {
  username?: string;
  email?: string;
  password?: string;
  role?: UserRole;
}
