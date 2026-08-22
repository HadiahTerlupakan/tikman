import { Card, Space, Button, Form } from "antd";
import { PlusOutlined, ReloadOutlined } from "@ant-design/icons";
import { OntFilters } from "@/presentation/components/OntFilters";
import { OntTable } from "@/presentation/components/OntTable";
import { OntCreateModal } from "@/presentation/components/OntCreateModal";
import { OntDetailModal } from "@/presentation/components/OntDetailModal";
import { useOntListLogic } from "@/application/hooks/useOntListLogic";

export default function OntListPage() {
  const [createForm] = Form.useForm();

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
    </div>
  );
}
