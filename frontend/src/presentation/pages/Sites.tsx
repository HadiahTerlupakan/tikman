import { useState } from "react";
import { Button, message } from "antd";
import { PlusOutlined } from "@ant-design/icons";
import {
  useSites,
  useCreateSite,
  useUpdateSite,
  useDeleteSite,
} from "@/application/hooks";
import { SiteTable, SiteModal } from "../components/sites";
import type { Site, CreateSiteDto, UpdateSiteDto } from "@/domain/entities";
import { PageHeader, DarkCard } from "../components/common";

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
            message.success("Site updated successfully");
            setModalOpen(false);
          },
          onError: () => {
            message.error("Failed to update site");
          },
        },
      );
    } else {
      createMutation.mutate(data as CreateSiteDto, {
        onSuccess: () => {
          message.success("Site created successfully");
          setModalOpen(false);
        },
        onError: () => {
          message.error("Failed to create site");
        },
      });
    }
  };

  const handleDelete = (id: string) => {
    deleteMutation.mutate(id, {
      onSuccess: () => {
        message.success("Site deleted successfully");
      },
      onError: () => {
        message.error("Failed to delete site");
      },
    });
  };

  return (
    <div>
      <PageHeader
        title="Sites Management"
        description="Manage site locations for OLT devices"
        extra={
          <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
            Create Site
          </Button>
        }
      />

      <DarkCard>
        <SiteTable
          sites={sites || []}
          loading={isLoading}
          onEdit={handleEdit}
          onDelete={handleDelete}
        />
      </DarkCard>

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
