import { useState } from "react";
import {
  Card,
  Table,
  Tag,
  Space,
  Button,
  Input,
  Select,
  Modal,
  Form,
  InputNumber,
  message,
} from "antd";
import {
  PlusOutlined,
  ReloadOutlined,
  SearchOutlined,
  EyeOutlined,
  EditOutlined,
  DeleteOutlined,
} from "@ant-design/icons";
import { useOnts, useCreateOnt, useUpdateOnt, useDeleteOnt } from "@/application/hooks/useOnts";
import { useOlts } from "@/application/hooks/useOlts";
import { OntDetailModal } from "@/presentation/components/OntDetailModal";
import type { Ont, CreateOntDto, UpdateOntDto, OntStatus } from "@/domain/entities";

const { Option } = Select;

export default function OntListPage() {
  const [searchText, setSearchText] = useState("");
  const [statusFilter, setStatusFilter] = useState<OntStatus | undefined>();
  const [oltFilter, setOltFilter] = useState<string | undefined>();
  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false);
  const [isEditModalOpen, setIsEditModalOpen] = useState(false);
  const [isDetailModalOpen, setIsDetailModalOpen] = useState(false);
  const [selectedOnt, setSelectedOnt] = useState<Ont | null>(null);
  const [createForm] = Form.useForm();
  const [editForm] = Form.useForm();

  const { data: ontsData, isLoading, refetch } = useOnts({
    oltId: oltFilter,
    status: statusFilter,
  });
  const { data: oltsData } = useOlts();
  const createMutation = useCreateOnt();
  const updateMutation = useUpdateOnt();
  const deleteMutation = useDeleteOnt();

  const getStatusColor = (status: OntStatus) => {
    switch (status) {
      case "online":
        return "success";
      case "offline":
        return "default";
      case "los":
        return "warning";
      case "unknown":
        return "error";
      default:
        return "default";
    }
  };

  const columns = [
    {
      title: "Serial Number",
      dataIndex: "serialNumber",
      key: "serialNumber",
      filteredValue: searchText ? [searchText] : null,
      onFilter: (value: unknown, record: Ont) =>
        record.serialNumber.toLowerCase().includes(String(value).toLowerCase()),
    },
    {
      title: "OLT",
      dataIndex: "oltName",
      key: "oltName",
    },
    {
      title: "Port/ONT ID",
      key: "position",
      render: (_: unknown, record: Ont) => `${record.portId}/${record.ontId}`,
    },
    {
      title: "Status",
      dataIndex: "status",
      key: "status",
      render: (status: OntStatus) => (
        <Tag color={getStatusColor(status)}>{status.toUpperCase()}</Tag>
      ),
    },
    {
      title: "Description",
      dataIndex: "description",
      key: "description",
      ellipsis: true,
    },
    {
      title: "Last Seen",
      dataIndex: "lastSeenAt",
      key: "lastSeenAt",
      render: (date: string | null) =>
        date ? new Date(date).toLocaleString() : "-",
    },
    {
      title: "Actions",
      key: "actions",
      render: (_: unknown, record: Ont) => (
        <Space>
          <Button
            type="link"
            icon={<EyeOutlined />}
            onClick={() => handleViewDetail(record)}
          >
            View
          </Button>
          <Button
            type="link"
            icon={<EditOutlined />}
            onClick={() => handleEdit(record)}
          >
            Edit
          </Button>
          <Button
            type="link"
            danger
            icon={<DeleteOutlined />}
            onClick={() => handleDelete(record)}
          >
            Delete
          </Button>
        </Space>
      ),
    },
  ];

  const handleViewDetail = (ont: Ont) => {
    setSelectedOnt(ont);
    setIsDetailModalOpen(true);
  };

  const handleEdit = (ont: Ont) => {
    setSelectedOnt(ont);
    editForm.setFieldsValue({
      description: ont.description,
      status: ont.status,
    });
    setIsEditModalOpen(true);
  };

  const handleDelete = (ont: Ont) => {
    Modal.confirm({
      title: "Delete ONT",
      content: `Are you sure you want to delete ONT ${ont.serialNumber}?`,
      okText: "Delete",
      okType: "danger",
      onOk: async () => {
        try {
          await deleteMutation.mutateAsync(ont.id);
          message.success("ONT deleted successfully");
        } catch (error) {
          console.error("Delete failed:", error);
        }
      },
    });
  };

  const handleCreate = async (values: CreateOntDto) => {
    try {
      await createMutation.mutateAsync(values);
      setIsCreateModalOpen(false);
      createForm.resetFields();
      message.success("ONT created successfully");
    } catch (error) {
      console.error("Create failed:", error);
    }
  };

  const handleUpdate = async (values: UpdateOntDto) => {
    if (!selectedOnt) return;
    try {
      await updateMutation.mutateAsync({ id: selectedOnt.id, data: values });
      setIsEditModalOpen(false);
      editForm.resetFields();
      setSelectedOnt(null);
      message.success("ONT updated successfully");
    } catch (error) {
      console.error("Update failed:", error);
    }
  };

  return (
    <div style={{ padding: "24px" }}>
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
        <Space style={{ marginBottom: 16 }} wrap>
          <Input
            placeholder="Search by serial number"
            prefix={<SearchOutlined />}
            value={searchText}
            onChange={(e) => setSearchText(e.target.value)}
            style={{ width: 250 }}
            allowClear
          />
          <Select
            placeholder="Filter by OLT"
            style={{ width: 200 }}
            value={oltFilter}
            onChange={setOltFilter}
            allowClear
          >
            {oltsData?.map((olt) => (
              <Option key={olt.id} value={olt.id}>
                {olt.name}
              </Option>
            ))}
          </Select>
          <Select
            placeholder="Filter by status"
            style={{ width: 150 }}
            value={statusFilter}
            onChange={setStatusFilter}
            allowClear
          >
            <Option value="online">Online</Option>
            <Option value="offline">Offline</Option>
            <Option value="los">LOS</Option>
            <Option value="unknown">Unknown</Option>
          </Select>
        </Space>

        <Table
          columns={columns}
          dataSource={ontsData?.data || []}
          rowKey="id"
          loading={isLoading}
          pagination={{
            total: ontsData?.total || 0,
            pageSize: ontsData?.limit || 20,
            showTotal: (total) => `Total ${total} ONTs`,
          }}
        />
      </Card>

      {/* Create Modal */}
      <Modal
        title="Add New ONT"
        open={isCreateModalOpen}
        onCancel={() => {
          setIsCreateModalOpen(false);
          createForm.resetFields();
        }}
        onOk={() => createForm.submit()}
        confirmLoading={createMutation.isPending}
      >
        <Form form={createForm} layout="vertical" onFinish={handleCreate}>
          <Form.Item
            name="oltId"
            label="OLT"
            rules={[{ required: true, message: "Please select an OLT" }]}
          >
            <Select placeholder="Select OLT">
              {oltsData?.map((olt) => (
                <Option key={olt.id} value={olt.id}>
                  {olt.name}
                </Option>
              ))}
            </Select>
          </Form.Item>
          <Form.Item
            name="portId"
            label="Port ID"
            rules={[
              { required: true, message: "Please enter port ID" },
              { type: "number", min: 0, max: 15, message: "Port ID must be 0-15" },
            ]}
          >
            <InputNumber style={{ width: "100%" }} min={0} max={15} />
          </Form.Item>
          <Form.Item
            name="ontId"
            label="ONT ID"
            rules={[
              { required: true, message: "Please enter ONT ID" },
              { type: "number", min: 0, max: 127, message: "ONT ID must be 0-127" },
            ]}
          >
            <InputNumber style={{ width: "100%" }} min={0} max={127} />
          </Form.Item>
          <Form.Item
            name="serialNumber"
            label="Serial Number"
            rules={[
              { required: true, message: "Please enter serial number" },
              { max: 20, message: "Serial number max 20 characters" },
            ]}
          >
            <Input placeholder="e.g., ZTEG12345678" />
          </Form.Item>
          <Form.Item name="description" label="Description">
            <Input.TextArea rows={3} placeholder="Optional description" />
          </Form.Item>
        </Form>
      </Modal>

      {/* Edit Modal */}
      <Modal
        title="Edit ONT"
        open={isEditModalOpen}
        onCancel={() => {
          setIsEditModalOpen(false);
          editForm.resetFields();
          setSelectedOnt(null);
        }}
        onOk={() => editForm.submit()}
        confirmLoading={updateMutation.isPending}
      >
        <Form form={editForm} layout="vertical" onFinish={handleUpdate}>
          <Form.Item name="description" label="Description">
            <Input.TextArea rows={3} />
          </Form.Item>
          <Form.Item name="status" label="Status">
            <Select>
              <Option value="online">Online</Option>
              <Option value="offline">Offline</Option>
              <Option value="los">LOS</Option>
              <Option value="unknown">Unknown</Option>
            </Select>
          </Form.Item>
        </Form>
      </Modal>

      {/* Detail Modal with Metrics */}
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
