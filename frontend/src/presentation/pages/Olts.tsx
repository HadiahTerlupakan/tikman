import { useState } from 'react';
import { Button, message } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { useOlts, useCreateOlt, useUpdateOlt, useDeleteOlt } from '@/application/hooks';
import { OltTable, OltModal } from '../components/olts';
import type { Olt, CreateOltDto, UpdateOltDto } from '@/domain/entities';
import { PageHeader, DarkCard } from '../components/common';

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
            message.success('OLT updated successfully');
            setModalOpen(false);
          },
          onError: () => {
            message.error('Failed to update OLT');
          },
        }
      );
    } else {
      createMutation.mutate(data as CreateOltDto, {
        onSuccess: () => {
          message.success('OLT created successfully');
          setModalOpen(false);
        },
        onError: () => {
          message.error('Failed to create OLT');
        },
      });
    }
  };

  const handleDelete = (id: string) => {
    deleteMutation.mutate(id, {
      onSuccess: () => {
        message.success('OLT deleted successfully');
      },
      onError: () => {
        message.error('Failed to delete OLT');
      },
    });
  };

  return (
    <div>
      <PageHeader
        title="OLTs Management"
        description="Manage OLT devices and configurations"
        extra={
          <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
            Create OLT
          </Button>
        }
      />

      <DarkCard>
        <OltTable
          olts={olts || []}
          loading={isLoading}
          onEdit={handleEdit}
          onDelete={handleDelete}
        />
      </DarkCard>

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
