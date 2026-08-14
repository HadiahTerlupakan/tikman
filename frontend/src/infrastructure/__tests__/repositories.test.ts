import { describe, it, expect, vi, beforeEach } from 'vitest';
import { UserRepository } from '../repositories/UserRepository';
import type { CreateUserDto } from '@/domain/entities';
import { UserRole } from '@/domain/entities';

describe('UserRepository', () => {
  let mockClient: any;
  let repository: UserRepository;

  beforeEach(() => {
    mockClient = {
      get: vi.fn(),
      post: vi.fn(),
      put: vi.fn(),
      delete: vi.fn(),
    };
    // @ts-ignore - testing with mock
    repository = new UserRepository();
    // @ts-ignore - inject mock
    repository.client = mockClient;
  });

  it('should fetch all users', async () => {
    const mockUsers = [{ id: '1', username: 'admin', role: UserRole.ADMIN }];
    mockClient.get.mockResolvedValue({ data: mockUsers });

    const users = await repository.getAll();

    expect(users).toEqual(mockUsers);
    expect(mockClient.get).toHaveBeenCalledWith('/api/v1/users');
  });

  it('should create user', async () => {
    const createDto: CreateUserDto = {
      username: 'test',
      email: 'test@test.com',
      password: 'password123',
      role: UserRole.TECHNICIAN,
    };
    const mockUser = { id: '2', ...createDto };
    mockClient.post.mockResolvedValue({ data: mockUser });

    const user = await repository.create(createDto);

    expect(user).toEqual(mockUser);
    expect(mockClient.post).toHaveBeenCalledWith('/api/v1/users', createDto);
  });

  it('should delete user', async () => {
    mockClient.delete.mockResolvedValue({});

    await repository.delete('123');

    expect(mockClient.delete).toHaveBeenCalledWith('/api/v1/users/123');
  });
});
