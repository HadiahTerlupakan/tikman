export enum UserRole {
  ADMIN = "admin",
  TECHNICIAN = "technician",
  VIEWER = "viewer",
  CS = "cs",
}

export interface User {
  id: string;
  username: string;
  email: string;
  // Never empty on a row read back from the API: a blank field on the form
  // means "derive it from the username", not "store nothing".
  initials: string;
  role: UserRole;
  createdAt: string;
  updatedAt: string;
}

export interface CreateUserDto {
  username: string;
  email: string;
  password: string;
  initials?: string;
  role: UserRole;
}

export interface UpdateUserDto {
  username?: string;
  email?: string;
  password?: string;
  initials?: string;
  role?: UserRole;
}
