import { useState } from "react";
import { Card, Space, Button, Form, message } from "antd";
import { PlusOutlined, ReloadOutlined } from "@ant-design/icons";
import { OntFilters } from "@/presentation/components/OntFilters";
import { OntTable } from "@/presentation/components/OntTable";
import { OntRemoveDialog } from "@/presentation/components/ont-detail/OntRemoveDialog";
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
import type { Ont, ZteProvisionTarget } from "@/domain/entities";
import { ZteProvisionModal } from "@/presentation/components/zte-provisioning";
import { useZteExistingService } from "@/application/hooks";

export default function OntListPage() {
  const [createForm] = Form.useForm();

  const [isProvisionModalOpen, setIsProvisionModalOpen] = useState(false);
  const [provisionTargetOnt, setProvisionTargetOnt] = useState<Ont | null>(
    null,
  );
  const [isHistoryModalOpen, setIsHistoryModalOpen] = useState(false);
  const [serviceTarget, setServiceTarget] = useState<{
    ont: Ont;
    target: ZteProvisionTarget;
  } | null>(null);
  const serviceMutation = useZteExistingService();
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
    total,
    page,
    setPage,
    pageSize,
    setPageSize,
    isLoading,
    createMutation,
    handleViewDetail,
    handleCreate,
    handleDelete,
    handleReset,
    refetch,
  } = useOntListLogic();

  // Held here rather than in the row, because the dialog fetches the commands
  // a removal would send and has to outlive the menu that opened it.
  const [ontPendingRemoval, setOntPendingRemoval] = useState<Ont | null>(null);
  const [isRemoving, setIsRemoving] = useState(false);

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

  const handleConfigureService = (ont: Ont) => {
    setServiceTarget({
      ont,
      target: {
        oltId: ont.oltId,
        card: ont.slot,
        pon: ont.portId,
        onuId: ont.ontId,
        serialNumber: ont.serialNumber,
        onuType: ont.deviceType,
        name: ont.name,
        description: ont.description,
      },
    });
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
          page={page}
          total={total}
          pageSize={pageSize}
          onPageChange={setPage}
          onPageSizeChange={setPageSize}
          isLoading={isLoading}
          onViewDetail={handleViewDetail}
          onDelete={(id) =>
            setOntPendingRemoval(
              filteredOnts.find((candidate) => candidate.id === id) ?? null,
            )
          }
          onProvision={handleProvision}
          onConfigureService={handleConfigureService}
          onViewHistory={handleViewHistory}
        />
      </Card>

      <OntRemoveDialog
        ont={ontPendingRemoval}
        open={!!ontPendingRemoval}
        // Removing from the OLT takes seconds. Closing the dialog first left
        // no sign anything was happening, so it was pressed again.
        loading={isRemoving}
        onCancel={() => setOntPendingRemoval(null)}
        onConfirm={async (removeFromOlt) => {
          const target = ontPendingRemoval;
          if (!target) return;
          setIsRemoving(true);
          try {
            await handleDelete(target.id, removeFromOlt);
            setOntPendingRemoval(null);
          } finally {
            setIsRemoving(false);
          }
        }}
      />

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

      {serviceTarget && (
        <ZteProvisionModal
          open
          mode="configure"
          target={serviceTarget.target}
          ontId={serviceTarget.ont.id}
          onClose={() => setServiceTarget(null)}
          onSubmit={(request) => {
            serviceMutation.mutate(
              { ontId: serviceTarget.ont.id, data: request },
              {
                onSuccess: () => {
                  message.success("Service configuration started");
                  setServiceTarget(null);
                  refetch();
                },
                onError: (submitError) => message.error(submitError.message),
              },
            );
          }}
          loading={serviceMutation.isPending}
          error={serviceMutation.error}
        />
      )}

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
