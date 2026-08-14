import { useState } from 'react';
import { Button, Typography, message } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { useUsers, useCreateUser, useUpdateUser, useDeleteUser } from '@/application/hooks';
import { UserTable, UserModal } from '../components/users';
import type { User, CreateUserDto, UpdateUserDto } from '@/domain/entities';

const { Title } = Typography;

export default function UsersPage() {
  const [modalOpen, setModalOpen] = useState(false);
  const [selectedUser, setSelectedUser] = useState<User | undefined>();

  const { data: users, isLoading } = useUsers();
  const createMutation = useCreateUser();
  const updateMutation = useUpdateUser();
  const deleteMutation = useDeleteUser();

  const handleCreate = () => {
    setSelectedUser(undefined);
    setModalOpen(true);
  };

  const handleEdit = (user: User) => {
    setSelectedUser(user);
    setModalOpen(true);
  };

  const handleSubmit = (data: CreateUserDto | UpdateUserDto) => {
    if (selectedUser) {
      updateMutation.mutate(
        { id: selectedUser.id, data: data as UpdateUserDto },
        {
          onSuccess: () => {
            message.success('User berhasil diupdate');
            setModalOpen(false);
          },
          onError: () => {
            message.error('Gagal update user');
          },
        }
      );
    } else {
      createMutation.mutate(data as CreateUserDto, {
        onSuccess: () => {
          message.success('User berhasil dibuat');
          setModalOpen(false);
        },
        onError: () => {
          message.error('Gagal membuat user');
        },
      });
    }
  };

  const handleDelete = (id: string) => {
    deleteMutation.mutate(id, {
      onSuccess: () => {
        message.success('User berhasil dihapus');
      },
      onError: () => {
        message.error('Gagal menghapus user');
      },
    });
  };

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Title level={2} style={{ margin: 0 }}>
          Users Management
        </Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
          Create User
        </Button>
      </div>

      <UserTable
        users={users || []}
        loading={isLoading}
        onEdit={handleEdit}
        onDelete={handleDelete}
      />

      <UserModal
        open={modalOpen}
        user={selectedUser}
        onClose={() => setModalOpen(false)}
        onSubmit={handleSubmit}
        loading={createMutation.isPending || updateMutation.isPending}
      />
    </div>
  );
}
