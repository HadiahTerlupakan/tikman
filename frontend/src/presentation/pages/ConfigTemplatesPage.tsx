import { useState } from "react";
import { Button, message } from "antd";
import { PlusOutlined } from "@ant-design/icons";
import {
  useConfigTemplates,
  useCreateConfigTemplate,
  useUpdateConfigTemplate,
  useDeleteConfigTemplate,
} from "@/application/hooks";
import {
  ConfigTemplateTable,
  ConfigTemplateModal,
} from "../components/config-templates";
import type {
  ConfigTemplate,
  CreateConfigTemplateDto,
  UpdateConfigTemplateDto,
} from "@/domain/entities/ConfigTemplate";
import { PageHeader, DarkCard } from "../components/common";

export default function ConfigTemplatesPage() {
  const [modalOpen, setModalOpen] = useState(false);
  const [selectedTemplate, setSelectedTemplate] = useState<
    ConfigTemplate | undefined
  >();

  const { data: templates, isLoading } = useConfigTemplates();
  const createMutation = useCreateConfigTemplate();
  const updateMutation = useUpdateConfigTemplate();
  const deleteMutation = useDeleteConfigTemplate();

  const handleCreate = () => {
    setSelectedTemplate(undefined);
    setModalOpen(true);
  };

  const handleEdit = (template: ConfigTemplate) => {
    setSelectedTemplate(template);
    setModalOpen(true);
  };

  const handleSubmit = (
    data: CreateConfigTemplateDto | UpdateConfigTemplateDto,
  ) => {
    if (selectedTemplate) {
      updateMutation.mutate(
        { id: selectedTemplate.id, data: data as UpdateConfigTemplateDto },
        {
          onSuccess: () => {
            message.success("Template updated successfully");
            setModalOpen(false);
          },
          onError: () => {
            message.error("Failed to update template");
          },
        },
      );
    } else {
      createMutation.mutate(data as CreateConfigTemplateDto, {
        onSuccess: () => {
          message.success("Template created successfully");
          setModalOpen(false);
        },
        onError: () => {
          message.error("Failed to create template");
        },
      });
    }
  };

  const handleDelete = (id: string) => {
    deleteMutation.mutate(id, {
      onSuccess: () => {
        message.success("Template deleted successfully");
      },
      onError: () => {
        message.error("Failed to delete template");
      },
    });
  };

  return (
    <div>
      <PageHeader
        title="Config Templates"
        description="Kelola template konfigurasi untuk provisioning ONT"
        extra={
          <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
            Create Template
          </Button>
        }
      />

      <DarkCard>
        <ConfigTemplateTable
          templates={templates || []}
          loading={isLoading}
          onEdit={handleEdit}
          onDelete={handleDelete}
        />
      </DarkCard>

      <ConfigTemplateModal
        open={modalOpen}
        template={selectedTemplate}
        onClose={() => setModalOpen(false)}
        onSubmit={handleSubmit}
        loading={createMutation.isPending || updateMutation.isPending}
      />
    </div>
  );
}
