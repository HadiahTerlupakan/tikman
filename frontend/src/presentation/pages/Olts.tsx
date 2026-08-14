import { useState } from 'react';
import { Button, Typography, message } from 'antd';
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
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Title level={2} style={{ margin: 0 }}>
          OLTs Management
        </Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
          Create OLT
        </Button>
      </div>

      <OltTable
        olts={olts || []}
        loading={isLoading}
        onEdit={handleEdit}
        onDelete={handleDelete}
      />

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
