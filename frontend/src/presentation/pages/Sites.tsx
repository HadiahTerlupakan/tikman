import { useState } from 'react';
import { Button, Typography, message, Card } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { useSites, useCreateSite, useUpdateSite, useDeleteSite } from '@/application/hooks';
import { SiteTable, SiteModal } from '../components/sites';
import type { Site, CreateSiteDto, UpdateSiteDto } from '@/domain/entities';

const { Title } = Typography;

export default function SitesPage() {
  const [modalOpen, setModalOpen] = useState(false);
  const [selectedSite, setSelectedSite] = useState<Site | undefined>();

  const { data: sites, isLoading } = useSites();
  const createMutation = useCreateSite();
  const updateMutation = useUpdateSite();
  const deleteMutation = useDeleteSite();

  const handleCreate = () => {
    setSelectedSite(undefined);
    setModalOpen(true);
  };

  const handleEdit = (site: Site) => {
    setSelectedSite(site);
    setModalOpen(true);
  };

  const handleSubmit = (data: CreateSiteDto | UpdateSiteDto) => {
    if (selectedSite) {
      updateMutation.mutate(
        { id: selectedSite.id, data: data as UpdateSiteDto },
        {
          onSuccess: () => {
            message.success('Site berhasil diupdate');
            setModalOpen(false);
          },
          onError: () => {
            message.error('Gagal update site');
          },
        }
      );
    } else {
      createMutation.mutate(data as CreateSiteDto, {
        onSuccess: () => {
          message.success('Site berhasil dibuat');
          setModalOpen(false);
        },
        onError: () => {
          message.error('Gagal membuat site');
        },
      });
    }
  };

  const handleDelete = (id: string) => {
    deleteMutation.mutate(id, {
      onSuccess: () => {
        message.success('Site berhasil dihapus');
      },
      onError: () => {
        message.error('Gagal menghapus site');
      },
    });
  };

  return (
    <div className="max-w-7xl">
      <div className="flex justify-between items-center mb-6">
        <Title level={3} className="!mb-0 !text-gray-900">
          Sites Management
        </Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
          Create Site
        </Button>
      </div>

      <Card>
        <SiteTable
          sites={sites || []}
          loading={isLoading}
          onEdit={handleEdit}
          onDelete={handleDelete}
        />
      </Card>

      <SiteModal
        open={modalOpen}
        site={selectedSite}
        onClose={() => setModalOpen(false)}
        onSubmit={handleSubmit}
        loading={createMutation.isPending || updateMutation.isPending}
      />
    </div>
  );
}
