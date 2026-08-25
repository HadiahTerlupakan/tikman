import { useState } from "react";
import { Card, Space, Button, Form, message } from "antd";
import { PlusOutlined, ReloadOutlined } from "@ant-design/icons";
import { OntFilters } from "@/presentation/components/OntFilters";
import { OntTable } from "@/presentation/components/OntTable";
import { OntCreateModal } from "@/presentation/components/OntCreateModal";
import { OntDetailModal } from "@/presentation/components/OntDetailModal";
import { useOntListLogic } from "@/application/hooks/useOntListLogic";
import {
  ProvisionModal,
  ProvisionHistoryModal,
} from "@/presentation/components/provisioning";
import {
  useConfigTemplates,
  useProvisionOnt,
  useProvisionJobsByONT,
} from "@/application/hooks";
import type { ProvisionRequest } from "@/domain/entities/Provisioning";
import type { Ont } from "@/domain/entities";

export default function OntListPage() {
  const [createForm] = Form.useForm();

  const [isProvisionModalOpen, setIsProvisionModalOpen] = useState(false);
  const [provisionTargetOnt, setProvisionTargetOnt] = useState<Ont | null>(
    null,
  );
  const [isHistoryModalOpen, setIsHistoryModalOpen] = useState(false);
  const [historyTargetOnt, setHistoryTargetOnt] = useState<Ont | null>(null);

  const {
    searchText,
    setSearchText,
    statusFilter,
    setStatusFilter,
    selectedOltId,
    setSelectedOltId,
    selectedSlotId,
    setSelectedSlotId,
    selectedPortId,
    setSelectedPortId,
    topologyData,
    isLoadingTopology,
    isCreateModalOpen,
    setIsCreateModalOpen,
    isDetailModalOpen,
    setIsDetailModalOpen,
    selectedOnt,
    setSelectedOnt,
    oltsData,
    filteredOnts,
    isLoading,
    createMutation,
    handleViewDetail,
    handleCreate,
    handleDelete,
    handleReset,
    refetch,
  } = useOntListLogic();

  const { data: templates } = useConfigTemplates();
  const provisionMutation = useProvisionOnt();
  const { data: provisionJobs, isLoading: isLoadingProvisionJobs } =
    useProvisionJobsByONT(historyTargetOnt?.id);

  const handleProvision = (ont: Ont) => {
    setProvisionTargetOnt(ont);
    setIsProvisionModalOpen(true);
  };

  const handleViewHistory = (ont: Ont) => {
    setHistoryTargetOnt(ont);
    setIsHistoryModalOpen(true);
  };

  const handleProvisionSubmit = (data: ProvisionRequest) => {
    if (!provisionTargetOnt) return;
    provisionMutation.mutate(
      { ontId: provisionTargetOnt.id, data },
      {
        onSuccess: (response) => {
          message.success(`Provisioning started: ${response.data.status}`);
          setIsProvisionModalOpen(false);
          setProvisionTargetOnt(null);
        },
        onError: (error) => {
          message.error(`Provisioning failed: ${error.message}`);
        },
      },
    );
  };

  return (
    <div style={{ padding: "24px" }}>
      <Card style={{ marginBottom: 16 }}>
        <OntFilters
          oltsData={oltsData}
          selectedOltId={selectedOltId}
          setSelectedOltId={setSelectedOltId}
          selectedSlotId={selectedSlotId}
          setSelectedSlotId={setSelectedSlotId}
          selectedPortId={selectedPortId}
          setSelectedPortId={setSelectedPortId}
          topologyData={topologyData}
          isLoadingTopology={isLoadingTopology}
          searchText={searchText}
          setSearchText={setSearchText}
          statusFilter={statusFilter}
          setStatusFilter={setStatusFilter}
          onReset={handleReset}
        />
      </Card>

      <Card
        title="ONT Monitoring"
        extra={
          <Space>
            <Button icon={<ReloadOutlined />} onClick={() => refetch()}>
              Refresh
            </Button>
            <Button
              type="primary"
              icon={<PlusOutlined />}
              onClick={() => setIsCreateModalOpen(true)}
            >
              Add ONT
            </Button>
          </Space>
        }
      >
        <OntTable
          dataSource={filteredOnts}
          isLoading={isLoading}
          onViewDetail={handleViewDetail}
          onDelete={handleDelete}
          onProvision={handleProvision}
          onViewHistory={handleViewHistory}
        />
      </Card>

      <OntCreateModal
        open={isCreateModalOpen}
        onCancel={() => {
          setIsCreateModalOpen(false);
          createForm.resetFields();
        }}
        onSubmit={handleCreate}
        form={createForm}
        oltsData={oltsData}
        isLoading={createMutation.isPending}
      />

      {selectedOnt && (
        <OntDetailModal
          ont={selectedOnt}
          visible={isDetailModalOpen}
          onClose={() => {
            setIsDetailModalOpen(false);
            setSelectedOnt(null);
          }}
        />
      )}

      <ProvisionModal
        open={isProvisionModalOpen}
        ontId={provisionTargetOnt?.id}
        templates={templates}
        onClose={() => {
          setIsProvisionModalOpen(false);
          setProvisionTargetOnt(null);
        }}
        onSubmit={handleProvisionSubmit}
        loading={provisionMutation.isPending}
      />

      <ProvisionHistoryModal
        open={isHistoryModalOpen}
        ontId={historyTargetOnt?.id}
        jobs={provisionJobs?.data}
        loading={isLoadingProvisionJobs}
        onClose={() => {
          setIsHistoryModalOpen(false);
          setHistoryTargetOnt(null);
        }}
      />
    </div>
  );
}
