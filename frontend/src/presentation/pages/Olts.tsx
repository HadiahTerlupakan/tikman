import { useState } from 'react';
import { Button, Typography, message, Card } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { useOlts, useCreateOlt, useUpdateOlt, useDeleteOlt } from '@/application/hooks';
import { OltTable, OltModal } from '../components/olts';
import type { Olt, CreateOltDto, UpdateOltDto } from '@/domain/entities';

const { Title } = Typography;

export default function OltsPage() {
  const [modalOpen, setModalOpen] = useState(false);
  const [selectedOlt, setSelectedOlt] = useState<Olt | undefined>();

  const { data: olts, isLoading } = useOlts();
  const createMutation = useCreateOlt();
  const updateMutation = useUpdateOlt();
  const deleteMutation = useDeleteOlt();

  const handleCreate = () => {
    setSelectedOlt(undefined);
    setModalOpen(true);
  };

  const handleEdit = (olt: Olt) => {
    setSelectedOlt(olt);
    setModalOpen(true);
  };

  const handleSubmit = (data: CreateOltDto | UpdateOltDto) => {
    if (selectedOlt) {
      updateMutation.mutate(
        { id: selectedOlt.id, data: data as UpdateOltDto },
        {
          onSuccess: () => {
            message.success('OLT berhasil diupdate');
            setModalOpen(false);
          },
          onError: () => {
            message.error('Gagal update OLT');
          },
        }
      );
    } else {
      createMutation.mutate(data as CreateOltDto, {
        onSuccess: () => {
          message.success('OLT berhasil dibuat');
          setModalOpen(false);
        },
        onError: () => {
          message.error('Gagal membuat OLT');
        },
      });
    }
  };

  const handleDelete = (id: string) => {
    deleteMutation.mutate(id, {
      onSuccess: () => {
        message.success('OLT berhasil dihapus');
      },
      onError: () => {
        message.error('Gagal menghapus OLT');
      },
    });
  };

  return (
    <div className="max-w-7xl">
      <div className="flex justify-between items-center mb-6">
        <Title level={3} className="!mb-0 !text-gray-900">
          OLTs Management
        </Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
          Create OLT
        </Button>
      </div>

      <Card>
        <OltTable
          olts={olts || []}
          loading={isLoading}
          onEdit={handleEdit}
          onDelete={handleDelete}
        />
      </Card>

      <OltModal
        open={modalOpen}
        olt={selectedOlt}
        onClose={() => setModalOpen(false)}
        onSubmit={handleSubmit}
        loading={createMutation.isPending || updateMutation.isPending}
      />
    </div>
  );
}
