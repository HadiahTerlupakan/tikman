import { apiClient } from "../http/apiClient";
import { API_ENDPOINTS } from "../http/endpoints";
import type { IUserRepository } from "@/domain/repositories";
import type { User, CreateUserDto, UpdateUserDto } from "@/domain/entities";

export class UserRepository implements IUserRepository {
  private client = apiClient;

  async getAll(): Promise<User[]> {
    const response = await this.client.get(API_ENDPOINTS.USERS);
    return response.data;
  }

  async getById(id: string): Promise<User> {
    const response = await this.client.get(API_ENDPOINTS.USER_BY_ID(id));
    return response.data;
  }

  async create(data: CreateUserDto): Promise<User> {
    const response = await this.client.post(API_ENDPOINTS.USERS, data);
    return response.data;
  }

  async update(id: string, data: UpdateUserDto): Promise<User> {
    const response = await this.client.put(API_ENDPOINTS.USER_BY_ID(id), data);
    return response.data;
  }

  async delete(id: string): Promise<void> {
    await this.client.delete(API_ENDPOINTS.USER_BY_ID(id));
  }
}
