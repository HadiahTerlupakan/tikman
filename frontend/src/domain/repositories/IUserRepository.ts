import type { User, CreateUserDto, UpdateUserDto } from '../entities';

export interface IUserRepository {
  getAll(): Promise<User[]>;
  getById(id: string): Promise<User>;
  create(data: CreateUserDto): Promise<User>;
  update(id: string, data: UpdateUserDto): Promise<User>;
  delete(id: string): Promise<void>;
}
